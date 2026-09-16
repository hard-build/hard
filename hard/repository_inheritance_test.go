package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"
)

func inheritedTestSnapshot(t *testing.T, source, ref, commit string, files map[string]string) (repositoryPin, []byte) {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var entries []githubTestArchiveEntry
	for _, name := range names {
		entries = append(entries, githubTestArchiveEntry{name: "snapshot/" + name, typeflag: tar.TypeReg, mode: 0o644, contents: files[name]})
	}
	archive := githubTestArchive(t, entries)
	root := t.TempDir()
	if err := extractGitHubSnapshot(bytes.NewReader(archive), root); err != nil {
		t.Fatal(err)
	}
	checksum, err := repositoryTreeChecksum(root)
	if err != nil {
		t.Fatal(err)
	}
	return repositoryPin{Source: source, Ref: ref, Commit: commit, Checksum: checksum}, archive
}

func inheritedTestYAML(t *testing.T, pins map[string]repositoryPin) string {
	t.Helper()
	contents, err := yaml.Marshal(projectFile{Version: 1, Format: "format.v1", Exclude: []string{"ignored"}, Repositories: pins})
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

type inheritanceTestProxy struct {
	defaults map[string]repositoryPin
	archives map[string][]byte
	mutex    sync.Mutex
	requests []string
}

func (proxy *inheritanceTestProxy) add(pin repositoryPin, archive []byte) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	proxy.defaults[pin.Source] = pin
	proxy.archives[pin.Source+"@"+pin.Commit] = archive
}

func (proxy *inheritanceTestProxy) serve(response http.ResponseWriter, request *http.Request) {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	source := request.URL.Query().Get("source")
	proxy.requests = append(proxy.requests, request.URL.Path+" "+source+"@"+request.URL.Query().Get("commit"))
	switch request.URL.Path {
	case "/v1/resolve":
		pin, ok := proxy.defaults[source]
		if !ok {
			http.Error(response, "unexpected source", 404)
			return
		}
		_ = json.NewEncoder(response).Encode(map[string]string{"commit": pin.Commit, "ref": pin.Ref})
	case "/v1/snapshot":
		archive, ok := proxy.archives[source+"@"+request.URL.Query().Get("commit")]
		if !ok {
			http.Error(response, "unexpected revision", 404)
			return
		}
		_, _ = response.Write(archive)
	default:
		http.Error(response, "unexpected endpoint", 500)
	}
}

func newInheritanceTestProxy(t *testing.T) *inheritanceTestProxy {
	t.Helper()
	proxy := &inheritanceTestProxy{defaults: make(map[string]repositoryPin), archives: make(map[string][]byte)}
	server := httptest.NewServer(http.HandlerFunc(proxy.serve))
	t.Cleanup(server.Close)
	t.Setenv("HARD_PROXY", server.URL)
	return proxy
}

func (proxy *inheritanceTestProxy) log() string {
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	return strings.Join(proxy.requests, "\n")
}

