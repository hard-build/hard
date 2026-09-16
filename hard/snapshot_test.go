package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDefaultSnapshotSelectionAndConcurrentReuse(t *testing.T) {
	server, requests := newRepositoryTestProxy(t)
	var configuration repositoryConfiguration
	configuration.Proxy.URL = server.URL
	root := t.TempDir()
	session := &dependencySession{root: root, provider: newRepositoryProvider(configuration)}
	const source = "github.com/demo/first"
	legacy := writeProjectTestFile(t, root, "source/"+source+"/first.h", "legacy bytes\n")
	old := writeProjectTestFile(t, root, "snapshot/"+repositoryDigest([]byte(source))+"/"+firstCommit+"/first.h", "old hashed cache\n")
	var wait sync.WaitGroup
	results := make([]repositoryPin, 8)
	failures := make([]error, len(results))
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], failures[index] = session.defaultPin(source, nil)
		}(index)
	}
	wait.Wait()
	for index, pin := range results {
		if failures[index] != nil || pin.Source != source || pin.Commit != firstCommit || !repositoryChecksumPattern.MatchString(pin.Checksum) {
			t.Fatalf("selection %d: %#v, %v", index, pin, failures[index])
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("default was resolved or downloaded more than once: %d", requests.Load())
	}
	parent := filepath.Join(root, "snapshot", filepath.FromSlash(source))
	filename := filepath.Join(parent, "@default")
	info, err := os.Lstat(filename)
	if err != nil || !info.Mode().IsRegular() || readTestFile(t, filename) != firstCommit+"\n" {
		t.Fatalf("default is not a regular commit record: %v", err)
	}
	if _, err := os.Lstat(filename + ".checksum"); !os.IsNotExist(err) {
		t.Fatalf("unexpected @default.checksum: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "@"+firstCommit+".checksum")); err != nil {
		t.Fatal(err)
	}
	pin, err := session.provider.resolve(source, "next")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := session.obtain(pin, nil); err != nil {
		t.Fatal(err)
	}
	if readTestFile(t, filename) != firstCommit+"\n" {
		t.Fatal("explicit revision changed the cached default")
	}
	before := requests.Load()
	selected, err := session.defaultPin(source, nil)
	if err != nil || selected.Commit != firstCommit || selected.Ref != firstCommit || requests.Load() != before {
		t.Fatalf("cached default was refreshed: %#v, %v", selected, err)
	}
	if readTestFile(t, legacy) != "legacy bytes\n" || readTestFile(t, old) != "old hashed cache\n" {
		t.Fatal("migration changed an old cache")
	}
}

func TestSnapshotPathsAndCorruption(t *testing.T) {
	server, requests := newRepositoryTestProxy(t)
	var configuration repositoryConfiguration
	configuration.Proxy.URL = server.URL
	for _, corruption := range []string{"commit", "default symlink", "default directory", "source", "checksum", "parent symlink"} {
		t.Run(corruption, func(t *testing.T) {
			root := t.TempDir()
			session := &dependencySession{root: root, provider: newRepositoryProvider(configuration)}
			const source = "git.corp.example/team/fork"
			parent := filepath.Join(root, "snapshot", filepath.FromSlash(source))
			if corruption == "parent symlink" {
				outside := t.TempDir()
				if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, parent); err != nil {
					t.Fatal(err)
				}
				if _, err := session.defaultPin(source, nil); err == nil {
					t.Fatal("accepted redirected source parent")
				}
				entries, err := os.ReadDir(outside)
				if err != nil || len(entries) != 0 {
					t.Fatalf("wrote outside HARD_ROOT: %v, %v", entries, err)
				}
				return
			}
			pin, err := session.defaultPin(source, nil)
			if err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(parent, "@default")
			switch corruption {
			case "commit":
				writeProjectTestFile(t, parent, "@default", "../escape\n")
			case "default symlink", "default directory":
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
				if corruption == "default symlink" {
					target := writeProjectTestFile(t, t.TempDir(), "commit", pin.Commit+"\n")
					err = os.Symlink(target, filename)
				} else {
					err = os.Mkdir(filename, 0o755)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "source":
				writeProjectTestFile(t, parent, "@"+pin.Commit+"/first.h", "changed\n")
			case "checksum":
				writeProjectTestFile(t, parent, "@"+pin.Commit+".checksum", "sha256:"+strings.Repeat("0", 64)+"\n")
			}
			before := requests.Load()
			if _, err := session.defaultPin(source, nil); err == nil {
				t.Fatal("accepted corrupted default snapshot")
			}
			if requests.Load() != before {
				t.Fatal("corruption triggered a revision refresh")
			}
		})
	}
	root := t.TempDir()
	for _, source := range []string{"github.com/../escape", "/github.com/a/b", "github.com/a/@default", "github.com/a/b/../../escape"} {
		if _, err := snapshotSourceDirectory(root, source); err == nil {
			t.Fatalf("accepted source %q", source)
		}
	}
	for _, source := range []string{"git.corp.example/team/fork", "git.corp.example/team/fork/nested"} {
		session := &dependencySession{root: root, provider: newRepositoryProvider(configuration)}
		if _, err := session.defaultPin(source, nil); err != nil {
			t.Fatalf("nested repository name collided with metadata: %v", err)
		}
	}
}

func TestUnrecordedCommandsUseSnapshotsWithoutWritingProject(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pin, archive := inheritedTestSnapshot(t, "github.com/demo/value", "main", firstCommit, map[string]string{
		"value.h":   "#pragma once\nint value();\n",
		"value.cpp": "#include \"value.h\"\nint value() { return 7; }\n",
	})
	proxy.add(pin, archive)
	providerConfiguration, err := loadRepositoryConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	configuration := projectTestConfiguration(t)
	seed := &dependencySession{root: configuration.root, provider: newRepositoryProvider(providerConfiguration)}
	if _, err := seed.defaultPin(pin.Source, nil); err != nil {
		t.Fatal(err)
	}
	// Public unrecorded commands must work offline using only @default.
	t.Setenv("HARD_PROXY", "")
	t.Setenv("HARD_CONFIG", "")
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "main.cpp", "#include <github.com/demo/value/value.h>\nint main() { return value() == 7 ? 0 : 1; }\n")
	for _, existing := range []bool{false, true} {
		var before []byte
		if existing {
			before = []byte("# Manual configuration\nversion: 1\nexclude: [ignored]\n")
			writeProjectTestFile(t, project, projectFilename, string(before))
		}
		for _, args := range [][]string{{"fetch", "-v"}, {"build", "-v"}, {"run", "--no-cache"}} {
			if out, diagnostics, err := runProjectTestCommand(configuration, args...); err != nil {
				t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
			}
		}
		after, err := os.ReadFile(filepath.Join(project, projectFilename))
		if existing && (err != nil || !bytes.Equal(before, after)) || !existing && !os.IsNotExist(err) {
			t.Fatalf("unrecorded commands changed hard.yaml: %v", err)
		}
	}
	for _, old := range []string{"source", "env", "fetch"} {
		if _, err := os.Lstat(filepath.Join(configuration.root, old)); !os.IsNotExist(err) {
			t.Fatalf("created legacy directory %s: %v", old, err)
		}
	}
	owner, _ := localProjectRoot(configuration.root, configuration.env, project)
	if destination, err := filepath.EvalSymlinks(filepath.Join(owner, "include", filepath.FromSlash(pin.Source))); err != nil || destination != filepath.Join(configuration.root, "snapshot", filepath.FromSlash(pin.Source), "@"+pin.Commit) {
		t.Fatalf("include does not select a snapshot: %s, %v", destination, err)
	}
	objects, err := filepath.Glob(filepath.Join(configuration.root, "project", "host", filepath.FromSlash(pin.Source), "build", "*", "value.cpp.o"))
	if err != nil || len(objects) != 1 {
		t.Fatalf("library object not owned by the logical repository: %v, %v", objects, err)
	}
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock"); err != nil {
		t.Fatalf("record cached default: %v\n%s\n%s", err, out, diagnostics)
	}
	file, err := readProjectFile(filepath.Join(project, projectFilename))
	if err != nil || file.Repositories[pin.Source].Commit != pin.Commit || file.Repositories[pin.Source].Ref != pin.Commit {
		t.Fatalf("cached default not recorded exactly: %#v, %v", file, err)
	}
}
