package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func discoverSources(command string, paths []string) ([]string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine working directory: %w", err)
	}
	return discoverSourcesFrom(command, paths, workingDirectory)
}

func discoverSourcesFrom(command string, paths []string, workingDirectory string) ([]string, error) {
	if _, err := matchesSource(command, ""); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var sources []string
	seen := make(map[string]struct{})
	add := func(path string) error {
		matches, err := matchesSource(command, path)
		if err != nil || !matches {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		realPath, err = filepath.Abs(realPath)
		if err != nil {
			return err
		}
		if _, ok := seen[realPath]; ok {
			return nil
		}
		relativePath, err := filepath.Rel(workingDirectory, path)
		if err != nil {
			return err
		}
		seen[realPath] = struct{}{}
		sources = append(sources, filepath.Clean(relativePath))
		return nil
	}

	if err := discoverPaths(paths, workingDirectory, add); err != nil {
		return nil, err
	}
	return sources, nil
}

func discoverPaths(paths []string, workingDirectory string, add func(string) error) error {
	visitedDirectories := make(map[string]struct{})
	var walkDirectory func(string) error
	walkDirectory = func(path string) error {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		realPath, err = filepath.Abs(realPath)
		if err != nil {
			return err
		}
		if _, ok := visitedDirectories[realPath]; ok {
			return nil
		}
		visitedDirectories[realPath] = struct{}{}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			entryPath := filepath.Join(path, entry.Name())
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Stat(entryPath)
				if err != nil {
					if err := add(entryPath); err != nil {
						return err
					}
					continue
				}
				if target.IsDir() {
					if err := walkDirectory(entryPath); err != nil {
						return err
					}
					continue
				}
			} else if info.IsDir() {
				if err := walkDirectory(entryPath); err != nil {
					return err
				}
				continue
			}
			if err := add(entryPath); err != nil {
				return err
			}
		}
		return nil
	}

	for _, input := range paths {
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDirectory, path)
		}
		path = filepath.Clean(path)

		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("%s: %w", input, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("%s: %w", input, err)
			}
			if target.IsDir() {
				if err := walkDirectory(path); err != nil {
					return fmt.Errorf("%s: %w", input, err)
				}
				continue
			}
			if err := add(path); err != nil {
				return fmt.Errorf("%s: %w", input, err)
			}
			continue
		}
		if !info.IsDir() {
			if err := add(path); err != nil {
				return fmt.Errorf("%s: %w", input, err)
			}
			continue
		}

		if err := walkDirectory(path); err != nil {
			return fmt.Errorf("%s: %w", input, err)
		}
	}
	return nil
}

func matchesSource(command, path string) (bool, error) {
	extension := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	stem := strings.TrimSuffix(base, extension)
	isTest := strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, "_test")

	switch command {
	case "build", "run":
		return isTranslationUnit(extension) && !isTest, nil
	case "fetch":
		return isTranslationUnit(extension), nil
	case "format":
		return isTranslationUnit(extension) || isHeader(extension), nil
	case "test":
		return isTranslationUnit(extension) && isTest, nil
	default:
		return false, fmt.Errorf("unknown command: %s", command)
	}
}

func isTranslationUnit(extension string) bool {
	switch extension {
	case ".c", ".cc", ".cpp", ".c++":
		return true
	default:
		return false
	}
}

func isHeader(extension string) bool {
	switch extension {
	case ".h", ".hh", ".hpp", ".h++":
		return true
	default:
		return false
	}
}