func TestInheritedRepositoryPins(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	leaf, leafArchive := inheritedTestSnapshot(t, "github.com/demo/leaf", "release", secondCommit, map[string]string{
		"leaf.h": "#pragma once\ninline int leaf_value() { return 7; }\n",
	})
	proxy.add(leaf, leafArchive)
	// Resolving the moving ref would select a different, deliberately unusable revision.
	proxy.defaults[leaf.Source] = repositoryPin{Source: leaf.Source, Ref: "main", Commit: nextCommit}
	middle, middleArchive := inheritedTestSnapshot(t, "github.com/demo/middle", "stable", secondCommit, map[string]string{
		"middle.h":  "#pragma once\n#include <github.com/demo/leaf/leaf.h>\n",
		"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{leaf.Source: leaf, "github.com/demo/unused": leaf}),
	})
	proxy.add(middle, middleArchive)
	proxy.defaults[middle.Source] = repositoryPin{Source: middle.Source, Ref: "main", Commit: nextCommit}
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/hard-build/library", "main", firstCommit, map[string]string{
		"parent.h":  "#pragma once\n#include <github.com/demo/middle/middle.h>\n",
		"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{middle.Source: middle}),
	})
	proxy.add(parent, parentArchive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <hard/parent.h>\nint main() { return leaf_value() == 7 ? 0 : 1; }\n")
	configuration := projectTestConfiguration(t)
	for _, args := range [][]string{{"fetch", "--lock"}, {"run", "--locked"}, {"run", "--locked", "--no-color", "-v"}} {
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
		}
		if len(args) == 4 && !strings.Contains(out, "Parsing app.cpp (CACHED)") {
			t.Fatalf("inherited analysis was not cached: %s", out)
		}
	}
	file, err := readProjectFile(filepath.Join(project, projectFilename))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Repositories) != 3 || file.Repositories[middle.Source] != middle || file.Repositories[leaf.Source] != leaf {
		t.Fatalf("wrong inherited records: %#v", file.Repositories)
	}
	if file.Format != "" || len(file.Exclude) != 0 {
		t.Fatal("inherited project defaults")
	}
	for _, forbidden := range []string{"/v1/resolve " + middle.Source, "/v1/resolve " + leaf.Source, "github.com/demo/unused"} {
		if strings.Contains(proxy.log(), forbidden) {
			t.Fatalf("unexpected request %s:\n%s", forbidden, proxy.log())
		}
	}
	// A cold locked fetch downloads exact snapshots without resolving any refs.
	before, _ := os.ReadFile(file.filename)
	configuration.root = t.TempDir()
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--locked"); err != nil {
		t.Fatalf("cold locked fetch: %v\n%s\n%s", err, out, diagnostics)
	}
	after, _ := os.ReadFile(file.filename)
	if !bytes.Equal(before, after) {
		t.Fatal("locked fetch changed the project")
	}
	t.Run("GoogleTest", func(t *testing.T) {
		if err := exec.Command("pkg-config", "--exists", googleTestPackage).Run(); err != nil {
			t.Skip("GoogleTest not installed")
		}
		writeProjectTestFile(t, project, "value.test.cpp", "#include <gtest/gtest.h>\n#include <hard/parent.h>\nTEST(Inherited, Value) { EXPECT_EQ(leaf_value(), 7); }\n")
		for pass := 0; pass < 2; pass++ {
			out, diagnostics, err := runProjectTestCommand(configuration, "test", "--locked", "-v", "--no-color")
			if err != nil || pass == 1 && !strings.Contains(out, "Testing value.test (CACHED)") {
				t.Fatalf("inherited test pass %d: %v\n%s\n%s", pass, err, out, diagnostics)
			}
		}
	})
}

