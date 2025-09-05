// Package shelf implements internal workspace shelving (stashing) functionality.
// This is used automatically during timeline switches to preserve uncommitted changes.
// Shelves are completely transparent to the user and managed automatically.
package shelf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
)

// Shelf represents a stashed workspace state.
type Shelf struct {
	ID              string              `json:"id"`
	TimelineName    string              `json:"timeline_name"`
	Message         string              `json:"message"`
	CreatedAt       time.Time           `json:"created_at"`
	WorkspaceIndex  wsindex.IndexRef    `json:"workspace_index"`
	BaseIndex       wsindex.IndexRef    `json:"base_index"`
	AutoCreated     bool                `json:"auto_created"`
}

// ShelfManager manages workspace shelves.
type ShelfManager struct {
	CAS       cas.CAS
	IvaldiDir string
	shelfDir  string
}

// NewShelfManager creates a new shelf manager.
func NewShelfManager(casStore cas.CAS, ivaldiDir string) *ShelfManager {
	shelfDir := filepath.Join(ivaldiDir, "shelves")
	os.MkdirAll(shelfDir, 0755)
	
	return &ShelfManager{
		CAS:       casStore,
		IvaldiDir: ivaldiDir,
		shelfDir:  shelfDir,
	}
}

// CreateAutoShelf automatically creates a shelf for the current workspace changes.
// This is called when switching timelines to preserve uncommitted changes.
func (sm *ShelfManager) CreateAutoShelf(timelineName string, currentIndex, baseIndex wsindex.IndexRef) (*Shelf, error) {
	shelfID := fmt.Sprintf("auto_%s_%d", timelineName, time.Now().Unix())
	message := fmt.Sprintf("Auto-shelf for timeline '%s' (created during timeline switch)", timelineName)
	
	shelf := &Shelf{
		ID:              shelfID,
		TimelineName:    timelineName,
		Message:         message,
		CreatedAt:       time.Now(),
		WorkspaceIndex:  currentIndex,
		BaseIndex:       baseIndex,
		AutoCreated:     true,
	}
	
	// Save shelf to disk
	if err := sm.saveShelf(shelf); err != nil {
		return nil, fmt.Errorf("failed to save auto-shelf: %w", err)
	}
	
	return shelf, nil
}


// GetAutoShelf retrieves the most recent auto-shelf for a timeline, if it exists.
func (sm *ShelfManager) GetAutoShelf(timelineName string) (*Shelf, error) {
	shelves, err := sm.listShelves()
	if err != nil {
		return nil, err
	}
	
	// Find the most recent auto-shelf for this timeline
	var latestAutoShelf *Shelf
	for _, shelf := range shelves {
		if shelf.TimelineName == timelineName && shelf.AutoCreated {
			if latestAutoShelf == nil || shelf.CreatedAt.After(latestAutoShelf.CreatedAt) {
				latestAutoShelf = &shelf
			}
		}
	}
	
	return latestAutoShelf, nil
}

// listShelves returns all shelves sorted by creation time (newest first).
// This is used internally to find the most recent auto-shelf.
func (sm *ShelfManager) listShelves() ([]Shelf, error) {
	files, err := os.ReadDir(sm.shelfDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read shelf directory: %w", err)
	}
	
	var shelves []Shelf
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			shelf, err := sm.loadShelf(file.Name())
			if err != nil {
				continue // Skip corrupted shelves
			}
			shelves = append(shelves, *shelf)
		}
	}
	
	// Sort by creation time (newest first)
	for i := 0; i < len(shelves)-1; i++ {
		for j := i + 1; j < len(shelves); j++ {
			if shelves[j].CreatedAt.After(shelves[i].CreatedAt) {
				shelves[i], shelves[j] = shelves[j], shelves[i]
			}
		}
	}
	
	return shelves, nil
}


// RemoveAutoShelf removes the auto-shelf for a specific timeline.
func (sm *ShelfManager) RemoveAutoShelf(timelineName string) error {
	shelf, err := sm.GetAutoShelf(timelineName)
	if err != nil {
		return err
	}
	
	if shelf == nil {
		return nil // No auto-shelf exists
	}
	
	return sm.removeShelf(shelf.ID)
}

// removeShelf removes a shelf by ID (internal method).
func (sm *ShelfManager) removeShelf(shelfID string) error {
	shelfPath := filepath.Join(sm.shelfDir, shelfID+".json")
	if err := os.Remove(shelfPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("shelf '%s' does not exist", shelfID)
		}
		return fmt.Errorf("failed to remove shelf: %w", err)
	}
	
	return nil
}

// saveShelf saves a shelf to disk.
func (sm *ShelfManager) saveShelf(shelf *Shelf) error {
	shelfPath := filepath.Join(sm.shelfDir, shelf.ID+".json")
	
	data, err := json.MarshalIndent(shelf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal shelf: %w", err)
	}
	
	if err := os.WriteFile(shelfPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write shelf file: %w", err)
	}
	
	return nil
}

// loadShelf loads a shelf from disk.
func (sm *ShelfManager) loadShelf(filename string) (*Shelf, error) {
	shelfPath := filepath.Join(sm.shelfDir, filename)
	
	data, err := os.ReadFile(shelfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read shelf file: %w", err)
	}
	
	var shelf Shelf
	if err := json.Unmarshal(data, &shelf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shelf: %w", err)
	}
	
	return &shelf, nil
}