package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchParseCache(t *testing.T) {
	project := t.TempDir()
	withWorkingDirectory(t, project)
	configuration := projectTestConfiguration(t)
	configuration.cc = "compiler-must-not-run"
	writeProjectTestFile(t, project, "main.cpp", "#include \"value.h\"\nint main() { return value(); }\n")
	writeProjectTestFile(t, project, "value.h", "#pragma once\nint value();\n")
	writeProjectTestFile(t, project, "value.cpp", "#include \"value.h\"\nint value() { return 0; }\n")
	check := func(cached bool, extra ...string) {
		t.Helper()
		args := append([]string{"fetch", "main.cpp", "-v", "--no-color"}, extra...)
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil || diagnostics != "" {
			t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
		}
		for _, source := range []string{"main.cpp", "value.cpp"} {
			want := "[1/?] Parsing " + source
			if cached {
				want += " (CACHED)"
			}
			if !strings.Contains(out, want+"\n") {
				t.Fatalf("missing %q:\n%s", want, out)
			}
		}
		if _, err := os.Stat(filepath.Join(configuration.root, "env")); !os.IsNotExist(err) {
			t.Fatalf("fetch created build artifacts: %v", err)
		}
	}
	check(false)
	check(true)
	check(false, "--no-cache")
	check(true)
	writeProjectTestFile(t, project, "value.h", "#pragma once\nint value();\n// changed\n")
	check(false)
	check(true)
	configuration.cflags = append(configuration.cflags, "-DFETCH_TEST=1")
	check(false)
	check(true)
	configuration.env = "another-target"
	check(false)
	check(true)
}

func TestFetchCacheMissesAndSourceClosure(t *testing.T) {
	project := t.TempDir()
	withWorkingDirectory(t, project)
	configuration := projectTestConfiguration(t)
	writeProjectTestFile(t, project, "main.cpp", "#include \"value.h\"\n")
	writeProjectTestFile(t, project, "value.h", "#pragma once\n")
	run := func(cached bool) string {
		t.Helper()
		out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "main.cpp", "-v", "--no-color", "-j4")
		if err != nil || diagnostics != "" || strings.Contains(out, "Parsing main.cpp (CACHED)") != cached {
			t.Fatalf("cached=%t: %v\n%s\n%s", cached, err, out, diagnostics)
		}
		return out
	}
	run(false)
	run(true)
	// Even a cached header list must discover a newly added implementation.
	writeProjectTestFile(t, project, "value.cpp", "#include \"value.h\"\n")
	if out := run(true); !strings.Contains(out, "Parsing value.cpp\n") {
		t.Fatalf("missed new implementation:\n%s", out)
	}
	if out := run(true); !strings.Contains(out, "Parsing value.cpp (CACHED)") {
		t.Fatalf("implementation not cached:\n%s", out)
	}
	recordPath := projectFetchRecord(t, configuration, project, "main.cpp")
	for _, corrupt := range []string{"json", "version", "kind", "include edges"} {
		t.Run(corrupt, func(t *testing.T) {
			record, ok, err := readParseCacheRecord(recordPath)
			if err != nil || !ok || len(record.Includes) == 0 {
				t.Fatalf("read record: %#v, %v", record, err)
			}
			switch corrupt {
			case "version":
				record.Version++
			case "kind":
				record.Kind = "source-parse"
			case "include edges":
				record.Includes[0].Source = "nonexistent-parent.h"
			}
			contents, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if corrupt == "json" {
				contents = []byte("{")
			}
			writeProjectTestFile(t, filepath.Dir(recordPath), filepath.Base(recordPath), string(contents))
			run(false)
			run(true)
		})
	}
	writeProjectTestFile(t, project, "main.cpp", "#include \"value.h\"\n// source changed\n")
	if out := run(false); !strings.Contains(out, "Parsing value.cpp (CACHED)") {
		t.Fatalf("unrelated implementation lost its cache:\n%s", out)
	}
	run(true)
	if err := os.Remove(filepath.Join(project, "value.h")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runProjectTestCommand(configuration, "fetch", "main.cpp"); err == nil {
		t.Fatal("cached analysis hid a missing header")
	}
	if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
		t.Fatalf("failed analysis retained a cache record: %v", err)
	}
	writeProjectTestFile(t, project, "main.cpp", "#if __has_include(\"optional.h\")\n#include \"optional.h\"\n#endif\n")
	run(false)
	run(false)
	writeProjectTestFile(t, project, "optional.h", "#include \"unavailable.h\"\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "main.cpp"); err == nil {
		t.Fatal("cached optional-header availability")
	}
}

