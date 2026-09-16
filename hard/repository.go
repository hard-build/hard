package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

var errDependencySetChanged = errors.New("dependency set expanded; repeat source analysis")

type dependencySession struct {
	project          *projectFile
	provider         *repositoryProvider
	root             string
	locked           bool
	record           bool
	directory        *os.File
	onCommit         func() error
	viewLock         *os.File
	workingDirectory string
	layout           *cacheLayout
	discovery        *dependencyDiscovery

	mutex        sync.Mutex
	pins         map[string]repositoryPin
	snapshots    map[string]string
	selected     map[string]string
	manifests    map[string]*projectFile
	requirements map[string]map[string]repositoryRequirement
	failure      error
	changed      bool
	dirty        bool
	committed    bool
}

func newDependencySession(project *projectFile, root string, options projectOptions, provider *repositoryProvider) (*dependencySession, error) {
	root, err := ensureCacheDirectory(root, ".")
	if err != nil {
		return nil, err
	}
	session := &dependencySession{
		project: project, provider: provider, root: root, locked: options.locked,
		record:           project.recorded || options.lock,
		workingDirectory: filepath.Dir(project.filename),
		pins:             make(map[string]repositoryPin), snapshots: make(map[string]string),
		manifests: make(map[string]*projectFile), requirements: make(map[string]map[string]repositoryRequirement),
		dirty: !project.recorded,
	}
	updates := make(map[string]string)
	for _, update := range options.updates {
		name, ref, found := strings.Cut(update, "@")
		if !found || !validLogicalRepository(name) || !validRepositoryRef(ref) {
			return nil, fmt.Errorf("invalid --update %q; expected github.com/owner/repository@ref", update)
		}
		if _, duplicate := updates[name]; duplicate {
			return nil, fmt.Errorf("duplicate --update for %s", name)
		}
		if _, exists := project.Repositories[name]; !exists {
			return nil, fmt.Errorf("cannot update unrecorded repository %s; fetch --lock first", name)
		}
		updates[name] = ref
	}
	for name, pin := range project.Repositories {
		replacement, replaced := provider.configuration.Replace[name]
		if ref, update := updates[name]; update {
			source := pin.Source
			if replaced {
				source = replacement.Source
				if replacement.Ref != "" && replacement.Ref != ref {
					return nil, fmt.Errorf("update ref conflicts with replacement rule for %s", name)
				}
			}
			resolved, err := provider.resolve(source, ref)
			if err != nil {
				return nil, fmt.Errorf("update %s: %w", name, err)
			}
			// Even an explicit update cannot bless changed bytes at an existing revision.
			if resolved.Source == pin.Source && resolved.Commit == pin.Commit {
				resolved.Checksum = pin.Checksum
			}
			pin = resolved
			session.dirty = true
		} else if replaced && (replacement.Source != pin.Source || replacement.Ref != "" && replacement.Ref != pin.Ref) {
			return nil, fmt.Errorf("replacement conflicts with recorded %s; use fetch --update=%s@ref", name, name)
		}
		session.pins[name] = pin
	}
	return session, nil
}

func (session *dependencySession) close() {
	if session != nil && session.viewLock != nil {
		_ = session.viewLock.Close()
		session.viewLock = nil
	}
	session.closeProjectFile()
}

func (session *dependencySession) closeProjectFile() {
	if session != nil && session.directory != nil {
		_ = session.directory.Close()
		session.directory = nil
	}
}

func (session *dependencySession) ensure(repository githubRepository, progress *progressBar, parents ...string) error {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	name := "github.com/" + repository.key()
	if !validLogicalRepository(name) {
		return fmt.Errorf("invalid logical repository %q", name)
	}
	if session.failure != nil {
		return session.failure
	}
	var parent string
	if len(parents) != 0 {
		parent = parents[0]
	}
	required, err := session.inheritedRequirement(name, parent)
	if err != nil {
		if !errors.Is(err, errDependencySetChanged) {
			session.failure = err
		}
		return err
	}
	pin, exists := session.pins[name]
	if required != nil && exists && pin != required.pin {
		// Inherited requirements apply only to unrecorded dependencies. Replace a
		// provisional default-branch choice before the project record is written.
		exists = false
	}
	if session.locked && !exists {
		return fmt.Errorf("--locked: repository %s is not recorded in %s", name, session.project.filename)
	}
	if !exists {
		if required != nil {
			pin = required.pin
		} else {
			source, ref := name, ""
			if replacement, ok := session.provider.configuration.Replace[name]; ok {
				source, ref = replacement.Source, replacement.Ref
			}
			if ref == "" {
				pin, err = session.defaultPin(source, progress)
			} else {
				if progress != nil {
					progress.updateStep("Resolving " + name)
				}
				pin, err = session.provider.resolve(source, ref)
			}
			if err != nil {
				return fmt.Errorf("resolve %s: %w", name, err)
			}
		}
		snapshot, checksum, err := session.obtain(pin, progress)
		if err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
		pin.Checksum = checksum
		session.pins[name], session.snapshots[name] = pin, snapshot
		session.dirty = true
		session.changed = true
		return errDependencySetChanged
	}
	if session.selected[name] != "" && session.selected[name] == session.snapshots[name] {
		return nil
	}
	session.changed = true
	return errDependencySetChanged
}

