package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func projectFetchRecord(t *testing.T, configuration configuration, project, source string) string {
	t.Helper()
	owner, err := localProjectRoot(configuration.root, configuration.env, project)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(owner, "parse", "fetch", "*", source+parseCacheSuffix))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one analysis record for %s: %v, %v", source, matches, err)
	}
	return matches[0]
}

func TestProjectIncludeLockAndConfigurationKeys(t *testing.T) {
	configuration := projectTestConfiguration(t)
	project := t.TempDir()
	filename := writeProjectTestFile(t, project, projectFilename, "version: 1\nrepositories: {}\n")
	source := writeProjectTestFile(t, project, "src/main.cpp", "int main() { return 0; }\n")
	file, err := readProjectFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	session, err := newDependencySession(file, configuration.root, projectOptions{}, newRepositoryProvider(repositoryConfiguration{}))
	if err != nil {
		t.Fatal(err)
	}
	defer session.close()
	owner, err := session.view(configuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := localProjectRoot(configuration.root, "host", project)
	if owner != want {
		t.Fatalf("local owner: %q, want %q", owner, want)
	}
	before := *session.layout
	path, err := before.sourcePath(source, false)
	if err != nil || path != filepath.Join(owner, "build", before.buildKey, "src/main.cpp") {
		t.Fatalf("relative source layout: %s, %v", path, err)
	}
	parsePath, err := before.parsePath(source, false)
	if err != nil || parsePath != filepath.Join(owner, "parse", "build", before.parseBuildKey, "src/main.cpp") {
		t.Fatalf("selection-independent parse layout: %s, %v", parsePath, err)
	}
	session.selected["github.com/demo/library"] = filepath.Join(configuration.root, "snapshot", "github.com/demo/library", "@"+firstCommit)
	withSelection, err := newCacheLayout(session, configuration, owner)
	if err != nil {
		t.Fatal(err)
	}
	selectedParsePath, err := withSelection.parsePath(source, false)
	if err != nil || selectedParsePath != parsePath || withSelection.buildKey == before.buildKey {
		t.Fatalf("snapshot changed parse path or failed to change build key: %s, %v", selectedParsePath, err)
	}
	writeProjectTestFile(t, project, "src/main.cpp", "int main() { return 1; }\n")
	if _, err := session.view(configuration, nil); err != nil {
		t.Fatal(err)
	}
	if session.layout.analysisKey != before.analysisKey || session.layout.buildKey != before.buildKey {
		t.Fatal("ordinary source edit changed configuration directories")
	}
	configuration.ldflags = []string{"-g"}
	if _, err := session.view(configuration, nil); err != nil {
		t.Fatal(err)
	}
	if session.layout.analysisKey != before.analysisKey || session.layout.buildKey == before.buildKey {
		t.Fatal("link flags did not isolate the build configuration")
	}
	configuration.cflags = append(configuration.cflags, "-DOTHER=1")
	if _, err := session.view(configuration, nil); err != nil {
		t.Fatal(err)
	}
	if session.layout.analysisKey == before.analysisKey || session.layout.buildKey == before.buildKey {
		t.Fatal("analysis flags reused incompatible configuration directories")
	}
	probe, err := os.Open(owner)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	assertLocked := func() {
		t.Helper()
		err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("project include view is not locked: %v", err)
		}
	}
	assertLocked()
	if err := session.commit(); err != nil {
		t.Fatal(err)
	}
	assertLocked() // Dependency publication must not unlock the following build.
	session.close()
	if err := syscall.Flock(int(probe.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("project include lock was not released: %v", err)
	}
	for _, environment := range []string{"", ".", "..", "../escape", "a/b", "/absolute"} {
		if _, err := localProjectRoot(configuration.root, environment, project); err == nil {
			t.Fatalf("accepted invalid HARD_ENV %q", environment)
		}
	}
}

func TestProjectIncludeViewReplacesOnlyManagedLinks(t *testing.T) {
	root := t.TempDir()
	owner, _ := localProjectRoot(root, "host", t.TempDir())
	first := filepath.Join(root, "snapshot", "github.com/hard-build/library", "@"+firstCommit)
	second := filepath.Join(root, "snapshot", "github.com/hard-build/library", "@"+secondCommit)
	writeProjectTestFile(t, first, "value.h", "first\n")
	writeProjectTestFile(t, second, "value.h", "second\n")
	session := &dependencySession{selected: map[string]string{"github.com/hard-build/library": first}}
	if err := session.prepareIncludeView(owner); err != nil {
		t.Fatal(err)
	}
	session.selected["github.com/hard-build/library"] = second
	if err := session.prepareIncludeView(owner); err != nil {
		t.Fatal(err)
	}
	if contents := readTestFile(t, filepath.Join(owner, "include", "hard", "value.h")); contents != "second\n" {
		t.Fatalf("include view kept the previous revision: %s", contents)
	}
	delete(session.selected, "github.com/hard-build/library")
	if err := session.prepareIncludeView(owner); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(owner, "include", "hard")); !os.IsNotExist(err) {
		t.Fatalf("stale alias remained visible: %v", err)
	}
	protected := writeProjectTestFile(t, owner, "include/hard", "preserve me\n")
	session.selected["github.com/hard-build/library"] = first
	if err := session.prepareIncludeView(owner); err == nil || readTestFile(t, protected) != "preserve me\n" {
		t.Fatalf("overwrote a non-symlink include entry: %v", err)
	}
}

func TestCacheLayoutWithRootSymlinkAndFork(t *testing.T) {
	configuration := projectTestConfiguration(t)
	realRoot := configuration.root
	alias := filepath.Join(t.TempDir(), "cache")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Fatal(err)
	}
	configuration.root = alias
	project := t.TempDir()
	filename := writeProjectTestFile(t, project, projectFilename, "version: 1\nrepositories: {}\n")
	file, err := readProjectFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	session, err := newDependencySession(file, alias, projectOptions{}, newRepositoryProvider(repositoryConfiguration{}))
	if err != nil {
		t.Fatal(err)
	}
	defer session.close()
	owner, err := session.view(configuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	source := writeProjectTestFile(t, project, "main.cpp", "int main() {}\n")
	path, err := session.layout.sourcePath(source, false)
	if err != nil || !pathWithin(realRoot, path) || !pathWithin(owner, path) {
		t.Fatalf("symlinked HARD_ROOT: %s, %v", path, err)
	}
	snapshot := filepath.Join(realRoot, "snapshot", "git.corp.example/team/fork", "@"+firstCommit)
	source = writeProjectTestFile(t, snapshot, "src/library.cpp", "int library() { return 1; }\n")
	session.selected["github.com/demo/library"] = snapshot
	layout, err := newCacheLayout(session, configuration, owner)
	if err != nil {
		t.Fatal(err)
	}
	path, err = layout.sourcePath(source, true)
	want := filepath.Join(realRoot, "project", "host", "github.com/demo/library", "fetch", layout.analysisKey, "src/library.cpp")
	if err != nil || path != want || layout.analysisKey == session.layout.analysisKey {
		t.Fatalf("fork lost logical owner or selection isolation: %s, %v", path, err)
	}
}
