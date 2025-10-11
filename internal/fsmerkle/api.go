package fsmerkle

import (
	"fmt"
	"path"
	"strings"
)

// BuildTreeFromMap creates a filesystem tree from a map of paths to content.
// Keys are POSIX-style paths ("a/b.txt"), values are file contents.
// Returns the root hash and total number of nodes created.
func BuildTreeFromMap(files map[string][]byte) (root Hash, count int, err error) {
	if len(files) == 0 {
		// Empty tree
		store := NewStore(NewMemoryCAS())
		hash, err := store.PutTree([]Entry{})
		return hash, 1, err
	}
	
	store := NewStore(NewMemoryCAS())
	root, err = buildTreeFromMapRecursive(store, files, "")
	if err != nil {
		return Hash{}, 0, err
	}
	
	return root, store.cas.(*MemoryCAS).Len(), nil
}

// buildTreeFromMapRecursive recursively builds the tree structure.
func buildTreeFromMapRecursive(store *Store, files map[string][]byte, prefix string) (Hash, error) {
	// Collect entries for this directory level
	entries := make(map[string]Entry)
	subdirs := make(map[string]map[string][]byte)
	
	for filepath, content := range files {
		// Skip files not in this directory level
		if prefix != "" && !strings.HasPrefix(filepath, prefix+"/") {
			continue
		}
		
		// Remove prefix to get relative path
		relPath := filepath
		if prefix != "" {
			relPath = strings.TrimPrefix(filepath, prefix+"/")
		}
		
		// Split into immediate child and remainder
		parts := strings.SplitN(relPath, "/", 2)
		name := parts[0]
		
		if len(parts) == 1 {
			// This is a file at this level
			hash, size, err := store.PutBlob(content)
			if err != nil {
				return Hash{}, fmt.Errorf("failed to create blob for %s: %w", filepath, err)
			}
			
			entries[name] = Entry{
				Name: name,
				Mode: 0100644,
				Kind: KindBlob,
				Hash: hash,
			}
			_ = size // size is tracked in the blob node
		} else {
			// This is a subdirectory - collect all files for it
			if subdirs[name] == nil {
				subdirs[name] = make(map[string][]byte)
			}
			subdirs[name][filepath] = content
		}
	}
	
	// Process subdirectories
	for name, subFiles := range subdirs {
		subPrefix := prefix
		if subPrefix == "" {
			subPrefix = name
		} else {
			subPrefix = subPrefix + "/" + name
		}
		
		subHash, err := buildTreeFromMapRecursive(store, subFiles, subPrefix)
		if err != nil {
			return Hash{}, err
		}
		
		entries[name] = Entry{
			Name: name,
			Mode: 040000,
			Kind: KindTree,
			Hash: subHash,
		}
	}
	
	// Convert map to sorted slice
	entrySlice := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		entrySlice = append(entrySlice, entry)
	}
	
	return store.PutTree(entrySlice)
}

// ChangeKind represents the type of change between two trees.
type ChangeKind int

const (
	Added      ChangeKind = iota // File/directory was added
	Deleted                      // File/directory was removed
	Modified                     // File content was changed
	TypeChange                   // Node type changed (file<->directory)
)

// String returns a human-readable representation of the ChangeKind.
func (ck ChangeKind) String() string {
	switch ck {
	case Added:
		return "added"
	case Deleted:
		return "deleted"
	case Modified:
		return "modified"
	case TypeChange:
		return "typechange"
	default:
		return fmt.Sprintf("unknown(%d)", ck)
	}
}

// Change represents a difference between two filesystem trees.
type Change struct {
	Path    string     // POSIX-style path of the changed item
	Kind    ChangeKind // Type of change
	OldHash Hash       // Hash of the old node (zero for Added)
	NewHash Hash       // Hash of the new node (zero for Deleted)
	OldMode uint32     // Mode of the old node (zero for Added)
	NewMode uint32     // Mode of the new node (zero for Deleted)
}

// DiffTrees computes the differences between two filesystem trees.
// Returns a slice of changes sorted by path.
// Uses structural sharing to efficiently skip identical subtrees.
func DiffTrees(a, b Hash, ldr Loader) ([]Change, error) {
	var changes []Change
	
	if err := diffTreesRecursive(a, b, "", ldr, &changes); err != nil {
		return nil, err
	}
	
	return changes, nil
}

// diffTreesRecursive performs the recursive tree comparison.
func diffTreesRecursive(aHash, bHash Hash, pathPrefix string, ldr Loader, changes *[]Change) error {
	// If hashes are equal, trees are identical - structural sharing optimization
	if aHash == bHash {
		return nil
	}
	
	// Load both trees
	var aTree, bTree *TreeNode
	var err error
	
	// Handle zero hashes (representing non-existent trees)
	if aHash == (Hash{}) {
		aTree = &TreeNode{Entries: []Entry{}}
	} else {
		aTree, err = ldr.LoadTree(aHash)
		if err != nil {
			return fmt.Errorf("failed to load tree A at %s: %w", pathPrefix, err)
		}
	}
	
	if bHash == (Hash{}) {
		bTree = &TreeNode{Entries: []Entry{}}
	} else {
		bTree, err = ldr.LoadTree(bHash)
		if err != nil {
			return fmt.Errorf("failed to load tree B at %s: %w", pathPrefix, err)
		}
	}
	
	// Create maps for efficient lookup
	aEntries := make(map[string]Entry)
	bEntries := make(map[string]Entry)
	
	for _, entry := range aTree.Entries {
		aEntries[entry.Name] = entry
	}
	for _, entry := range bTree.Entries {
		bEntries[entry.Name] = entry
	}
	
	// Find all unique names
	allNames := make(map[string]bool)
	for name := range aEntries {
		allNames[name] = true
	}
	for name := range bEntries {
		allNames[name] = true
	}
	
	// Process each name
	for name := range allNames {
		childPath := path.Join(pathPrefix, name)
		aEntry, aExists := aEntries[name]
		bEntry, bExists := bEntries[name]
		
		switch {
		case !aExists && bExists:
			// Added
			*changes = append(*changes, Change{
				Path:    childPath,
				Kind:    Added,
				NewHash: bEntry.Hash,
				NewMode: bEntry.Mode,
			})
			
		case aExists && !bExists:
			// Deleted
			*changes = append(*changes, Change{
				Path:    childPath,
				Kind:    Deleted,
				OldHash: aEntry.Hash,
				OldMode: aEntry.Mode,
			})
			
		case aExists && bExists:
			// Both exist - check for changes
			if aEntry.Kind != bEntry.Kind {
				// Type change (file <-> directory)
				*changes = append(*changes, Change{
					Path:    childPath,
					Kind:    TypeChange,
					OldHash: aEntry.Hash,
					NewHash: bEntry.Hash,
					OldMode: aEntry.Mode,
					NewMode: bEntry.Mode,
				})
			} else if aEntry.Hash != bEntry.Hash {
				if aEntry.Kind == KindTree {
					// Recurse into subdirectories
					if err := diffTreesRecursive(aEntry.Hash, bEntry.Hash, childPath, ldr, changes); err != nil {
						return err
					}
				} else {
					// Modified blob
					*changes = append(*changes, Change{
						Path:    childPath,
						Kind:    Modified,
						OldHash: aEntry.Hash,
						NewHash: bEntry.Hash,
						OldMode: aEntry.Mode,
						NewMode: bEntry.Mode,
					})
				}
			}
			// If hashes are equal, no change (already handled by structural sharing check above)
		}
	}
	
	return nil
}