func TestInheritedRepositoryConflicts(t *testing.T) {
	for _, kind := range []string{"recorded", "two parents", "same commit different ref", "source", "checksum"} {
		for _, reverse := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
				proxy := newInheritanceTestProxy(t)
				pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "release", secondCommit, map[string]string{"shared.h": "#pragma once\n"})
				proxy.add(pin, archive)
				other, otherArchive := inheritedTestSnapshot(t, pin.Source, "next", nextCommit, map[string]string{"shared.h": "#pragma once\n// next\n"})
				proxy.add(other, otherArchive)
				switch kind {
				case "same commit different ref":
					other = pin
					other.Ref = "another-tag"
				case "source":
					other = pin
					other.Source = "git.corp.example/fork/shared"
				case "checksum":
					other = pin
					other.Checksum = "sha256:" + strings.Repeat("0", 64)
				}
				parents := []repositoryPin{}
				for index, required := range []repositoryPin{pin, other} {
					name := []string{"first", "second"}[index]
					parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/"+name, "main", firstCommit, map[string]string{
						"parent.h":  "#pragma once\n#include <github.com/demo/shared/shared.h>\n",
						"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{pin.Source: required}),
					})
					proxy.add(parent, parentArchive)
					parents = append(parents, parent)
				}
				project := t.TempDir()
				withWorkingDirectory(t, project)
				configuration := projectTestConfiguration(t)
				first, second := parents[0], parents[1]
				if reverse {
					first, second = second, first
				}
				args := []string{"fetch", "--lock", "-j4"}
				pins := map[string]repositoryPin{parents[0].Source: parents[0], parents[1].Source: parents[1]}
				// Preselect the correct snapshot for source/checksum errors: this must
				// validate resolved includes rather than only missing-include recovery.
				if kind == "source" || kind == "checksum" || kind == "recorded" {
					pins[pin.Source] = pin
					args = []string{"fetch", "--locked", "-j4"}
				}
				filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, pins))
				writeProjectTestFile(t, project, "app.cpp", "#include <"+first.Source+"/parent.h>\n#include <"+second.Source+"/parent.h>\n")
				before, _ := os.ReadFile(filename)
				out, diagnostics, err := runProjectTestCommand(configuration, args...)
				if kind == "same commit different ref" {
					if err != nil {
						t.Fatalf("equivalent refs conflict: %v\n%s\n%s", err, out, diagnostics)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "repository pin conflict for "+pin.Source) || !strings.Contains(err.Error(), "hard.yaml") || !strings.Contains(err.Error(), pin.Commit) || !strings.Contains(err.Error(), other.Commit) {
					t.Fatalf("missing conflict evidence: %v\n%s\n%s", err, out, diagnostics)
				}
				after, _ := os.ReadFile(filename)
				if !bytes.Equal(before, after) {
					t.Fatal("conflict changed the project")
				}
			})
		}
	}
}

func TestInheritedPinReplacesOnlyProvisionalDefault(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		args         []string
		sameRevision bool
	}{
		{"serial", []string{"fetch", "--lock", "-j1"}, false},
		{"parallel", []string{"fetch", "--lock", "-j4"}, false},
		{"build", []string{"build", "-j4"}, false},
		{"ref spelling", []string{"fetch", "--lock", "-j1"}, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			proxy := newInheritanceTestProxy(t)
			pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "release", secondCommit, map[string]string{"shared.h": "#pragma once\n"})
			proxy.add(pin, archive)
			moving, movingArchive := inheritedTestSnapshot(t, pin.Source, "main", nextCommit, map[string]string{"shared.h": "#pragma once\n// moving\n"})
			if scenario.sameRevision {
				moving, movingArchive = pin, archive
				moving.Ref = "main"
			}
			proxy.add(moving, movingArchive)
			parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
				"parent.h":  "#pragma once\n#include <github.com/demo/shared/shared.h>\n",
				"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{pin.Source: pin}),
			})
			proxy.add(parent, parentArchive)
			project := t.TempDir()
			withWorkingDirectory(t, project)
			writeProjectTestFile(t, project, projectFilename, "version: 1\nrepositories: {}\n")
			writeProjectTestFile(t, project, "a.cpp", "#include <github.com/demo/shared/shared.h>\n")
			writeProjectTestFile(t, project, "b.cpp", "#include <github.com/demo/parent/parent.h>\n")
			if out, diagnostics, err := runProjectTestCommand(projectTestConfiguration(t), scenario.args...); err != nil {
				t.Fatalf("provisional default prevented inheritance: %v\n%s\n%s", err, out, diagnostics)
			}
			file, err := readProjectFile(filepath.Join(project, projectFilename))
			if err != nil || file.Repositories[pin.Source] != pin {
				t.Fatalf("wrong inherited pin: %v, %#v", err, file)
			}
		})
	}
}

