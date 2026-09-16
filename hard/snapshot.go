package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot paths preserve the actual source identity. Reserved @ names keep
// revision records separate from nested corporate repository paths.
func snapshotSourceDirectory(root, source string) (string, error) {
	if !validRepositorySource(source) {
		return "", fmt.Errorf("invalid snapshot source %q", source)
	}
	return ensureCacheDirectory(root, filepath.Join("snapshot", filepath.FromSlash(source)))
}

// HARD_ROOT itself may be an intentional symlink, but no managed descendant
// may redirect a cache write outside that root.
func ensureCacheDirectory(root, relative string) (string, error) {
	if filepath.IsAbs(relative) || filepath.Clean(relative) != relative || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid cache directory")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	parent := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		parent = filepath.Join(parent, component)
		if err := os.Mkdir(parent, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		info, err := os.Lstat(parent)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("cache parent is not a real directory: %s", parent)
		}
	}
	return parent, nil
}

func (session *dependencySession) defaultPin(source string, progress *progressBar) (repositoryPin, error) {
	parent, err := snapshotSourceDirectory(session.root, source)
	if err != nil {
		return repositoryPin{}, err
	}
	lock, err := lockProjectDirectory(filepath.Join(parent, "@default"))
	if err != nil {
		return repositoryPin{}, err
	}
	defer lock.Close()
	filename := filepath.Join(parent, "@default")
	contents, err := readRegularProjectFile(filename)
	if err == nil {
		commit := strings.TrimSuffix(string(contents), "\n")
		if !repositoryCommitPattern.MatchString(commit) {
			return repositoryPin{}, fmt.Errorf("invalid default snapshot commit for %s", source)
		}
		pin := repositoryPin{Source: source, Ref: commit, Commit: commit}
		_, pin.Checksum, err = session.obtainLocked(parent, pin, progress)
		return pin, err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return repositoryPin{}, fmt.Errorf("read default snapshot for %s: %w", source, err)
	}
	if progress != nil {
		progress.updateStep("Resolving " + source)
	}
	pin, err := session.provider.resolve(source, "")
	if err != nil {
		return repositoryPin{}, err
	}
	_, pin.Checksum, err = session.obtainLocked(parent, pin, progress)
	if err != nil {
		return repositoryPin{}, err
	}
	if err := writeCacheRecord(filename, []byte(pin.Commit+"\n")); err != nil {
		return repositoryPin{}, fmt.Errorf("write default snapshot for %s: %w", source, err)
	}
	return pin, nil
}
