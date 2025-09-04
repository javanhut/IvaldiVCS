package history

import (
	"fmt"
)

// TimelineStore manages timeline head pointers.
// A timeline is a named pointer to the latest leaf index in that logical stream.
type TimelineStore interface {
	// GetHead returns the head leaf index for a timeline, or (0, false) if not found.
	GetHead(name string) (idx uint64, ok bool)
	
	// SetHead sets the head leaf index for a timeline.
	SetHead(name string, idx uint64) error
	
	// List returns all timeline names.
	List() []string
}

// MemoryTimelineStore is an in-memory implementation of TimelineStore for testing.
type MemoryTimelineStore struct {
	heads map[string]uint64
}

// NewMemoryTimelineStore creates a new in-memory timeline store.
func NewMemoryTimelineStore() *MemoryTimelineStore {
	return &MemoryTimelineStore{
		heads: make(map[string]uint64),
	}
}

// GetHead implements TimelineStore.GetHead.
func (m *MemoryTimelineStore) GetHead(name string) (uint64, bool) {
	idx, ok := m.heads[name]
	return idx, ok
}

// SetHead implements TimelineStore.SetHead.
func (m *MemoryTimelineStore) SetHead(name string, idx uint64) error {
	m.heads[name] = idx
	return nil
}

// List implements TimelineStore.List.
func (m *MemoryTimelineStore) List() []string {
	names := make([]string, 0, len(m.heads))
	for name := range m.heads {
		names = append(names, name)
	}
	return names
}

// Manager provides high-level timeline and commit management.
type Manager interface {
	// Accumulator returns the underlying MMR accumulator.
	Accumulator() Accumulator
	
	// GetTimelineHead returns the head leaf index for a timeline.
	GetTimelineHead(name string) (uint64, bool)
	
	// SetTimelineHead sets the head leaf index for a timeline.
	SetTimelineHead(name string, idx uint64) error
	
	// Commit creates a new commit on a timeline.
	// Automatically fills PrevIdx from the current timeline head.
	Commit(timeline string, leaf Leaf) (idx uint64, root Hash, err error)
	
	// LCA computes the lowest common ancestor of two leaf indices.
	LCA(aIdx, bIdx uint64) (uint64, error)
}

// HistoryManager implements the Manager interface.
type HistoryManager struct {
	accumulator     Accumulator
	timelineStore   TimelineStore
	skipTables      map[string]*SkipTable // Skip tables per timeline for fast LCA
}

// NewHistoryManager creates a new history manager with the given stores.
func NewHistoryManager(accumulator Accumulator, timelineStore TimelineStore) *HistoryManager {
	return &HistoryManager{
		accumulator:   accumulator,
		timelineStore: timelineStore,
		skipTables:    make(map[string]*SkipTable),
	}
}

// Accumulator implements Manager.Accumulator.
func (h *HistoryManager) Accumulator() Accumulator {
	return h.accumulator
}

// GetTimelineHead implements Manager.GetTimelineHead.
func (h *HistoryManager) GetTimelineHead(name string) (uint64, bool) {
	return h.timelineStore.GetHead(name)
}

// SetTimelineHead implements Manager.SetTimelineHead.
func (h *HistoryManager) SetTimelineHead(name string, idx uint64) error {
	return h.timelineStore.SetHead(name, idx)
}

// Commit implements Manager.Commit.
func (h *HistoryManager) Commit(timeline string, leaf Leaf) (uint64, Hash, error) {
	// Fill in PrevIdx from current timeline head
	if headIdx, exists := h.timelineStore.GetHead(timeline); exists {
		leaf.PrevIdx = headIdx
	} else {
		leaf.PrevIdx = NoParent
	}
	
	// Ensure timeline ID is set correctly
	leaf.TimelineID = timeline
	
	// Append to accumulator
	idx, root, err := h.accumulator.AppendLeaf(leaf)
	if err != nil {
		return 0, Hash{}, fmt.Errorf("failed to append leaf: %w", err)
	}
	
	// Update timeline head
	if err := h.timelineStore.SetHead(timeline, idx); err != nil {
		return 0, Hash{}, fmt.Errorf("failed to update timeline head: %w", err)
	}
	
	// Update skip table for this timeline
	h.updateSkipTable(timeline, idx)
	
	return idx, root, nil
}

// LCA implements Manager.LCA using binary lifting.
func (h *HistoryManager) LCA(aIdx, bIdx uint64) (uint64, error) {
	if aIdx == bIdx {
		return aIdx, nil
	}
	
	// Get the leaves
	leafA, err := h.accumulator.GetLeaf(aIdx)
	if err != nil {
		return 0, fmt.Errorf("failed to get leaf A: %w", err)
	}
	
	leafB, err := h.accumulator.GetLeaf(bIdx)
	if err != nil {
		return 0, fmt.Errorf("failed to get leaf B: %w", err)
	}
	
	// If they're on the same timeline, use skip table
	if leafA.TimelineID == leafB.TimelineID {
		skipTable := h.getSkipTable(leafA.TimelineID)
		return skipTable.LCA(aIdx, bIdx, h.accumulator)
	}
	
	// Different timelines - find where they converge
	// This is a simplified implementation; a full version would maintain
	// cross-timeline ancestry information
	return h.findCrossTimelineLCA(aIdx, bIdx)
}