func (session *dependencySession) view(configuration configuration, progress *progressBar) (string, error) {
	if session.failure != nil {
		return "", session.failure
	}
	view, err := localProjectRoot(session.root, configuration.env, session.workingDirectory)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(session.root, view)
	if err != nil {
		return "", err
	}
	view, err = ensureCacheDirectory(session.root, relative)
	if err != nil {
		return "", err
	}
	if session.viewLock == nil {
		session.viewLock, err = lockProjectDirectory(filepath.Join(view, "include"))
		if err != nil {
			return "", err
		}
		session.restoreDiscoveryCache()
	}
	session.changed = false
	session.selected = make(map[string]string)
	names := make([]string, 0, len(session.pins))
	for name := range session.pins {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pin := session.pins[name]
		if session.snapshots[name] == "" {
			snapshot, checksum, err := session.obtain(pin, progress)
			if err != nil {
				return "", fmt.Errorf("repository %s: %w", name, err)
			}
			pin.Checksum = checksum
			session.pins[name], session.snapshots[name] = pin, snapshot
		}
		session.selected[name] = session.snapshots[name]
	}
	if err := session.prepareIncludeView(view); err != nil {
		return "", err
	}
	session.layout, err = newCacheLayout(session, configuration, view)
	if err != nil {
		return "", err
	}
	return view, nil
}

func (session *dependencySession) commit() error {
	if session == nil || session.committed {
		return nil
	}
	if session.failure != nil {
		return session.failure
	}
	if session.changed {
		return errDependencySetChanged
	}
	for name, snapshot := range session.snapshots {
		checksum, err := repositoryTreeChecksum(snapshot)
		if err != nil {
			return err
		}
		if checksum != session.pins[name].Checksum {
			return fmt.Errorf("checksum mismatch after dependency preparation for %s", name)
		}
	}
	if session.dirty && session.record {
		if session.locked {
			return errors.New("--locked: dependency record would change")
		}
		if err := session.project.writeRepositories(session.pins); err != nil {
			return err
		}
	}
	if err := session.storeDiscoveryCache(); err != nil {
		return err
	}
	session.committed = true
	session.closeProjectFile()
	if session.onCommit != nil {
		return session.onCommit()
	}
	return nil
}

func (session *dependencySession) obtain(pin repositoryPin, progress *progressBar) (string, string, error) {
	if !repositoryCommitPattern.MatchString(pin.Commit) {
		return "", "", errors.New("invalid snapshot commit")
	}
	parent, err := snapshotSourceDirectory(session.root, pin.Source)
	if err != nil {
		return "", "", err
	}
	// Serialize installation and validation of snapshots from this source.
	lock, err := lockProjectDirectory(filepath.Join(parent, "snapshot"))
	if err != nil {
		return "", "", err
	}
	defer lock.Close()
	return session.obtainLocked(parent, pin, progress)
}