func TestFetchCachePinnedRecipe(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	vendor, archive := inheritedTestSnapshot(t, "github.com/owner/library", "release", secondCommit, map[string]string{
		"library.h":      "#pragma once\nint library_value();\n",
		"library.cpp":    "#include \"library.h\"\nint library_value() { return 1; }\n",
		"CMakeLists.txt": "message(FATAL_ERROR \"fetch must not run CMake\")\n",
	})
	proxy.add(vendor, archive)
	recipe, archive := inheritedTestSnapshot(t, "github.com/hard-build/recipe", "main", firstCommit, map[string]string{
		"library.hard.h": "/* hard.recipe.v1\n" + validLibraryRecipeYAML() + "*/\n#pragma once\n#include <library.h>\n",
		"hard.yaml":      inheritedTestYAML(t, map[string]repositoryPin{vendor.Source: vendor}),
	})
	proxy.add(recipe, archive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	configuration := projectTestConfiguration(t)
	configuration.cc = "compiler-must-not-run"
	writeProjectTestFile(t, project, "main.cpp", "#include <recipe/library.hard.h>\nint main() { return library_value(); }\n")
	run := func(cached bool, extra ...string) {
		t.Helper()
		args := append([]string{"fetch", "--no-color", "-v", "-j4"}, extra...)
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil || diagnostics != "" {
			t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
		}
		for _, source := range []string{"main.cpp", vendor.Source + "/library.cpp"} {
			want := "Parsing " + source
			if cached {
				want += " (CACHED)"
			}
			if !strings.Contains(out, want+"\n") {
				t.Fatalf("missing %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "snapshot/") || strings.Contains(out, "../") {
			t.Fatalf("physical paths in progress:\n%s", out)
		}
		if matches, err := filepath.Glob(filepath.Join(configuration.root, "project", "*", "env")); err != nil || len(matches) != 0 {
			t.Fatalf("fetch created build artifacts: %v, %v", matches, err)
		}
	}
	run(false, "--lock")
	filename := filepath.Join(project, projectFilename)
	before, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	requests := proxy.log()
	run(true, "--lock")
	run(true, "--locked")
	run(false, "--no-cache", "--locked")
	run(true, "--locked")
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(before, after) || proxy.log() != requests {
		t.Fatalf("repeat/--no-cache changed pins or made a network request: %v\n%s", err, proxy.log())
	}
	// A project override must get a different analysis cache, even though the
	// parent recipe still requires the old vendor revision.
	updated, archive := inheritedTestSnapshot(t, vendor.Source, "next", nextCommit, map[string]string{
		"library.h":   "#pragma once\nint library_value();\n",
		"library.cpp": "#include \"library.h\"\nint library_value() { return 2; }\n",
	})
	proxy.add(updated, archive)
	run(false, "--update="+vendor.Source+"@next")
	run(true, "--locked")
	file, err := readProjectFile(filename)
	if err != nil || file.Repositories[vendor.Source] != updated {
		t.Fatalf("wrong updated pin: %#v, %v", file, err)
	}
	before, _ = os.ReadFile(filename)
	requests = proxy.log()
	writeProjectTestFile(t, project, "main.cpp", "#include <github.com/demo/unrecorded/new.h>\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "--locked"); err == nil || !strings.Contains(err.Error(), "--locked") {
		t.Fatalf("cached analysis bypassed --locked: %v", err)
	}
	writeProjectTestFile(t, project, "main.cpp", "#include <recipe/library.hard.h>\nint main() { return library_value(); }\n")
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--locked", "-v", "--no-color"); err != nil || !strings.Contains(out, "Parsing main.cpp\n") || !strings.Contains(out, "Parsing "+vendor.Source+"/library.cpp (CACHED)") {
		t.Fatalf("source-only invalidation: %v\n%s\n%s", err, out, diagnostics)
	}
	run(true, "--locked")
	// Corrupt a file which is not an analysis input. Snapshot validation must
	// still fail instead of returning a valid parse-cache hit.
	snapshot := filepath.Join(configuration.root, "snapshot", filepath.FromSlash(recipe.Source), "@"+recipe.Commit)
	writeProjectTestFile(t, snapshot, "hard.yaml", "invalid manifest\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "--locked"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("cached analysis bypassed snapshot validation: %v", err)
	}
	after, _ = os.ReadFile(filename)
	if !bytes.Equal(before, after) || requests != proxy.log() {
		t.Fatal("failed fetch changed pins or requested an unrecorded dependency")
	}
}

func TestFetchCacheRevalidatesInheritedEdges(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	leaf, archive := inheritedTestSnapshot(t, "github.com/demo/leaf", "release", secondCommit, map[string]string{"leaf.h": "#pragma once\n"})
	proxy.add(leaf, archive)
	other := leaf
	other.Commit = nextCommit
	pins := map[string]repositoryPin{leaf.Source: leaf}
	for index, required := range []repositoryPin{leaf, other} {
		name := "github.com/demo/" + []string{"first", "second"}[index]
		parent, archive := inheritedTestSnapshot(t, name, "main", firstCommit, map[string]string{
			"parent.h":  "#pragma once\n#include <" + leaf.Source + "/leaf.h>\n",
			"hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{leaf.Source: required}),
		})
		proxy.add(parent, archive)
		pins[name] = parent
	}
	project := t.TempDir()
	withWorkingDirectory(t, project)
	configuration := projectTestConfiguration(t)
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, pins))
	writeProjectTestFile(t, project, "main.cpp", "#include <github.com/demo/first/parent.h>\n#include <github.com/demo/second/parent.h>\n")
	for pass := 0; pass < 2; pass++ {
		out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--locked", "-v", "--no-color")
		if err != nil || pass == 1 && !strings.Contains(out, "Parsing main.cpp (CACHED)") {
			t.Fatalf("project override pass %d: %v\n%s\n%s", pass, err, out, diagnostics)
		}
	}
	before, _ := os.ReadFile(filename)
	requests := proxy.log()
	parsed := arguments{command: "fetch"}
	file, session, err := prepareProject(&parsed, projectOptions{}, configuration.root, project)
	if err != nil {
		t.Fatal(err)
	}
	defer session.close()
	// Model an in-progress resolution with the same selected pins but no root
	// override for leaf: a parse hit must not conceal the two parent requirements.
	delete(file.Repositories, leaf.Source)
	view, err := session.view(configuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := newGitHubSnapshotResolver(view, nil)
	resolver.session = session
	cache, err := newArtifactCache(true, resolver)
	if err != nil {
		t.Fatal(err)
	}
	cflags := effectiveCFlags(configuration.cflags, view, configuration.runtimeRoot)
	recordPath, err := fetchParseCachePath(view, configuration.env, "main.cpp", session.layout)
	if err != nil {
		t.Fatal(err)
	}
	_, hit, err := cache.parseHit(recordPath, "fetch-parse", "main.cpp", parseCacheArguments(cflags, nil), project, compilerCacheWorkingDirectory(cflags, project))
	if err != nil || !hit {
		t.Fatalf("test requires a valid cache hit: %v", err)
	}
	progress := newProgressBar(io.Discard, -1, false, true, true)
	err = fetchSourcesWithCache(view, configuration.env, cflags, []string{"main.cpp"}, 1, progress, io.Discard, false, resolver)
	if err == nil || !strings.Contains(err.Error(), "repository pin conflict for "+leaf.Source) {
		t.Fatalf("cache hit did not replay inherited requirements: %v", err)
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(before, after) || requests != proxy.log() {
		t.Fatal("conflict changed pins or requested an overridden revision")
	}
}

func TestFetchParseCachePath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source.cpp")
	path, err := fetchParseCachePath(root, "host", source)
	if err != nil || !pathWithin(filepath.Join(root, "fetch", "host"), path) || !strings.HasSuffix(path, "source.cpp"+parseCacheSuffix) {
		t.Fatalf("fetch cache path = %q, %v", path, err)
	}
	if _, err := fetchParseCachePath(root, "../escape", source); err == nil {
		t.Fatal("accepted an escaping HARD_ENV")
	}
}
