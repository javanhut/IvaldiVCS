// Package strategies implements automatic conflict resolution strategies.
//
// Strategies allow automatic resolution of merge conflicts without manual intervention:
// - Auto: Intelligent chunk-level merge (default, best for most cases)
// - Ours: Always keep target timeline version
// - Theirs: Always accept source timeline version
// - Union: Combine both versions (for append-only files)
// - Base: Revert to common ancestor version
package diffmerge

import (
	"fmt"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// StrategyType represents the type of merge strategy.
type StrategyType string

const (
	StrategyAuto   StrategyType = "auto"   // Intelligent chunk-level merge (default)
	StrategyOurs   StrategyType = "ours"   // Keep target timeline version
	StrategyTheirs StrategyType = "theirs" // Accept source timeline version
	StrategyUnion  StrategyType = "union"  // Combine both versions
	StrategyBase   StrategyType = "base"   // Revert to common ancestor
)

// Strategy defines the interface for conflict resolution strategies.
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Resolve attempts to resolve conflicts using this strategy
	Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error)
}

// AutoStrategy performs intelligent chunk-level merging.
// This is the default and most intelligent strategy.
type AutoStrategy struct{}

func (s *AutoStrategy) Name() string {
	return "auto"
}

func (s *AutoStrategy) Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	// Use the chunk merger to perform intelligent merge
	result, err := merger.MergeFile(path, base, left, right)
	if err != nil {
		return nil, err
	}

	// Auto strategy uses chunk-level intelligence
	// If there are still conflicts, they're real conflicts that need resolution
	return result, nil
}

// OursStrategy always keeps the target timeline (left) version.
type OursStrategy struct{}

func (s *OursStrategy) Name() string {
	return "ours"
}

func (s *OursStrategy) Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	result := &ChunkMergeResult{
		Path:    path,
		Success: true,
	}

	// If left version exists, use it
	if left != nil {
		result.MergedChunks, result.MergedSize = merger.extractChunks(left.FileRef)
	}
	// Otherwise, file is deleted in left (accept deletion)

	return result, nil
}

// TheirsStrategy always accepts the source timeline (right) version.
type TheirsStrategy struct{}

func (s *TheirsStrategy) Name() string {
	return "theirs"
}

func (s *TheirsStrategy) Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	result := &ChunkMergeResult{
		Path:    path,
		Success: true,
	}

	// If right version exists, use it
	if right != nil {
		result.MergedChunks, result.MergedSize = merger.extractChunks(right.FileRef)
	}
	// Otherwise, file is deleted in right (accept deletion)

	return result, nil
}

// UnionStrategy combines both versions by concatenating changes.
// Useful for append-only files like changelogs.
type UnionStrategy struct{}

func (s *UnionStrategy) Name() string {
	return "union"
}

func (s *UnionStrategy) Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	result := &ChunkMergeResult{
		Path:    path,
		Success: true,
	}

	// If both versions exist, combine them
	if left != nil && right != nil {
		// Get chunks from both versions
		leftChunks, _ := merger.extractChunks(left.FileRef)
		rightChunks, _ := merger.extractChunks(right.FileRef)

		// Find what changed in each version compared to base
		var baseChunks []cas.Hash
		if base != nil {
			baseChunks, _ = merger.extractChunks(base.FileRef)
		}

		// Strategy: Include all chunks from both versions
		// This is a simple concatenation approach
		// A smarter approach would deduplicate and order intelligently

		// For now, use a simple heuristic:
		// - If chunks are same, include once
		// - If chunks differ, include both (right after left)

		merged := make(map[cas.Hash]bool)
		var combinedChunks []cas.Hash
		var totalSize int64

		// Add unique chunks from left
		for _, chunk := range leftChunks {
			if !merged[chunk] {
				combinedChunks = append(combinedChunks, chunk)
				merged[chunk] = true
			}
		}

		// Add unique chunks from right
		for _, chunk := range rightChunks {
			if !merged[chunk] {
				combinedChunks = append(combinedChunks, chunk)
				merged[chunk] = true
			}
		}

		// Remove base chunks that weren't modified
		var finalChunks []cas.Hash
		baseSet := make(map[cas.Hash]bool)
		for _, chunk := range baseChunks {
			baseSet[chunk] = true
		}

		for _, chunk := range combinedChunks {
			finalChunks = append(finalChunks, chunk)
			chunkData, err := merger.CAS.Get(chunk)
			if err == nil {
				totalSize += int64(len(chunkData))
			}
		}

		result.MergedChunks = finalChunks
		result.MergedSize = totalSize
		return result, nil
	}

	// If only one version exists, use it
	if left != nil {
		result.MergedChunks, result.MergedSize = merger.extractChunks(left.FileRef)
		return result, nil
	}
	if right != nil {
		result.MergedChunks, result.MergedSize = merger.extractChunks(right.FileRef)
		return result, nil
	}

	// Both deleted
	return result, nil
}

