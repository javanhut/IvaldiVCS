package history

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkLeafEncoding(b *testing.B) {
	leaf := &Leaf{
		TreeRoot:   [32]byte{1, 2, 3, 4, 5},
		TimelineID: "main",
		PrevIdx:    42,
		MergeIdxs:  []uint64{10, 20, 30},
		Author:     "Alice Smith <alice@example.com>",
		TimeUnix:   time.Now().Unix(),
		Message:    "This is a longer commit message with more details about the changes made",
		Meta: map[string]string{
			"tag":         "v1.2.3",
			"autoshelved": "1",
			"reviewer":    "Bob",
			"ci-build":    "passing",
		},
	}

	b.Run("CanonicalBytes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = leaf.CanonicalBytes()
		}
	})

	canonical := leaf.CanonicalBytes()

	b.Run("ParseCanonical", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := parseLeafCanonical(canonical)
			if err != nil {
				b.Fatalf("Parse failed: %v", err)
			}
		}
	})

	b.Run("Hash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = leaf.Hash()
		}
	})
}

func BenchmarkMMROperations(b *testing.B) {
	b.Run("AppendLeaf", func(b *testing.B) {
		mmr := NewMMR()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			leaf := Leaf{
				TreeRoot:   [32]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)},
				TimelineID: "main",
				Author:     "Alice",
				TimeUnix:   int64(i),
				Message:    fmt.Sprintf("Commit %d", i),
			}
			
			if i > 0 {
				leaf.PrevIdx = uint64(i - 1)
			} else {
				leaf.PrevIdx = NoParent
			}
			
			_, _, err := mmr.AppendLeaf(leaf)
			if err != nil {
				b.Fatalf("AppendLeaf failed: %v", err)
			}
		}
	})

	// Pre-populate MMR for other benchmarks
	mmr := NewMMR()
	const prePopSize = 10000
	
	for i := 0; i < prePopSize; i++ {
		leaf := Leaf{
			TreeRoot:   [32]byte{byte(i)},
			TimelineID: "main",
			Author:     "Alice",
			Message:    fmt.Sprintf("Commit %d", i),
		}
		if i > 0 {
			leaf.PrevIdx = uint64(i - 1)
		} else {
			leaf.PrevIdx = NoParent
		}
		mmr.AppendLeaf(leaf)
	}

	b.Run("Root", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mmr.Root()
		}
	})

	b.Run("GetLeaf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := uint64(i % prePopSize)
			_, err := mmr.GetLeaf(idx)
			if err != nil {
				b.Fatalf("GetLeaf failed: %v", err)
			}
		}
	})

	b.Run("Proof", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx := uint64(i % prePopSize)
			_, err := mmr.Proof(idx)
			if err != nil {
				b.Fatalf("Proof failed: %v", err)
			}
		}
	})

	// Generate a proof for verification benchmark
	proof, _ := mmr.Proof(prePopSize / 2)
	leaf, _ := mmr.GetLeaf(prePopSize / 2)
	leafHash := leaf.Hash()
	root := mmr.Root()

	b.Run("Verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !mmr.Verify(leafHash, proof, root) {
				b.Fatal("Proof verification failed")
			}
		}
	})
}

func BenchmarkHistoryManager(b *testing.B) {
	b.Run("Commit", func(b *testing.B) {
		mmr := NewMMR()
		timelineStore := NewMemoryTimelineStore()
		manager := NewHistoryManager(mmr, timelineStore)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			leaf := Leaf{
				TreeRoot: [32]byte{byte(i)},
				Author:   "Alice",
				TimeUnix: int64(i),
				Message:  fmt.Sprintf("Commit %d", i),
			}
			
			_, _, err := manager.Commit("main", leaf)
			if err != nil {
				b.Fatalf("Commit failed: %v", err)
			}
		}
	})

	// Pre-populate for LCA benchmarks
	mmr := NewMMR()
	timelineStore := NewMemoryTimelineStore()
	manager := NewHistoryManager(mmr, timelineStore)
	
	const chainLength = 1000
	var indices []uint64
	
	// Create main chain
	for i := 0; i < chainLength; i++ {
		leaf := Leaf{
			TreeRoot: [32]byte{byte(i)},
			Author:   "Alice",
			Message:  fmt.Sprintf("Main commit %d", i),
		}
		idx, _, _ := manager.Commit("main", leaf)
		indices = append(indices, idx)
	}
	
	// Create feature branch from middle
	midIdx := indices[chainLength/2]
	manager.SetTimelineHead("feature", midIdx)
	
	for i := 0; i < chainLength/2; i++ {
		leaf := Leaf{
			TreeRoot: [32]byte{byte(i + 200)},
			Author:   "Bob",
			Message:  fmt.Sprintf("Feature commit %d", i),
		}
		idx, _, _ := manager.Commit("feature", leaf)
		indices = append(indices, idx)
	}

	b.Run("LCA-SameTimeline", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// LCA between different points on main timeline
			aIdx := indices[chainLength/4]
			bIdx := indices[chainLength*3/4]
			
			_, err := manager.LCA(aIdx, bIdx)
			if err != nil {
				b.Fatalf("LCA failed: %v", err)
			}
		}
	})

	b.Run("LCA-CrossTimeline", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// LCA between main and feature timelines
			mainHead, _ := manager.GetTimelineHead("main")
			featureHead, _ := manager.GetTimelineHead("feature")
			
			_, err := manager.LCA(mainHead, featureHead)
			if err != nil {
				b.Fatalf("Cross-timeline LCA failed: %v", err)
			}
		}
	})
}

