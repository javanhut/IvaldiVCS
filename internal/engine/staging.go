package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GatherFiles stages the specified files for the next seal
func GatherFiles(workDir, ivaldiDir string, files []string) error {
	stageDir := filepath.Join(ivaldiDir, "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}

	stageFile := filepath.Join(stageDir, "files")

	// Read existing staged files
	existingStaged := make(map[string]bool)
	if data, err := os.ReadFile(stageFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				existingStaged[line] = true
			}
		}
	}

	// Add new files
	for _, file := range files {
		existingStaged[file] = true
	}

	// Write all staged files
	f, err := os.Create(stageFile)
	if err != nil {
		return fmt.Errorf("failed to create stage file: %w", err)
	}
	defer f.Close()

	for file := range existingStaged {
		if _, err := f.WriteString(file + "\n"); err != nil {
			return fmt.Errorf("failed to write to stage file: %w", err)
		}
	}

	return nil
}

// UngatherFiles removes the specified files from the staging area
func UngatherFiles(ivaldiDir string, files []string) error {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return nil // Nothing staged
	}

	data, err := os.ReadFile(stageFile)
	if err != nil {
		return fmt.Errorf("failed to read stage file: %w", err)
	}

	// Build set of files to remove
	removeSet := make(map[string]bool, len(files))
	for _, f := range files {
		removeSet[f] = true
	}

	// Filter out the files to ungather
	lines := strings.Split(string(data), "\n")
	var remaining []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !removeSet[line] {
			remaining = append(remaining, line)
		}
	}

	// Write back
	f, err := os.Create(stageFile)
	if err != nil {
		return fmt.Errorf("failed to create stage file: %w", err)
	}
	defer f.Close()

	for _, file := range remaining {
		if _, err := f.WriteString(file + "\n"); err != nil {
			return fmt.Errorf("failed to write to stage file: %w", err)
		}
	}

	return nil
}

// GatherAllUnstaged stages all files that are modified or untracked
func GatherAllUnstaged(workDir, ivaldiDir string, files []FileStatusInfo) error {
	var toGather []string
	for _, f := range files {
		if f.Status == StatusModified || f.Status == StatusUntracked || f.Status == StatusDeleted {
			toGather = append(toGather, f.Path)
		}
	}
	if len(toGather) == 0 {
		return nil
	}
	return GatherFiles(workDir, ivaldiDir, toGather)
}

// UngatherAll removes all files from the staging area
func UngatherAll(ivaldiDir string) error {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(stageFile)
}
