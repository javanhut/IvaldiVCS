package converter

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/submodule"
)

type GitSubmoduleConversionResult struct {
	Converted     int
	ClonedModules int
	Skipped       int
	Errors        []error
}

type GitSubmodule struct {
	Name   string
	Path   string
	URL    string
	Branch string
}

func ConvertGitSubmodulesToIvaldi(
	gitDir, ivaldiDir, workDir string,
	recursive bool,
) (*GitSubmoduleConversionResult, error) {
	result := &GitSubmoduleConversionResult{}

	gitmodulesPath := filepath.Join(workDir, ".gitmodules")
	gitmodules, err := parseGitmodulesFile(gitmodulesPath)
	if err != nil {
		return result, fmt.Errorf("parse .gitmodules: %w", err)
	}

	if len(gitmodules) == 0 {
		return result, nil
	}

	ivaldimodules := convertGitmodulesToIvaldimodules(gitmodules)
	ivaldimodulesPath := filepath.Join(workDir, ".ivaldimodules")
	if err := submodule.WriteIvaldimodules(ivaldimodulesPath, ivaldimodules); err != nil {
		return result, fmt.Errorf("write .ivaldimodules: %w", err)
	}

	for _, gitsub := range gitmodules {
		log.Printf("Converting Git submodule: %s", gitsub.Path)

		submodulePath := filepath.Join(workDir, gitsub.Path)
		submoduleGitDir := filepath.Join(submodulePath, ".git")

		var commitHash string

		if _, err := os.Stat(submoduleGitDir); err == nil {
			commitHash, err = getGitSubmoduleCommit(submodulePath)
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Errorf("get commit for %s: %w", gitsub.Path, err))
				result.Skipped++
				continue
			}
		} else {
			log.Printf("Cloning submodule %s from %s", gitsub.Path, gitsub.URL)

			commitHash, err = cloneGitSubmodule(gitsub.URL, gitsub.Path, workDir)
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Errorf("clone submodule %s: %w", gitsub.Path, err))
				result.Skipped++
				continue
			}
			result.ClonedModules++
		}

		submoduleIvaldiDir := filepath.Join(ivaldiDir, "modules", gitsub.Path)
		if err := os.MkdirAll(submoduleIvaldiDir, 0755); err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("create submodule ivaldi dir: %w", err))
			result.Skipped++
			continue
		}

		if err := initializeIvaldiInSubmodule(submodulePath, submoduleIvaldiDir, recursive); err != nil {
			result.Errors = append(result.Errors,
				fmt.Errorf("initialize Ivaldi in submodule %s: %w", gitsub.Path, err))
			result.Skipped++
			continue
		}

		result.Converted++
		if len(commitHash) >= 8 {
			log.Printf("Successfully converted submodule: %s (commit: %s)",
				gitsub.Path, commitHash[:8])
		} else {
			log.Printf("Successfully converted submodule: %s", gitsub.Path)
		}
	}

	return result, nil
}

func parseGitmodulesFile(path string) ([]GitSubmodule, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var submodules []GitSubmodule
	var current *GitSubmodule

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[submodule") {
			if current != nil {
				submodules = append(submodules, *current)
			}

			start := strings.Index(line, "\"")
			end := strings.LastIndex(line, "\"")
			if start != -1 && end != -1 && end > start {
				name := line[start+1 : end]
				current = &GitSubmodule{Name: name}
			}
		} else if current != nil {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "path":
					current.Path = value
				case "url":
					current.URL = value
				case "branch":
					current.Branch = value
				}
			}
		}
	}

	if current != nil {
		submodules = append(submodules, *current)
	}

	return submodules, scanner.Err()
}

func convertGitmodulesToIvaldimodules(gitmodules []GitSubmodule) []submodule.Config {
	ivaldimodules := make([]submodule.Config, len(gitmodules))

	for i, gitsub := range gitmodules {
		timeline := gitsub.Branch
		if timeline == "" {
			timeline = "main"
		}

		ivaldimodules[i] = submodule.Config{
			Name:     gitsub.Name,
			Path:     gitsub.Path,
			URL:      gitsub.URL,
			Timeline: timeline,
		}
	}

	return ivaldimodules
}

func cloneGitSubmodule(url, path, workDir string) (commitHash string, err error) {
	submodulePath := filepath.Join(workDir, path)

	if err := os.MkdirAll(filepath.Dir(submodulePath), 0755); err != nil {
		return "", fmt.Errorf("create parent directory: %w", err)
	}

	cmd := exec.Command("git", "clone", url, submodulePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %w\n%s", err, output)
	}

	return getGitSubmoduleCommit(submodulePath)
}

func getGitSubmoduleCommit(submodulePath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = submodulePath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get git commit: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func initializeIvaldiInSubmodule(submodulePath, ivaldiDir string, recursive bool) error {
	if err := os.MkdirAll(ivaldiDir, 0755); err != nil {
		return fmt.Errorf("create .ivaldi dir: %w", err)
	}

	gitDir := filepath.Join(submodulePath, ".git")
	convResult, err := ConvertGitObjectsToIvaldiConcurrent(gitDir, ivaldiDir, 8)
	if err != nil {
		return fmt.Errorf("convert git objects: %w", err)
	}

	log.Printf("  Converted %d Git objects in submodule", convResult.Converted)

	if recursive {
		subGitmodules := filepath.Join(submodulePath, ".gitmodules")
		if _, err := os.Stat(subGitmodules); err == nil {
			log.Printf("  Detecting nested submodules in %s", submodulePath)
			_, err := ConvertGitSubmodulesToIvaldi(gitDir, ivaldiDir, submodulePath, true)
			if err != nil {
				log.Printf("  Warning: nested submodule conversion: %v", err)
			}
		}
	}

	return nil
}