// BaseStrategy reverts to the common ancestor version.
type BaseStrategy struct{}

func (s *BaseStrategy) Name() string {
	return "base"
}

func (s *BaseStrategy) Resolve(merger *ChunkMerger, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	result := &ChunkMergeResult{
		Path:    path,
		Success: true,
	}

	// If base version exists, use it
	if base != nil {
		result.MergedChunks, result.MergedSize = merger.extractChunks(base.FileRef)
	}
	// Otherwise, file didn't exist in base (accept deletion/non-existence)

	return result, nil
}

// StrategyResolver manages strategy selection and application.
type StrategyResolver struct {
	merger     *ChunkMerger
	strategies map[StrategyType]Strategy
}

// NewStrategyResolver creates a new StrategyResolver.
func NewStrategyResolver(casStore cas.CAS) *StrategyResolver {
	merger := NewChunkMerger(casStore)

	strategies := map[StrategyType]Strategy{
		StrategyAuto:   &AutoStrategy{},
		StrategyOurs:   &OursStrategy{},
		StrategyTheirs: &TheirsStrategy{},
		StrategyUnion:  &UnionStrategy{},
		StrategyBase:   &BaseStrategy{},
	}

	return &StrategyResolver{
		merger:     merger,
		strategies: strategies,
	}
}

// Resolve applies the specified strategy to resolve conflicts.
func (sr *StrategyResolver) Resolve(strategyType StrategyType, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	strategy, exists := sr.strategies[strategyType]
	if !exists {
		return nil, fmt.Errorf("unknown strategy: %s", strategyType)
	}

	return strategy.Resolve(sr.merger, path, base, left, right)
}

// GetStrategy returns a strategy by type.
func (sr *StrategyResolver) GetStrategy(strategyType StrategyType) (Strategy, error) {
	strategy, exists := sr.strategies[strategyType]
	if !exists {
		return nil, fmt.Errorf("unknown strategy: %s", strategyType)
	}
	return strategy, nil
}

// ResolveWithFallback attempts to resolve with primary strategy, falls back to secondary if conflicts remain.
func (sr *StrategyResolver) ResolveWithFallback(primary, fallback StrategyType, path string, base, left, right *wsindex.FileMetadata) (*ChunkMergeResult, error) {
	// Try primary strategy
	result, err := sr.Resolve(primary, path, base, left, right)
	if err != nil {
		return nil, err
	}

	// If successful, return
	if result.Success {
		return result, nil
	}

	// If conflicts remain and we have a fallback, try it
	if fallback != "" {
		return sr.Resolve(fallback, path, base, left, right)
	}

	// No fallback, return result with conflicts
	return result, nil
}

// BuildMergedFile reconstructs a complete file from merged chunks.
func BuildMergedFile(casStore cas.CAS, chunks []cas.Hash, totalSize int64) (filechunk.NodeRef, error) {
	if len(chunks) == 0 {
		// Empty file
		builder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())
		return builder.Build(nil)
	}

	if len(chunks) == 1 {
		// Single chunk - return as leaf
		return filechunk.NodeRef{
			Hash: chunks[0],
			Kind: filechunk.Leaf,
			Size: totalSize,
		}, nil
	}

	// Multiple chunks - need to build a proper Merkle tree
	// For now, simplified: reconstruct content and rebuild
	var content []byte

	for _, chunkHash := range chunks {
		chunkData, err := casStore.Get(chunkHash)
		if err != nil {
			return filechunk.NodeRef{}, fmt.Errorf("failed to get chunk: %w", err)
		}
		// Extract leaf data if it's encoded
		if len(chunkData) > 0 && chunkData[0] == 0x00 {
			// It's a leaf - extract the content
			merger := &ChunkMerger{CAS: casStore}
			leafData := merger.extractLeafData(chunkData)
			content = append(content, leafData...)
		} else {
			// Raw data or internal node - for now, append as-is
			content = append(content, chunkData...)
		}
	}

	// Rebuild the file with proper chunking
	builder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())
	return builder.Build(content)
}