// findCrossTimelineLCA finds LCA across different timelines.
// This is a naive implementation that traces back both chains.
func (h *HistoryManager) findCrossTimelineLCA(aIdx, bIdx uint64) (uint64, error) {
	// Build ancestor sets
	ancestorsA := make(map[uint64]bool)
	ancestorsB := make(map[uint64]bool)
	
	// Trace back from A
	current := aIdx
	for current != NoParent {
		ancestorsA[current] = true
		
		leaf, err := h.accumulator.GetLeaf(current)
		if err != nil {
			break
		}
		
		if leaf.HasParent() {
			current = leaf.PrevIdx
		} else {
			break
		}
	}
	
	// Trace back from B, looking for common ancestor
	current = bIdx
	for current != NoParent {
		if ancestorsA[current] {
			return current, nil // Found common ancestor
		}
		ancestorsB[current] = true
		
		leaf, err := h.accumulator.GetLeaf(current)
		if err != nil {
			break
		}
		
		if leaf.HasParent() {
			current = leaf.PrevIdx
		} else {
			break
		}
	}
	
	return NoParent, fmt.Errorf("no common ancestor found")
}

// updateSkipTable updates the skip table for a timeline after adding a new leaf.
func (h *HistoryManager) updateSkipTable(timeline string, newIdx uint64) {
	skipTable := h.getSkipTable(timeline)
	skipTable.AddLeaf(newIdx, h.accumulator)
}

// getSkipTable gets or creates a skip table for a timeline.
func (h *HistoryManager) getSkipTable(timeline string) *SkipTable {
	if table, exists := h.skipTables[timeline]; exists {
		return table
	}
	
	table := NewSkipTable()
	h.skipTables[timeline] = table
	
	// Initialize with existing leaves on this timeline
	h.initializeSkipTable(timeline, table)
	
	return table
}

// initializeSkipTable builds a skip table for an existing timeline.
func (h *HistoryManager) initializeSkipTable(timeline string, table *SkipTable) {
	// This would need to walk through all leaves and build the skip table
	// For now, we'll do a simple initialization
	if headIdx, exists := h.timelineStore.GetHead(timeline); exists {
		// Walk back through the timeline and add all leaves
		current := headIdx
		indices := make([]uint64, 0)
		
		for current != NoParent {
			indices = append(indices, current)
			
			leaf, err := h.accumulator.GetLeaf(current)
			if err != nil {
				break
			}
			
			if leaf.TimelineID == timeline && leaf.HasParent() {
				current = leaf.PrevIdx
			} else {
				break
			}
		}
		
		// Add them in forward order
		for i := len(indices) - 1; i >= 0; i-- {
			table.AddLeaf(indices[i], h.accumulator)
		}
	}
}

// SkipTable implements binary lifting for efficient LCA queries within a timeline.
type SkipTable struct {
	// up[i][j] = 2^j-th ancestor of leaf i
	up   map[uint64]map[int]uint64
	maxK int // Maximum k value (log2 of max depth)
}

// NewSkipTable creates a new skip table.
func NewSkipTable() *SkipTable {
	return &SkipTable{
		up:   make(map[uint64]map[int]uint64),
		maxK: 20, // Support up to 2^20 = ~1M deep chains
	}
}

// AddLeaf adds a leaf to the skip table.
func (s *SkipTable) AddLeaf(idx uint64, acc Accumulator) {
	leaf, err := acc.GetLeaf(idx)
	if err != nil {
		return
	}
	
	// Initialize skip table entry
	s.up[idx] = make(map[int]uint64)
	
	// Base case: 2^0 = 1st ancestor
	if leaf.HasParent() {
		s.up[idx][0] = leaf.PrevIdx
	} else {
		s.up[idx][0] = NoParent
	}
	
	// Fill higher powers
	for k := 1; k < s.maxK; k++ {
		prevAncestor := s.up[idx][k-1]
		if prevAncestor == NoParent {
			s.up[idx][k] = NoParent
		} else if prevUp, exists := s.up[prevAncestor]; exists {
			s.up[idx][k] = prevUp[k-1]
		} else {
			s.up[idx][k] = NoParent
		}
	}
}

// LCA computes the LCA of two indices using binary lifting.
func (s *SkipTable) LCA(aIdx, bIdx uint64, acc Accumulator) (uint64, error) {
	if aIdx == bIdx {
		return aIdx, nil
	}
	
	// Ensure both indices are in the skip table
	if _, exists := s.up[aIdx]; !exists {
		s.AddLeaf(aIdx, acc)
	}
	if _, exists := s.up[bIdx]; !exists {
		s.AddLeaf(bIdx, acc)
	}
	
	// Get depths by following parent chain
	depthA := s.getDepth(aIdx)
	depthB := s.getDepth(bIdx)
	
	// Bring them to the same depth
	if depthA > depthB {
		aIdx = s.liftUp(aIdx, depthA-depthB)
	} else if depthB > depthA {
		bIdx = s.liftUp(bIdx, depthB-depthA)
	}
	
	if aIdx == bIdx {
		return aIdx, nil
	}
	
	// Binary search for LCA
	for k := s.maxK - 1; k >= 0; k-- {
		aUp := s.up[aIdx][k]
		bUp := s.up[bIdx][k]
		
		if aUp != NoParent && bUp != NoParent && aUp != bUp {
			aIdx = aUp
			bIdx = bUp
		}
	}
	
	// The LCA should be one step up
	return s.up[aIdx][0], nil
}

// getDepth computes the depth of a leaf by following parent pointers.
func (s *SkipTable) getDepth(idx uint64) int {
	depth := 0
	current := idx
	
	for current != NoParent && s.up[current] != nil {
		if parent := s.up[current][0]; parent != NoParent {
			current = parent
			depth++
		} else {
			break
		}
	}
	
	return depth
}

// liftUp lifts a node up by the specified number of steps.
func (s *SkipTable) liftUp(idx uint64, steps int) uint64 {
	current := idx
	
	for k := 0; k < s.maxK && steps > 0; k++ {
		if steps&(1<<k) != 0 {
			if up, exists := s.up[current]; exists && up[k] != NoParent {
				current = up[k]
			} else {
				break
			}
		}
	}
	
	return current
}