// The caller holds the source-directory lock, including when publishing @default.
func (session *dependencySession) obtainLocked(parent string, pin repositoryPin, progress *progressBar) (string, string, error) {
	destination := filepath.Join(parent, "@"+pin.Commit)
	exists, err := existingGitHubRepository(destination)
	if err != nil {
		return "", "", err
	}
	if exists {
		checksum, err := repositoryTreeChecksum(destination)
		if err != nil {
			return "", "", err
		}
		stored, err := readRegularProjectFile(destination + ".checksum")
		if err != nil {
			return "", "", fmt.Errorf("read snapshot checksum: %w", err)
		}
		if checksum != strings.TrimSpace(string(stored)) || pin.Checksum != "" && checksum != pin.Checksum {
			return "", "", fmt.Errorf("checksum mismatch for %s at %s", pin.Source, pin.Commit)
		}
		return destination, checksum, nil
	}
	if progress != nil {
		progress.updateStep("Downloading " + pin.Source + "@" + pin.Commit)
	}
	input, err := session.provider.snapshot(pin)
	if err != nil {
		return "", "", err
	}
	defer input.Close()
	temporary, err := os.MkdirTemp(parent, ".snapshot-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(temporary)
	if err := extractGitHubSnapshot(input, temporary); err != nil {
		return "", "", err
	}
	checksum, err := repositoryTreeChecksum(temporary)
	if err != nil {
		return "", "", err
	}
	if pin.Checksum != "" && checksum != pin.Checksum {
		return "", "", fmt.Errorf("checksum mismatch for %s at %s", pin.Source, pin.Commit)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return "", "", err
	}
	if err := writeCacheRecord(destination+".checksum", []byte(checksum+"\n")); err != nil {
		return "", "", err
	}
	return destination, checksum, nil
}

func repositoryDigest(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
}

// v1 hashes sorted, length-delimited JSON records, not tar/gzip metadata.
// File names, contents, executable bits, directories and symlink targets matter.
func repositoryTreeChecksum(root string) (string, error) {
	digest := sha256.New()
	encoder := json.NewEncoder(digest)
	if err := encoder.Encode("hard-source-tree-v1"); err != nil {
		return "", err
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		kind, value, executable := "directory", "", false
		switch {
		case info.IsDir():
		case info.Mode().IsRegular():
			kind = "file"
			executable = info.Mode()&0o111 != 0
			value, err = digestInputFile(path)
		case info.Mode()&os.ModeSymlink != 0:
			kind = "symlink"
			value, err = os.Readlink(path)
			if err == nil && (filepath.IsAbs(value) || !pathWithin(root, filepath.Join(filepath.Dir(path), value))) {
				return fmt.Errorf("snapshot symlink escapes source tree: %s", relative)
			}
			resolved, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr == nil && !pathWithin(root, resolved) {
				return fmt.Errorf("snapshot symlink resolves outside source tree: %s", relative)
			}
			if resolveErr != nil && !errors.Is(resolveErr, os.ErrNotExist) {
				return resolveErr
			}
		default:
			return fmt.Errorf("unsupported snapshot entry: %s", relative)
		}
		if err != nil {
			return err
		}
		return encoder.Encode([]any{filepath.ToSlash(relative), kind, executable, value})
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func invocationRepositoryResolver(root string, progress *progressBar, supplied []*githubSnapshotResolver) *githubSnapshotResolver {
	if len(supplied) != 0 && supplied[0] != nil {
		return supplied[0]
	}
	return newGitHubSnapshotResolver(root, progress)
}

func (resolver *githubSnapshotResolver) commitDependencies() error {
	return resolver.session.commit()
}

func (resolver *githubSnapshotResolver) dependencySetChanged() bool {
	return resolver.session != nil && resolver.session.changed
}

func prepareProject(parsed *arguments, options projectOptions, root string, workingDirectory string) (*projectFile, *dependencySession, error) {
	filename, err := findProjectFile(workingDirectory)
	if err != nil {
		return nil, nil, err
	}
	if filename == "" {
		if options.locked || len(options.updates) != 0 {
			return nil, nil, errors.New("dependency options require hard.yaml; run fetch --lock first")
		}
		if !options.lock {
			if parsed.command != "format" && (os.Getenv("HARD_CONFIG") != "" || os.Getenv("HARD_PROXY") != "") {
				return nil, nil, errors.New("corporate dependency configuration requires hard.yaml; run fetch --lock first")
			}
			if parsed.command == "format" {
				return nil, nil, nil
			}
		}
		filename = filepath.Join(workingDirectory, projectFilename)
	}
	// All source-processing commands cooperate with dependency updates. Release this
	// lock immediately for format or unpinned commands, otherwise after resolution.
	directory, err := lockProjectDirectory(filename)
	if err != nil {
		return nil, nil, err
	}
	keepLock := false
	defer func() {
		if !keepLock {
			_ = directory.Close()
		}
	}()
	project, err := readProjectFile(filename)
	if errors.Is(err, os.ErrNotExist) && !options.locked && len(options.updates) == 0 {
		project = &projectFile{Version: 1, filename: filename}
		if err = yaml.Unmarshal([]byte("version: 1\n"), &project.document); err != nil {
			return nil, nil, err
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if parsed.command == "format" && !options.explicitFormat && project.Format != "" {
		parsed.format = project.Format
	}
	if parsed.command == "format" {
		return project, nil, nil
	}
	if !project.recorded && (options.locked || len(options.updates) != 0) {
		return nil, nil, errors.New("repositories section is missing; run fetch --lock first")
	}
	if !project.recorded && !options.lock {
		if os.Getenv("HARD_CONFIG") != "" || os.Getenv("HARD_PROXY") != "" {
			return nil, nil, errors.New("corporate dependency configuration requires repositories or fetch --lock")
		}
	}
	providerConfiguration, err := loadRepositoryConfiguration()
	if err != nil {
		return nil, nil, err
	}
	session, err := newDependencySession(project, root, options, newRepositoryProvider(providerConfiguration))
	if err != nil {
		return nil, nil, err
	}
	session.workingDirectory = workingDirectory
	if session.record {
		session.directory = directory
		keepLock = true
	}
	return project, session, nil
}