func TestInheritedPinUsesOnlyIncludingRepository(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "main", secondCommit, map[string]string{"shared.h": "#pragma once\n"})
	proxy.add(pin, archive)
	unused := pin
	unused.Commit = nextCommit
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
		"parent.h":  "#pragma once\n", // This header does not request shared.
		"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{pin.Source: unused}),
	})
	proxy.add(parent, parentArchive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/parent/parent.h>\n#include <github.com/demo/shared/shared.h>\n")
	if out, diagnostics, err := runProjectTestCommand(projectTestConfiguration(t), "fetch", "--lock"); err != nil {
		t.Fatalf("an unrelated parent's record affected a direct include: %v\n%s\n%s", err, out, diagnostics)
	}
	file, err := readProjectFile(filepath.Join(project, projectFilename))
	if err != nil || file.Repositories[pin.Source] != pin {
		t.Fatalf("wrong direct pin: %v, %#v", err, file)
	}
}

func TestInheritedPinExplicitUpdate(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "release", secondCommit, map[string]string{"shared.h": "#pragma once\n"})
	proxy.add(pin, archive)
	moving, movingArchive := inheritedTestSnapshot(t, pin.Source, "main", nextCommit, map[string]string{"shared.h": "#pragma once\n// moving\n"})
	proxy.add(moving, movingArchive)
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
		"parent.h":  "#pragma once\n#include <github.com/demo/shared/shared.h>\n",
		"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{pin.Source: pin}),
	})
	proxy.add(parent, parentArchive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{parent.Source: parent, pin.Source: moving}))
	writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/parent/parent.h>\n")
	configuration := projectTestConfiguration(t)
	before, _ := os.ReadFile(filename)
	if _, _, err := runProjectTestCommand(configuration, "fetch", "--lock"); err == nil || !strings.Contains(err.Error(), "repository pin conflict") {
		t.Fatalf("existing incompatible pin was accepted: %v", err)
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(before, after) {
		t.Fatal("--lock silently repaired an existing pin")
	}
	proxy.mutex.Lock()
	proxy.defaults[pin.Source] = pin
	proxy.mutex.Unlock()
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--update="+pin.Source+"@release"); err != nil {
		t.Fatalf("explicit repair: %v\n%s\n%s", err, out, diagnostics)
	}
	file, err := readProjectFile(filename)
	if err != nil || file.Repositories[pin.Source] != pin {
		t.Fatalf("explicit repair recorded the wrong pin: %v, %#v", err, file)
	}
	before, _ = os.ReadFile(filename)
	proxy.mutex.Lock()
	proxy.defaults[pin.Source] = moving
	proxy.mutex.Unlock()
	if _, _, err := runProjectTestCommand(configuration, "fetch", "--update="+pin.Source+"@main"); err == nil || !strings.Contains(err.Error(), "repository pin conflict") {
		t.Fatalf("update ignored an inherited requirement: %v", err)
	}
	after, _ = os.ReadFile(filename)
	if !bytes.Equal(before, after) {
		t.Fatal("incompatible update changed the project")
	}
}

