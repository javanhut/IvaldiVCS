package butterfly

import (
	"fmt"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

type Syncer struct {
	manager  *Manager
	cas      cas.CAS
	refs     *refs.RefsManager
	mmr      *history.PersistentMMR
	resolver *ConflictResolver
}

func NewSyncer(manager *Manager, casStore cas.CAS, refsManager *refs.RefsManager, mmr *history.PersistentMMR) *Syncer {
	return &Syncer{
		manager:  manager,
		cas:      casStore,
		refs:     refsManager,
		mmr:      mmr,
		resolver: NewConflictResolver(casStore),
	}
}

func (s *Syncer) SyncUp(butterflyName string) error {
	bf, err := s.manager.GetButterflyInfo(butterflyName)
	if err != nil {
		return fmt.Errorf("not a butterfly timeline: %w", err)
	}

	if bf.IsOrphaned {
		return fmt.Errorf("cannot sync orphaned butterfly '%s'", butterflyName)
	}

	butterflyTimeline, err := s.refs.GetTimeline(butterflyName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get butterfly timeline: %w", err)
	}

	parentTimeline, err := s.refs.GetTimeline(bf.ParentName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get parent timeline: %w", err)
	}

	butterflyHash := butterflyTimeline.Blake3Hash
	parentHash := parentTimeline.Blake3Hash

	if butterflyHash == parentHash {
		return nil
	}

	mergedHash, err := s.resolver.FastForwardMerge(butterflyHash, parentHash)
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	var mergedHashArray [32]byte
	copy(mergedHashArray[:], mergedHash[:])

	err = s.refs.UpdateTimeline(
		bf.ParentName,
		refs.LocalTimeline,
		mergedHashArray,
		parentTimeline.SHA256Hash,
		parentTimeline.GitSHA1Hash,
	)
	if err != nil {
		return fmt.Errorf("failed to update parent timeline: %w", err)
	}

	err = s.manager.UpdateDivergence(butterflyName, mergedHash)
	if err != nil {
		return fmt.Errorf("failed to update divergence: %w", err)
	}

	return nil
}

func (s *Syncer) SyncDown(butterflyName string) error {
	bf, err := s.manager.GetButterflyInfo(butterflyName)
	if err != nil {
		return fmt.Errorf("not a butterfly timeline: %w", err)
	}

	if bf.IsOrphaned {
		return fmt.Errorf("cannot sync orphaned butterfly '%s'", butterflyName)
	}

	butterflyTimeline, err := s.refs.GetTimeline(butterflyName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get butterfly timeline: %w", err)
	}

	parentTimeline, err := s.refs.GetTimeline(bf.ParentName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get parent timeline: %w", err)
	}

	butterflyHash := butterflyTimeline.Blake3Hash
	parentHash := parentTimeline.Blake3Hash

	if butterflyHash == parentHash {
		return nil
	}

	mergedHash, err := s.resolver.FastForwardMerge(parentHash, butterflyHash)
	if err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	var mergedHashArray [32]byte
	copy(mergedHashArray[:], mergedHash[:])

	err = s.refs.UpdateTimeline(
		butterflyName,
		refs.LocalTimeline,
		mergedHashArray,
		butterflyTimeline.SHA256Hash,
		butterflyTimeline.GitSHA1Hash,
	)
	if err != nil {
		return fmt.Errorf("failed to update butterfly timeline: %w", err)
	}

	err = s.manager.UpdateDivergence(butterflyName, parentHash)
	if err != nil {
		return fmt.Errorf("failed to update divergence: %w", err)
	}

	return nil
}

func (s *Syncer) GetParentStatus(butterflyName string) (commitsAhead int, commitsBehind int, err error) {
	bf, err := s.manager.GetButterflyInfo(butterflyName)
	if err != nil {
		return 0, 0, err
	}

	if bf.IsOrphaned {
		return 0, 0, nil
	}

	return 0, 0, nil
}
