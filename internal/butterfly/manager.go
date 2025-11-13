package butterfly

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

type Manager struct {
	metadataStore *MetadataStore
	refsManager   *refs.RefsManager
	ivaldiDir     string
	cas           cas.CAS
	mmr           *history.PersistentMMR
}

func NewManager(ivaldiDir string, casStore cas.CAS, refsManager *refs.RefsManager, mmr *history.PersistentMMR) (*Manager, error) {
	butterfliesDir := filepath.Join(ivaldiDir, "butterflies")
	if err := os.MkdirAll(butterfliesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create butterflies directory: %w", err)
	}

	metadataStore, err := NewMetadataStore(ivaldiDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		metadataStore: metadataStore,
		refsManager:   refsManager,
		ivaldiDir:     ivaldiDir,
		cas:           casStore,
		mmr:           mmr,
	}, nil
}

func (m *Manager) Close() error {
	return m.metadataStore.Close()
}

func (m *Manager) CreateButterfly(name, parentName string, divergenceHash cas.Hash) error {
	timeline, err := m.refsManager.GetTimeline(parentName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("parent timeline not found: %w", err)
	}

	if m.IsButterfly(name) {
		return fmt.Errorf("butterfly '%s' already exists", name)
	}

	bf := &Butterfly{
		Name:           name,
		ParentName:     parentName,
		DivergenceHash: divergenceHash,
		CreatedAt:      time.Now(),
		IsOrphaned:     false,
		OriginalParent: "",
	}

	if err := m.metadataStore.StoreButterfly(bf); err != nil {
		return fmt.Errorf("failed to store butterfly metadata: %w", err)
	}

	if err := m.metadataStore.AddChild(parentName, name); err != nil {
		return fmt.Errorf("failed to add child reference: %w", err)
	}

	var blake3Hash, sha256Hash [32]byte
	copy(blake3Hash[:], timeline.Blake3Hash[:])
	copy(sha256Hash[:], timeline.SHA256Hash[:])

	err = m.refsManager.CreateTimeline(
		name,
		refs.LocalTimeline,
		blake3Hash,
		sha256Hash,
		timeline.GitSHA1Hash,
		fmt.Sprintf("Butterfly from '%s'", parentName),
	)
	if err != nil {
		m.metadataStore.DeleteButterfly(name)
		m.metadataStore.RemoveChild(parentName, name)
		return fmt.Errorf("failed to create timeline: %w", err)
	}

	return nil
}

func (m *Manager) GetButterflyInfo(name string) (*Butterfly, error) {
	return m.metadataStore.GetButterfly(name)
}

func (m *Manager) IsButterfly(name string) bool {
	_, err := m.metadataStore.GetButterfly(name)
	return err == nil
}

func (m *Manager) GetParent(name string) (string, error) {
	bf, err := m.metadataStore.GetButterfly(name)
	if err != nil {
		return "", err
	}
	return bf.ParentName, nil
}

func (m *Manager) GetChildren(name string) ([]string, error) {
	return m.metadataStore.GetChildren(name)
}

func (m *Manager) DeleteButterfly(name string, cascade bool) error {
	if !m.IsButterfly(name) {
		return fmt.Errorf("'%s' is not a butterfly timeline", name)
	}

	bf, err := m.GetButterflyInfo(name)
	if err != nil {
		return err
	}

	children, _ := m.GetChildren(name)

	if cascade {
		for _, child := range children {
			if err := m.DeleteButterfly(child, true); err != nil {
				return fmt.Errorf("failed to delete child butterfly '%s': %w", child, err)
			}
		}
	} else {
		for _, child := range children {
			if err := m.metadataStore.MarkOrphaned(child, bf.ParentName); err != nil {
				return fmt.Errorf("failed to mark child '%s' as orphaned: %w", child, err)
			}
		}
	}

	if !bf.IsOrphaned {
		if err := m.metadataStore.RemoveChild(bf.ParentName, name); err != nil {
			return fmt.Errorf("failed to remove child reference: %w", err)
		}
	}

	refPath := filepath.Join(m.ivaldiDir, "refs", "heads", name)
	if err := os.Remove(refPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove timeline ref: %w", err)
	}

	if err := m.metadataStore.DeleteButterfly(name); err != nil {
		return fmt.Errorf("failed to delete butterfly metadata: %w", err)
	}

	return nil
}

func (m *Manager) GetMetadata(timelineName string) (*ButterflyMetadata, error) {
	return m.metadataStore.GetMetadata(timelineName)
}

func (m *Manager) ListAllButterflies() ([]*Butterfly, error) {
	return m.metadataStore.ListAllButterflies()
}

func (m *Manager) GetDivergencePoint(name string) (cas.Hash, error) {
	bf, err := m.GetButterflyInfo(name)
	if err != nil {
		return cas.Hash{}, err
	}
	return bf.DivergenceHash, nil
}

func (m *Manager) UpdateDivergence(name string, newDivergence cas.Hash) error {
	bf, err := m.GetButterflyInfo(name)
	if err != nil {
		return err
	}
	bf.DivergenceHash = newDivergence
	return m.metadataStore.StoreButterfly(bf)
}