func TestInheritedPinReplacementAndCache(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "release", secondCommit, map[string]string{"shared.h": "#pragma once\ninline int value() { return 7; }\n"})
	proxy.add(pin, archive)
	fork, forkArchive := inheritedTestSnapshot(t, "git.corp.example/fork/shared", "company", nextCommit, map[string]string{"shared.h": "#pragma once\ninline int value() { return 7; }\n"})
	proxy.add(fork, forkArchive)
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
		"parent.h":  "#pragma once\n#include <github.com/demo/shared/shared.h>\n",
		"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{pin.Source: pin}),
	})
	proxy.add(parent, parentArchive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/parent/parent.h>\nint main() { return value() == 7 ? 0 : 1; }\n")
	configuration := projectTestConfiguration(t)
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock"); err != nil {
		t.Fatalf("initial pins: %v\n%s\n%s", err, out, diagnostics)
	}
	external := writeProjectTestFile(t, t.TempDir(), "config.yaml", "replace:\n  "+pin.Source+":\n    source: "+fork.Source+"\n    ref: "+fork.Ref+"\n")
	t.Setenv("HARD_CONFIG", external)
	if _, _, err := runProjectTestCommand(configuration, "fetch"); err == nil || !strings.Contains(err.Error(), "replacement conflicts") {
		t.Fatalf("replacement changed a recorded pin without --update: %v", err)
	}
	for _, args := range [][]string{{"fetch", "--update=" + pin.Source + "@company"}, {"run", "--locked"}, {"run", "--locked", "--no-color", "-v"}} {
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil {
			t.Fatalf("replacement %v: %v\n%s\n%s", args, err, out, diagnostics)
		}
		if len(args) == 4 && !strings.Contains(out, "Parsing app.cpp (CACHED)") {
			t.Fatalf("replacement cache missed: %s", out)
		}
	}
	filename := filepath.Join(project, projectFilename)
	before, _ := os.ReadFile(filename)
	t.Setenv("HARD_CONFIG", "")
	if out, diagnostics, err := runProjectTestCommand(configuration, "run", "--locked"); err == nil || !strings.Contains(err.Error(), "repository pin conflict") || strings.Contains(out, "Parsing app.cpp (CACHED)") {
		t.Fatalf("cache hid a removed replacement: %v\n%s\n%s", err, out, diagnostics)
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(before, after) {
		t.Fatal("removed replacement rewrote pins")
	}
	// The same explicit replacement is also allowed on initial automatic addition.
	t.Setenv("HARD_CONFIG", external)
	otherProject := t.TempDir()
	withWorkingDirectory(t, otherProject)
	writeProjectTestFile(t, otherProject, "app.cpp", "#include <github.com/demo/parent/parent.h>\n")
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock"); err != nil {
		t.Fatalf("initial fork: %v\n%s\n%s", err, out, diagnostics)
	}
	file, err := readProjectFile(filepath.Join(otherProject, projectFilename))
	if err != nil || file.Repositories[pin.Source] != fork {
		t.Fatalf("initial replacement was not recorded: %v, %#v", err, file)
	}
}

func TestInheritedPinFailuresDoNotWrite(t *testing.T) {
	for _, kind := range []string{"checksum", "malformed manifest", "locked missing"} {
		t.Run(kind, func(t *testing.T) {
			proxy := newInheritanceTestProxy(t)
			pin, archive := inheritedTestSnapshot(t, "github.com/demo/shared", "release", secondCommit, map[string]string{"shared.h": "#pragma once\n"})
			proxy.add(pin, archive)
			if kind == "checksum" {
				pin.Checksum = "sha256:" + strings.Repeat("0", 64)
			}
			manifest := inheritedTestYAML(t, map[string]repositoryPin{pin.Source: pin})
			if kind == "malformed manifest" {
				manifest = "version: 2\nrepositories: {}\n"
			}
			parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
				"parent.h": "#pragma once\n#include <github.com/demo/shared/shared.h>\n", "hard.yaml": manifest,
			})
			proxy.add(parent, parentArchive)
			project := t.TempDir()
			withWorkingDirectory(t, project)
			filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{parent.Source: parent}))
			writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/parent/parent.h>\n")
			before, _ := os.ReadFile(filename)
			args := []string{"fetch"}
			if kind == "locked missing" {
				args = append(args, "--locked")
			}
			out, diagnostics, err := runProjectTestCommand(projectTestConfiguration(t), args...)
			wanted := map[string]string{"checksum": "checksum mismatch", "malformed manifest": "version must be 1", "locked missing": "--locked"}[kind]
			if err == nil || !strings.Contains(err.Error(), wanted) {
				t.Fatalf("missing %s error: %v\n%s\n%s", kind, err, out, diagnostics)
			}
			after, _ := os.ReadFile(filename)
			if !bytes.Equal(before, after) || strings.Contains(proxy.log(), "/v1/resolve "+pin.Source) {
				t.Fatal("failure wrote pins or fell back to default ref")
			}
			if kind == "locked missing" && strings.Contains(proxy.log(), "/v1/snapshot "+pin.Source) {
				t.Fatal("locked fetched an unrecorded inherited pin")
			}
		})
	}
}
