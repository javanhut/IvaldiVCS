package submodule

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ParseIvaldimodules(path string) ([]Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open .ivaldimodules: %w", err)
	}
	defer file.Close()

	var configs []Config
	var current *Config

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[submodule") {
			if current != nil {
				if err := validateConfig(current); err != nil {
					return nil, fmt.Errorf("line %d: invalid config: %w", lineNum, err)
				}
				configs = append(configs, *current)
			}

			start := strings.Index(line, "\"")
			end := strings.LastIndex(line, "\"")
			if start == -1 || end == -1 || end <= start {
				return nil, fmt.Errorf("line %d: invalid submodule section", lineNum)
			}

			name := line[start+1 : end]
			current = &Config{Name: name}
			continue
		}

		if current == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "path":
			current.Path = value
		case "url":
			current.URL = value
		case "timeline":
			current.Timeline = value
		case "commit":
			current.Commit = value
		case "git-commit":
			current.GitCommit = value
		case "shallow":
			current.Shallow = value == "true"
		case "freeze":
			current.Freeze = value == "true"
		case "ignore":
			current.Ignore = value
		}
	}

	if current != nil {
		if err := validateConfig(current); err != nil {
			return nil, fmt.Errorf("line %d: invalid config: %w", lineNum, err)
		}
		configs = append(configs, *current)
	}

	return configs, scanner.Err()
}

func WriteIvaldimodules(path string, configs []Config) error {
	var buf bytes.Buffer

	buf.WriteString("# .ivaldimodules - Ivaldi Submodule Configuration\n")
	buf.WriteString("# Version: 1\n\n")

	for _, cfg := range configs {
		buf.WriteString(fmt.Sprintf("[submodule \"%s\"]\n", cfg.Name))
		buf.WriteString(fmt.Sprintf("\tpath = %s\n", cfg.Path))
		buf.WriteString(fmt.Sprintf("\turl = %s\n", cfg.URL))

		if cfg.Timeline != "" {
			buf.WriteString(fmt.Sprintf("\ttimeline = %s\n", cfg.Timeline))
		}

		if cfg.Commit != "" {
			buf.WriteString(fmt.Sprintf("\tcommit = %s\n", cfg.Commit))
		}

		if cfg.GitCommit != "" {
			buf.WriteString(fmt.Sprintf("\tgit-commit = %s\n", cfg.GitCommit))
		}

		if cfg.Shallow {
			buf.WriteString("\tshallow = true\n")
		}

		if cfg.Freeze {
			buf.WriteString("\tfreeze = true\n")
		}

		if cfg.Ignore != "" {
			buf.WriteString(fmt.Sprintf("\tignore = %s\n", cfg.Ignore))
		}

		buf.WriteByte('\n')
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func validateConfig(cfg *Config) error {
	if cfg.Path == "" {
		return fmt.Errorf("missing path for submodule %s", cfg.Name)
	}

	if strings.Contains(cfg.Path, "..") {
		return fmt.Errorf("invalid path (contains ..): %s", cfg.Path)
	}

	if filepath.IsAbs(cfg.Path) {
		return fmt.Errorf("path must be relative: %s", cfg.Path)
	}

	if cfg.URL == "" {
		return fmt.Errorf("missing URL for submodule %s", cfg.Name)
	}

	if !strings.HasPrefix(cfg.URL, "https://") &&
		!strings.HasPrefix(cfg.URL, "ssh://") &&
		!strings.HasPrefix(cfg.URL, "file://") &&
		!strings.HasPrefix(cfg.URL, "git@") {
		return fmt.Errorf("invalid URL protocol: %s", cfg.URL)
	}

	if cfg.Commit != "" && len(cfg.Commit) != 64 {
		return fmt.Errorf("invalid commit hash (expected 64 hex chars): %s", cfg.Commit)
	}

	if cfg.GitCommit != "" && len(cfg.GitCommit) != 40 {
		return fmt.Errorf("invalid git-commit hash (expected 40 hex chars): %s", cfg.GitCommit)
	}

	return nil
}

func ConfigToSubmodule(cfg Config) (*Submodule, error) {
	sub := &Submodule{
		Name:      cfg.Name,
		Path:      cfg.Path,
		URL:       cfg.URL,
		Timeline:  cfg.Timeline,
		GitCommit: cfg.GitCommit,
		Shallow:   cfg.Shallow,
		Freeze:    cfg.Freeze,
	}

	if cfg.Commit != "" {
		hashBytes, err := hex.DecodeString(cfg.Commit)
		if err != nil {
			return nil, fmt.Errorf("decode commit hash: %w", err)
		}
		if len(hashBytes) != 32 {
			return nil, fmt.Errorf("invalid hash length: %d", len(hashBytes))
		}
		copy(sub.Commit[:], hashBytes)
	}

	if sub.Timeline == "" {
		sub.Timeline = "main"
	}

	return sub, nil
}

func SubmoduleToConfig(sub *Submodule) Config {
	return Config{
		Name:      sub.Name,
		Path:      sub.Path,
		URL:       sub.URL,
		Timeline:  sub.Timeline,
		Commit:    hex.EncodeToString(sub.Commit[:]),
		GitCommit: sub.GitCommit,
		Shallow:   sub.Shallow,
		Freeze:    sub.Freeze,
	}
}

func ParseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
