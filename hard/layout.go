package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A layout is immutable during an analysis attempt. In particular, changing a
// selected snapshot changes the keys even though the include directory is fixed.
type cacheLayout struct {
	root, environment, directory, owner string
	analysisKey, buildKey               string
	parseAnalysisKey, parseBuildKey     string
	snapshots                           map[string]string
}

func projectEnvironmentRoot(root, environment string) (string, error) {
	if !validGitHubPathSegment(environment) || strings.ContainsAny(environment, "/\\") {
		return "", fmt.Errorf("invalid HARD_ENV directory: %s", environment)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "project", environment), nil
}

func localProjectRoot(root, environment, directory string) (string, error) {
	parent, err := projectEnvironmentRoot(root, environment)
	if err != nil {
		return "", err
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, "root", strings.TrimLeft(directory, string(filepath.Separator))), nil
}

func newCacheLayout(session *dependencySession, configuration configuration, owner string) (*cacheLayout, error) {
	layout := &cacheLayout{
		root: session.root, environment: configuration.env,
		directory: session.workingDirectory, owner: owner,
		snapshots: make(map[string]string),
	}
	for name, snapshot := range session.selected {
		layout.snapshots[name] = snapshot
	}
	// Keep the project include context in both keys: it can affect __FILE__,
	// relative flags and quoted includes. Package fingerprints are independent.
	analysisContext := struct {
		Version                            int
		Directory, Include, Runtime, Clang string
		CFlags                             []string
		Snapshots                          map[string]string
	}{1, layout.directory, filepath.Join(owner, "include"), configuration.runtimeRoot, clangVersion(), configuration.cflags, make(map[string]string)}
	parseAnalysis, err := json.Marshal(analysisContext)
	if err != nil {
		return nil, err
	}
	layout.parseAnalysisKey = repositoryDigest(parseAnalysis)
	analysisContext.Snapshots = layout.snapshots
	analysis, err := json.Marshal(analysisContext)
	if err != nil {
		return nil, err
	}
	layout.analysisKey = repositoryDigest(analysis)
	buildContext := struct {
		Analysis, Compiler, Suffix string
		LDFlags, Entries           []string
	}{layout.parseAnalysisKey, configuration.cc, configuration.executableSuffix, configuration.ldflags, configuration.entrypoints}
	parseBuild, err := json.Marshal(buildContext)
	if err != nil {
		return nil, err
	}
	layout.parseBuildKey = repositoryDigest(parseBuild)
	buildContext.Analysis = layout.analysisKey
	build, err := json.Marshal(buildContext)
	if err != nil {
		return nil, err
	}
	layout.buildKey = repositoryDigest(build)
	return layout, nil
}

func (layout *cacheLayout) sourcePath(source string, fetch bool) (string, error) {
	kind, key := "build", layout.buildKey
	if fetch {
		kind, key = "fetch", layout.analysisKey
	}
	return layout.sourcePathWithKey(source, kind, key)
}

func (layout *cacheLayout) parsePath(source string, fetch bool) (string, error) {
	kind, key := filepath.Join("parse", "build"), layout.parseBuildKey
	if fetch {
		kind, key = filepath.Join("parse", "fetch"), layout.parseAnalysisKey
	}
	return layout.sourcePathWithKey(source, kind, key)
}

func (layout *cacheLayout) sourcePathWithKey(source, kind, key string) (string, error) {
	canonical, err := realAbsolutePath(source, layout.directory)
	if err != nil {
		return "", err
	}
	owner, relative := layout.owner, ""
	names := make([]string, 0, len(layout.snapshots))
	for name := range layout.snapshots {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if snapshot := layout.snapshots[name]; pathWithin(snapshot, canonical) {
			parent, err := projectEnvironmentRoot(layout.root, layout.environment)
			if err != nil {
				return "", err
			}
			owner = filepath.Join(parent, filepath.FromSlash(name))
			relative, err = filepath.Rel(snapshot, canonical)
			if err != nil {
				return "", err
			}
			break
		}
	}
	if relative == "" {
		absolute, err := lexicalAbsolutePath(source, layout.directory)
		if err != nil {
			return "", err
		}
		if pathWithin(layout.directory, absolute) {
			relative, err = filepath.Rel(layout.directory, absolute)
			if err != nil {
				return "", err
			}
		} else {
			owner, err = localProjectRoot(layout.root, layout.environment, filepath.Dir(absolute))
			if err != nil {
				return "", err
			}
			relative = filepath.Base(absolute)
		}
	}
	path := filepath.Join(owner, kind, key, relative)
	parent, err := filepath.Rel(layout.root, filepath.Dir(path))
	if err != nil {
		return "", err
	}
	directory, err := ensureCacheDirectory(layout.root, parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, filepath.Base(path)), nil
}

func (session *dependencySession) prepareIncludeView(owner string) error {
	include := filepath.Join(owner, "include")
	if _, err := ensureCacheDirectory(owner, "include"); err != nil {
		return err
	}
	wanted := make(map[string]string)
	for name, snapshot := range session.selected {
		alias := filepath.Join(include, filepath.FromSlash(name))
		wanted[alias] = snapshot
		for _, known := range wellKnownGitHubRepositories {
			if "github.com/"+known.repository.key() == name {
				wanted[filepath.Join(include, known.include)] = alias
			}
		}
	}
	// Remove only managed symlinks. Never replace user files or follow a symlink
	// while walking the namespace. The caller holds the project/environment lock.
	if err := filepath.WalkDir(include, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(include, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink == 0 || !(validLogicalRepository(name) || name == "hard" || name == "recipe") {
			return fmt.Errorf("unexpected project include entry: %s", path)
		}
		if _, selected := wanted[path]; !selected {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		return err
	}
	for alias, destination := range wanted {
		if err := os.MkdirAll(filepath.Dir(alias), 0o755); err != nil {
			return err
		}
		target, err := filepath.Rel(filepath.Dir(alias), destination)
		if err != nil {
			return err
		}
		if existing, err := os.Readlink(alias); err == nil && existing == target {
			continue
		}
		if info, err := os.Lstat(alias); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("project include entry is not a symbolic link: %s", alias)
			}
			if err := os.Remove(alias); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Symlink(target, alias); err != nil {
			return err
		}
	}
	return nil
}