func BenchmarkSkipTable(b *testing.B) {
	mmr := NewMMR()
	skipTable := NewSkipTable()
	
	const chainLength = 10000
	var indices []uint64
	
	// Build a long chain
	for i := 0; i < chainLength; i++ {
		leaf := Leaf{
			TreeRoot:   [32]byte{byte(i)},
			TimelineID: "main",
			Author:     "Alice",
			Message:    fmt.Sprintf("Commit %d", i),
		}
		
		if i > 0 {
			leaf.PrevIdx = indices[i-1]
		} else {
			leaf.PrevIdx = NoParent
		}
		
		idx, _, _ := mmr.AppendLeaf(leaf)
		indices = append(indices, idx)
		skipTable.AddLeaf(idx, mmr)
	}

	b.Run("AddLeaf", func(b *testing.B) {
		// Test adding new leaves to skip table
		testMMR := NewMMR()
		testSkipTable := NewSkipTable()
		
		// Pre-populate
		for i := 0; i < 100; i++ {
			leaf := Leaf{
				TreeRoot: [32]byte{byte(i)},
				Author:   "Alice",
				Message:  fmt.Sprintf("Setup %d", i),
			}
			if i > 0 {
				leaf.PrevIdx = uint64(i - 1)
			} else {
				leaf.PrevIdx = NoParent
			}
			idx, _, _ := testMMR.AppendLeaf(leaf)
			testSkipTable.AddLeaf(idx, testMMR)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			leaf := Leaf{
				TreeRoot: [32]byte{byte(i + 100)},
				Author:   "Alice",
				Message:  fmt.Sprintf("Bench %d", i),
				PrevIdx:  uint64(i + 99),
			}
			
			idx, _, _ := testMMR.AppendLeaf(leaf)
			testSkipTable.AddLeaf(idx, testMMR)
		}
	})

	b.Run("LCA-Near", func(b *testing.B) {
		// LCA of nearby commits
		for i := 0; i < b.N; i++ {
			aIdx := indices[chainLength-100]
			bIdx := indices[chainLength-10]
			
			_, err := skipTable.LCA(aIdx, bIdx, mmr)
			if err != nil {
				b.Fatalf("Near LCA failed: %v", err)
			}
		}
	})

	b.Run("LCA-Far", func(b *testing.B) {
		// LCA of distant commits
		for i := 0; i < b.N; i++ {
			aIdx := indices[100]
			bIdx := indices[chainLength-100]
			
			_, err := skipTable.LCA(aIdx, bIdx, mmr)
			if err != nil {
				b.Fatalf("Far LCA failed: %v", err)
			}
		}
	})
}

func BenchmarkTimelineStore(b *testing.B) {
	store := NewMemoryTimelineStore()
	
	// Pre-populate with many timelines
	for i := 0; i < 1000; i++ {
		store.SetHead(fmt.Sprintf("timeline-%d", i), uint64(i*100))
	}

	b.Run("GetHead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timeline := fmt.Sprintf("timeline-%d", i%1000)
			_, ok := store.GetHead(timeline)
			if !ok {
				b.Fatalf("Timeline should exist: %s", timeline)
			}
		}
	})

	b.Run("SetHead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timeline := fmt.Sprintf("bench-timeline-%d", i)
			err := store.SetHead(timeline, uint64(i))
			if err != nil {
				b.Fatalf("SetHead failed: %v", err)
			}
		}
	})

	b.Run("List", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timelines := store.List()
			if len(timelines) == 0 {
				b.Fatal("Should have timelines")
			}
		}
	})
}

// Benchmark the hash computation functions
func BenchmarkHashComputation(b *testing.B) {
	leafHash := [32]byte{1, 2, 3, 4, 5}
	leftHash := [32]byte{10, 20, 30}
	rightHash := [32]byte{40, 50, 60}

	b.Run("LeafHash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = computeLeafHash(leafHash)
		}
	})

	b.Run("InternalHash", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = computeInternalHash(leftHash, rightHash)
		}
	})

	// Benchmark root computation from peaks
	peaks := []Hash{
		{1}, {2}, {3}, {4}, {5}, {6}, {7}, {8},
	}

	b.Run("RootFromPeaks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = computeRootFromPeaks(peaks)
		}
	})
}

// Benchmark various MMR sizes to show scalability
func BenchmarkMMRScaling(b *testing.B) {
	sizes := []int{100, 1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				mmr := NewMMR()
				
				for j := 0; j < size; j++ {
					leaf := Leaf{
						TreeRoot: [32]byte{byte(j)},
						Author:   "Alice",
						Message:  fmt.Sprintf("Commit %d", j),
					}
					if j > 0 {
						leaf.PrevIdx = uint64(j - 1)
					} else {
						leaf.PrevIdx = NoParent
					}
					
					_, _, err := mmr.AppendLeaf(leaf)
					if err != nil {
						b.Fatalf("AppendLeaf failed at %d: %v", j, err)
					}
				}
				
				// Compute final root to ensure all operations complete
				_ = mmr.Root()
			}
		})
	}
}