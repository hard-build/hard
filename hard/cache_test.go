package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestActionFingerprintIsStableAndContentBased(t *testing.T) {
	directory := t.TempDir()
	tool := filepath.Join(directory, "c++")
	input := filepath.Join(directory, "input.cpp")
	writeBuildFile(t, directory, "c++", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatalf("chmod tool: %v", err)
	}
	writeBuildFile(t, directory, "input.cpp", "AAAA")

	cache := newTestArtifactCache(t, true)
	first, err := cache.actionFingerprint("compile", tool, []string{"-c"}, []string{input}, directory)
	if err != nil {
		t.Fatalf("first actionFingerprint() error = %v", err)
	}
	second, err := cache.actionFingerprint("compile", tool, []string{"-c"}, []string{input}, directory)
	if err != nil {
		t.Fatalf("second actionFingerprint() error = %v", err)
	}
	if first != second {
		t.Fatalf("repeated actionFingerprint() = %q, want %q", second, first)
	}

	writeBuildFile(t, directory, "input.cpp", "BBBB")
	changedInput, err := newTestArtifactCache(t, true).actionFingerprint(
		"compile",
		tool,
		[]string{"-c"},
		[]string{input},
		directory,
	)
	if err != nil {
		t.Fatalf("changed-input actionFingerprint() error = %v", err)
	}
	if changedInput == first {
		t.Fatal("actionFingerprint() did not change with same-size input content")
	}

	writeBuildFile(t, directory, "c++", "#!/bin/sh\nexit 1\n")
	if err := os.Chmod(tool, 0o755); err != nil {
		t.Fatalf("chmod changed tool: %v", err)
	}
	changedTool, err := newTestArtifactCache(t, true).actionFingerprint(
		"compile",
		tool,
		[]string{"-c"},
		[]string{input},
		directory,
	)
	if err != nil {
		t.Fatalf("changed-tool actionFingerprint() error = %v", err)
	}
	if changedTool == changedInput {
		t.Fatal("actionFingerprint() did not change with compiler content")
	}
}

func TestCompilerCacheWorkingDirectoryOnlyForRelativeArguments(t *testing.T) {
	workingDirectory := filepath.Join(string(filepath.Separator), "project")
	absoluteInclude := filepath.Join(string(filepath.Separator), "sdk", "include")
	tests := []struct {
		name      string
		arguments []string
		want      string
	}{
		{name: "ordinary flags", arguments: []string{"-std=c++20", "-O3", "-Wall"}},
		{
			name:      "absolute paths",
			arguments: []string{"-I" + absoluteInclude, "-include", filepath.Join(absoluteInclude, "config.h")},
		},
		{name: "relative joined path", arguments: []string{"-Iinclude"}, want: workingDirectory},
		{name: "relative separate path", arguments: []string{"-include", "config.h"}, want: workingDirectory},
		{name: "response file", arguments: []string{"@options"}, want: workingDirectory},
		{name: "opaque linker option", arguments: []string{"-Wl,--as-needed"}, want: workingDirectory},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compilerCacheWorkingDirectory(tt.arguments, workingDirectory)
			if got != tt.want {
				t.Fatalf("compilerCacheWorkingDirectory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArtifactCacheVerifiesArtifactContent(t *testing.T) {
	directory := t.TempDir()
	artifact := filepath.Join(directory, "artifact")
	writeBuildFile(t, directory, "artifact", "first")
	cache := newTestArtifactCache(t, true)
	if err := cache.store(artifact, buildCacheSuffix, "input"); err != nil {
		t.Fatalf("store() error = %v", err)
	}
	cached, err := cache.hit(artifact, buildCacheSuffix, "input")
	if err != nil {
		t.Fatalf("hit() error = %v", err)
	}
	if !cached {
		t.Fatal("hit() = false after store")
	}

	writeBuildFile(t, directory, "artifact", "other")
	cached, err = cache.hit(artifact, buildCacheSuffix, "input")
	if err != nil {
		t.Fatalf("hit() after artifact change error = %v", err)
	}
	if cached {
		t.Fatal("hit() = true after artifact content changed")
	}

	if err := os.WriteFile(artifact+buildCacheSuffix, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("corrupt cache record: %v", err)
	}
	cached, err = cache.hit(artifact, buildCacheSuffix, "input")
	if err != nil {
		t.Fatalf("hit() with corrupt record error = %v", err)
	}
	if cached {
		t.Fatal("hit() = true with corrupt record")
	}
}

func TestCompileSourceBatchCachesAndNoCacheRebuilds(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "source.cpp", "int value = 1;\n")
	compiler := filepath.Join(t.TempDir(), "c++")
	log := filepath.Join(t.TempDir(), "compile.log")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf 'compile\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'object\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	withWorkingDirectory(t, project)
	forward, err := sourceForwardHeaderPath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath() error = %v", err)
	}
	if err := writeForwardHeader(forward, []byte("#pragma once\n")); err != nil {
		t.Fatalf("write source forward header: %v", err)
	}

	run := func(read bool) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, 1, false, false, true)
		results, err := compileSourceBatch(
			root,
			"host",
			compiler,
			nil,
			[]string{"source.cpp"},
			[][]string{nil},
			1,
			false,
			false,
			project,
			progress,
			newTestArtifactCache(t, read),
		)
		if err != nil {
			t.Fatalf("compileSourceBatch() error = %v", err)
		}
		if err := compileResultsError(results); err != nil {
			t.Fatalf("compile result error = %v", err)
		}
		if err := progress.finish(); err != nil {
			t.Fatalf("progress.finish() error = %v", err)
		}
		return stdout.String()
	}

	if output := run(true); strings.Contains(output, "(CACHED)") {
		t.Fatalf("first compile output = %q, want cache miss", output)
	}
	if got := cacheLogLineCount(t, log); got != 1 {
		t.Fatalf("compiler runs after first compile = %d, want 1", got)
	}
	if output := run(true); !strings.Contains(output, "Compiling source.cpp (CACHED)") {
		t.Fatalf("second compile output = %q, want (CACHED)", output)
	}
	if got := cacheLogLineCount(t, log); got != 1 {
		t.Fatalf("compiler runs after cached compile = %d, want 1", got)
	}
	if output := run(false); strings.Contains(output, "(CACHED)") {
		t.Fatalf("no-cache compile output = %q, want cache miss", output)
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("compiler runs after no-cache compile = %d, want 2", got)
	}

	writeBuildFile(t, project, "source.cpp", "int value = 2;\n")
	if output := run(true); strings.Contains(output, "(CACHED)") {
		t.Fatalf("changed-source compile output = %q, want cache miss", output)
	}
	if got := cacheLogLineCount(t, log); got != 3 {
		t.Fatalf("compiler runs after source change = %d, want 3", got)
	}
}

func TestCompileCacheSeparatesRelativeFlagWorkingDirectories(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	writeBuildFile(t, project, "source.cpp", "")
	source := filepath.Join(project, "source.cpp")
	forward := filepath.Join(parent, "source.cpp.fwd.h")
	writeBuildFile(t, parent, "source.cpp.fwd.h", "#pragma once\n")
	object := filepath.Join(parent, "source.cpp.o")
	compiler := filepath.Join(parent, "c++")
	log := filepath.Join(parent, "compile.log")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf 'compile\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'object\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}

	run := func(workingDirectory string) bool {
		t.Helper()
		fatal, cached, err := compileSourceWithCache(
			newTestArtifactCache(t, true),
			compiler,
			[]string{"-I."},
			[]string{forward},
			nil,
			source,
			object,
			workingDirectory,
			io.Discard,
		)
		if err != nil || fatal {
			t.Fatalf("compileSourceWithCache() = fatal %v, error %v", fatal, err)
		}
		return cached
	}

	if run(parent) {
		t.Fatal("first parent compile was cached")
	}
	if run(project) {
		t.Fatal("first project compile reused the parent working directory")
	}
	if !run(project) {
		t.Fatal("second project compile was not cached")
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("compiler runs = %d, want 2", got)
	}
}

func TestLinkBinaryWithCacheUsesObjectContent(t *testing.T) {
	directory := t.TempDir()
	object := filepath.Join(directory, "source.o")
	artifact := filepath.Join(directory, "application")
	compiler := filepath.Join(directory, "c++")
	log := filepath.Join(directory, "link.log")
	t.Setenv("CACHE_LOG", log)
	writeBuildFile(t, directory, "source.o", "object one\n")
	script := "#!/bin/sh\n" +
		"printf 'link\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'binary\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake linker: %v", err)
	}

	run := func(read bool) bool {
		t.Helper()
		fatal, cached, err := linkBinaryWithCache(
			newTestArtifactCache(t, read),
			compiler,
			nil,
			[]string{object},
			"source.cpp",
			artifact,
			directory,
			io.Discard,
		)
		if err != nil || fatal {
			t.Fatalf("linkBinaryWithCache() = fatal %v, error %v", fatal, err)
		}
		return cached
	}

	if run(true) {
		t.Fatal("first link was cached")
	}
	if !run(true) {
		t.Fatal("second link was not cached")
	}
	writeBuildFile(t, directory, "source.o", "object two\n")
	if run(true) {
		t.Fatal("link after object change was cached")
	}
	if run(false) {
		t.Fatal("link with cache reads disabled was cached")
	}
	if got := cacheLogLineCount(t, log); got != 3 {
		t.Fatalf("linker runs = %d, want 3", got)
	}
}

func TestBuildSourcesWithProgressCachesCompileLinkAndCopy(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "application.h", "#pragma once\nstruct application_options {};\n")
	writeBuildFile(t, project, "application.cpp", "#include \"application.h\"\nint main() { return 0; }\n")
	compiler := filepath.Join(t.TempDir(), "c++")
	log := filepath.Join(t.TempDir(), "build.log")
	output := filepath.Join(project, "application")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf 'tool\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'artifact\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake build compiler: %v", err)
	}
	withWorkingDirectory(t, project)

	run := func(noCache bool) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, -1, true, false, true)
		err := buildSourcesWithProgress(
			root,
			t.TempDir(),
			"host",
			compiler,
			nil,
			nil,
			[]string{"main"},
			[]string{"application.cpp"},
			output,
			1,
			true,
			false,
			progress,
			io.Discard,
			noCache,
		)
		if err != nil {
			t.Fatalf("buildSourcesWithProgress() error = %v", err)
		}
		return stdout.String()
	}

	if first := run(false); strings.Contains(first, "(CACHED)") {
		t.Fatalf("first build output = %q, want cache miss", first)
	}
	second := run(false)
	for _, want := range []string{
		"Parsing application.cpp (CACHED)",
		"Compiling application.cpp (CACHED)",
		"Linking " + output + " (CACHED)",
		"Copying " + output + " (CACHED)",
	} {
		if !strings.Contains(second, want) {
			t.Fatalf("second build output = %q, want %q", second, want)
		}
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("compiler/linker runs after cached build = %d, want 2", got)
	}
	if forced := run(true); strings.Contains(forced, "(CACHED)") {
		t.Fatalf("forced build output = %q, want no cache hits", forced)
	}
	if got := cacheLogLineCount(t, log); got != 4 {
		t.Fatalf("compiler/linker runs after forced build = %d, want 4", got)
	}
}

func TestBuildCacheReusesSourceAcrossWorkingDirectories(t *testing.T) {
	root := t.TempDir()
	runtimeRoot := t.TempDir()
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	writeBuildFile(t, project, "application.cpp", "int main() { return 0; }\n")
	compiler := filepath.Join(t.TempDir(), "c++")
	log := filepath.Join(t.TempDir(), "build.log")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf 'tool\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'artifact\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake build compiler: %v", err)
	}
	withWorkingDirectory(t, parent)

	run := func(source string) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, -1, true, false, true)
		err := buildSourcesWithProgress(
			root,
			runtimeRoot,
			"host",
			compiler,
			nil,
			nil,
			[]string{"main"},
			[]string{source},
			"",
			1,
			true,
			false,
			progress,
			io.Discard,
			false,
		)
		if err != nil {
			t.Fatalf("buildSourcesWithProgress() error = %v", err)
		}
		return stdout.String()
	}

	if first := run(filepath.Join("project", "application.cpp")); strings.Contains(first, "(CACHED)") {
		t.Fatalf("first parent build output = %q, want cache miss", first)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatalf("change to project directory: %v", err)
	}
	second := run("application.cpp")
	for _, want := range []string{
		"Parsing application.cpp (CACHED)",
		"Compiling application.cpp (CACHED)",
		"Linking application (CACHED)",
		"Copying application (CACHED)",
	} {
		if !strings.Contains(second, want) {
			t.Fatalf("project build output = %q, want %q", second, want)
		}
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("compiler/linker runs after cross-directory hit = %d, want 2", got)
	}
}

func TestBuildCacheTreatsSystemHeadersAsHardEnvironment(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	system := t.TempDir()
	writeBuildFile(t, system, "toolchain.h", "#define TOOLCHAIN_VALUE 1\n")
	writeBuildFile(
		t,
		project,
		"source.cpp",
		"#include <toolchain.h>\nint value = TOOLCHAIN_VALUE;\n",
	)
	compiler := filepath.Join(t.TempDir(), "c++")
	log := filepath.Join(t.TempDir(), "compile.log")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf 'compile\\n' >> \"$CACHE_LOG\"\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"printf 'object\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	withWorkingDirectory(t, project)

	run := func(noCache bool) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, -1, true, false, true)
		err := buildSourcesWithProgress(
			root,
			t.TempDir(),
			"host",
			compiler,
			[]string{"-isystem", system},
			nil,
			nil,
			[]string{"source.cpp"},
			"",
			1,
			true,
			false,
			progress,
			io.Discard,
			noCache,
		)
		if err != nil {
			t.Fatalf("buildSourcesWithProgress() error = %v", err)
		}
		return stdout.String()
	}

	if first := run(false); strings.Contains(first, "(CACHED)") {
		t.Fatalf("first build output = %q, want cache miss", first)
	}
	recordPath, err := parseCachePath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("parseCachePath() error = %v", err)
	}
	record, ok, err := readParseCacheRecord(recordPath)
	if err != nil || !ok {
		t.Fatalf("readParseCacheRecord() = ok %v, error %v", ok, err)
	}
	if len(record.Dependencies) != 0 {
		t.Fatalf("system-only parse dependencies = %#v, want empty", record.Dependencies)
	}

	writeBuildFile(t, system, "toolchain.h", "#define TOOLCHAIN_VALUE 2\n")
	second := run(false)
	for _, want := range []string{
		"Parsing source.cpp (CACHED)",
		"Compiling source.cpp (CACHED)",
	} {
		if !strings.Contains(second, want) {
			t.Fatalf("second build output = %q, want %q", second, want)
		}
	}
	if got := cacheLogLineCount(t, log); got != 1 {
		t.Fatalf("compiler runs after system-header change = %d, want 1", got)
	}
	if forced := run(true); strings.Contains(forced, "(CACHED)") {
		t.Fatalf("forced build output = %q, want no cache hits", forced)
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("compiler runs after forced build = %d, want 2", got)
	}
}

func TestRunTestsCachesOnlySuccessfulResults(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "pass_test")
	log := filepath.Join(directory, "test.log")
	t.Setenv("CACHE_LOG", log)
	writeTestBinary := func(exit int) {
		t.Helper()
		script := "#!/bin/sh\n" +
			"printf 'run\\n' >> \"$CACHE_LOG\"\n" +
			"exit " + strconv.Itoa(exit) + "\n"
		if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake test binary: %v", err)
		}
	}
	writeTestBinary(0)
	task := testRunJob{source: "pass_test.cpp", binary: binary}

	run := func(read bool) (string, error) {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, 1, false, false, true)
		results := runTests(
			[]testRunJob{task},
			1,
			false,
			false,
			true,
			directory,
			progress,
			newTestArtifactCache(t, read),
		)
		if err := progress.finish(); err != nil {
			t.Fatalf("progress.finish() error = %v", err)
		}
		if len(results) != 1 || results[0] == nil {
			t.Fatalf("runTests() results = %#v", results)
		}
		return stdout.String(), results[0].err
	}

	if output, err := run(true); err != nil || strings.Contains(output, "(CACHED)") {
		t.Fatalf("first run output = %q, error = %v", output, err)
	}
	if output, err := run(true); err != nil || !strings.Contains(output, "Testing pass_test (CACHED)") {
		t.Fatalf("cached run output = %q, error = %v", output, err)
	}
	if got := cacheLogLineCount(t, log); got != 1 {
		t.Fatalf("test runs after cache hit = %d, want 1", got)
	}
	if output, err := run(false); err != nil || strings.Contains(output, "(CACHED)") {
		t.Fatalf("no-cache run output = %q, error = %v", output, err)
	}
	if got := cacheLogLineCount(t, log); got != 2 {
		t.Fatalf("test runs after no-cache = %d, want 2", got)
	}

	writeTestBinary(7)
	for attempt := 0; attempt < 2; attempt++ {
		output, err := run(true)
		if err == nil {
			t.Fatalf("failed test attempt %d error = nil", attempt+1)
		}
		if strings.Contains(output, "(CACHED)") {
			t.Fatalf("failed test attempt %d output = %q, want cache miss", attempt+1, output)
		}
	}
	if got := cacheLogLineCount(t, log); got != 4 {
		t.Fatalf("test runs after two failures = %d, want 4", got)
	}
	if _, err := os.Stat(binary + testResultCacheSuffix); !os.IsNotExist(err) {
		t.Fatalf("failed test cache record error = %v, want not exist", err)
	}
}

func TestRunTestsCacheSeparatesSelectorsAndNeverCachesListing(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "pass_test")
	log := filepath.Join(directory, "test.log")
	t.Setenv("CACHE_LOG", log)
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$CACHE_LOG\"\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake test binary: %v", err)
	}
	task := testRunJob{source: "pass_test.cpp", binary: binary}

	run := func(arguments []string, cache *artifactCache, action string) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, 1, false, false, true)
		results := runTestsWithArguments(
			[]testRunJob{task},
			1,
			false,
			false,
			directory,
			progress,
			cache,
			action,
			arguments,
		)
		if err := progress.finish(); err != nil {
			t.Fatalf("progress.finish() error = %v", err)
		}
		if len(results) != 1 || results[0] == nil || results[0].err != nil {
			t.Fatalf("runTestsWithArguments() results = %#v", results)
		}
		return stdout.String()
	}

	firstSelector := testBinaryArguments(true, []string{"Suite.First"})
	if output := run(firstSelector, newTestArtifactCache(t, true), "Testing"); strings.Contains(output, "(CACHED)") {
		t.Fatalf("first selector output = %q, want cache miss", output)
	}
	if output := run(firstSelector, newTestArtifactCache(t, true), "Testing"); !strings.Contains(output, "(CACHED)") {
		t.Fatalf("repeated selector output = %q, want cache hit", output)
	}
	secondSelector := testBinaryArguments(true, []string{"Suite.Second"})
	if output := run(secondSelector, newTestArtifactCache(t, true), "Testing"); strings.Contains(output, "(CACHED)") {
		t.Fatalf("different selector output = %q, want cache miss", output)
	}
	listArguments := testListBinaryArguments()
	for attempt := 0; attempt < 2; attempt++ {
		if output := run(listArguments, nil, "Listing"); strings.Contains(output, "(CACHED)") {
			t.Fatalf("listing attempt %d output = %q, want cache miss", attempt+1, output)
		}
	}

	got := readTestFile(t, log)
	want := strings.Join([]string{
		"--gtest_filter=Suite.First --gtest_color=no",
		"--gtest_filter=Suite.Second --gtest_color=no",
		"--gtest_list_tests --gtest_color=no",
		"--gtest_list_tests --gtest_color=no",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("test invocation log = %q, want %q", got, want)
	}
}

func TestSourceParseCacheUsesDependencySnapshotAndNoCacheRefreshes(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "dependency.h", "#pragma once\nstruct dependency {};\n")
	writeBuildFile(
		t,
		project,
		"source.cpp",
		"#include \"dependency.h\"\nint main() { return 0; }\n",
	)
	withWorkingDirectory(t, project)

	result, cached := inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("first parse cache activity = %#v, want miss", cached)
	}
	if result.entrypoint != "main" {
		t.Fatalf("first parse entry point = %q, want main", result.entrypoint)
	}
	recordPath, err := parseCachePath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("parseCachePath() error = %v", err)
	}
	record, ok, err := readParseCacheRecord(recordPath)
	if err != nil || !ok {
		t.Fatalf("readParseCacheRecord() = ok %v, error %v", ok, err)
	}
	dependency, err := realAbsolutePath("dependency.h", project)
	if err != nil {
		t.Fatalf("resolve dependency: %v", err)
	}
	if !reflect.DeepEqual(record.Dependencies, []string{dependency}) {
		t.Fatalf("parse cache dependencies = %#v, want %#v", record.Dependencies, []string{dependency})
	}
	if !reflect.DeepEqual(record.ManagedDependencies, []string{dependency}) {
		t.Fatalf(
			"parse cache managed dependencies = %#v, want %#v",
			record.ManagedDependencies,
			[]string{dependency},
		)
	}
	wantForward := "#pragma once\n\nstruct dependency;\n"
	if record.Forward != wantForward {
		t.Fatalf("parse cache forward = %q, want %q", record.Forward, wantForward)
	}

	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || !cached[0] {
		t.Fatalf("second parse cache activity = %#v, want hit", cached)
	}

	writeBuildFile(
		t,
		project,
		"source.cpp",
		"#include \"dependency.h\"\nint main() { return 0; }\n// changed\n",
	)
	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("source change parse cache activity = %#v, want miss", cached)
	}

	writeBuildFile(t, project, "dependency.h", "#pragma once\nstruct changed_dependency {};\n")
	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("dependency change parse cache activity = %#v, want miss", cached)
	}

	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		false,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("no-cache parse activity = %#v, want miss", cached)
	}
	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || !cached[0] {
		t.Fatalf("parse after no-cache refresh activity = %#v, want hit", cached)
	}

	if err := os.WriteFile(recordPath, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("corrupt parse cache record: %v", err)
	}
	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		nil,
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("malformed parse cache activity = %#v, want miss", cached)
	}
	if _, ok, err := readParseCacheRecord(recordPath); err != nil || !ok {
		t.Fatalf("refreshed parse cache record = ok %v, error %v", ok, err)
	}

	_, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		[]string{"-DVALUE=1"},
		[]string{"main"},
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("changed cflags parse cache activity = %#v, want miss", cached)
	}

	result, cached = inspectCachedBuildSource(
		t,
		root,
		project,
		"source.cpp",
		[]string{"-DVALUE=1"},
		nil,
		true,
	)
	if len(cached) != 1 || cached[0] {
		t.Fatalf("changed entry points parse cache activity = %#v, want miss", cached)
	}
	if result.entrypoint != "" {
		t.Fatalf("empty entry point configuration result = %q, want empty", result.entrypoint)
	}
}

func TestParseCacheSkipsHasIncludeAndFailedParses(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		root := t.TempDir()
		project := t.TempDir()
		writeBuildFile(
			t,
			project,
			"source.cpp",
			"#if __has_include(\"optional.h\")\n#include \"optional.h\"\n#endif\n",
		)
		withWorkingDirectory(t, project)
		inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		recordPath, err := parseCachePath(root, "host", "source.cpp")
		if err != nil {
			t.Fatalf("parseCachePath() error = %v", err)
		}
		if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
			t.Fatalf("__has_include source parse cache error = %v, want not exist", err)
		}
	})

	t.Run("transitive dependency", func(t *testing.T) {
		root := t.TempDir()
		project := t.TempDir()
		writeBuildFile(t, project, "source.cpp", "#include \"dependency.h\"\n")
		writeBuildFile(
			t,
			project,
			"dependency.h",
			"#if __has_include(\"optional.h\")\n#include \"optional.h\"\n#endif\n",
		)
		withWorkingDirectory(t, project)
		_, cached := inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		if len(cached) != 1 || cached[0] {
			t.Fatalf("first transitive __has_include activity = %#v, want miss", cached)
		}
		recordPath, err := parseCachePath(root, "host", "source.cpp")
		if err != nil {
			t.Fatalf("parseCachePath() error = %v", err)
		}
		if _, ok, err := readParseCacheRecord(recordPath); err != nil || !ok {
			t.Fatalf("transitive __has_include parse cache record = ok %v, error %v", ok, err)
		}
		_, cached = inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		if len(cached) != 1 || !cached[0] {
			t.Fatalf("second transitive __has_include activity = %#v, want hit", cached)
		}
	})

	t.Run("standard library dependency", func(t *testing.T) {
		root := t.TempDir()
		project := t.TempDir()
		writeBuildFile(t, project, "source.cpp", "#include <iostream>\n")
		withWorkingDirectory(t, project)

		result, cached := inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		if result.err != nil {
			t.Fatalf("first standard-library parse error = %v", result.err)
		}
		if len(cached) != 1 || cached[0] {
			t.Fatalf("first standard-library parse activity = %#v, want miss", cached)
		}
		recordPath, err := parseCachePath(root, "host", "source.cpp")
		if err != nil {
			t.Fatalf("parseCachePath() error = %v", err)
		}
		record, ok, err := readParseCacheRecord(recordPath)
		if err != nil || !ok {
			t.Fatalf("standard-library parse cache record = ok %v, error %v", ok, err)
		}
		if len(record.Dependencies) != 0 {
			t.Fatalf("standard-library parse dependencies = %#v, want empty", record.Dependencies)
		}
		_, cached = inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		if len(cached) != 1 || !cached[0] {
			t.Fatalf("second standard-library parse activity = %#v, want hit", cached)
		}
	})

	t.Run("cflags", func(t *testing.T) {
		project := t.TempDir()
		writeBuildFile(t, project, "source.cpp", "")
		unsafe, err := parseCacheInputContainsHasInclude(
			"source.cpp",
			[]string{"-DPROBE=__has_include"},
			project,
		)
		if err != nil {
			t.Fatalf("parseCacheInputContainsHasInclude() error = %v", err)
		}
		if !unsafe {
			t.Fatal("parseCacheInputContainsHasInclude() = false, want true")
		}
	})

	t.Run("failed parse", func(t *testing.T) {
		root := t.TempDir()
		project := t.TempDir()
		writeBuildFile(t, project, "source.cpp", "#include \"missing.h\"\n")
		withWorkingDirectory(t, project)
		result, _ := inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, true)
		if result.err == nil {
			t.Fatal("failed source parse error = nil")
		}
		recordPath, err := parseCachePath(root, "host", "source.cpp")
		if err != nil {
			t.Fatalf("parseCachePath() error = %v", err)
		}
		if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
			t.Fatalf("failed source parse cache error = %v, want not exist", err)
		}
	})
}

func TestSourceParseCacheRestoresForwardAndTracksDependencies(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	dependency := filepath.Join(project, "dependency.h")
	public := filepath.Join(project, "public.h")
	writeBuildFile(t, project, "dependency.h", "#pragma once\nstruct dependency {};\n")
	writeBuildFile(
		t,
		project,
		"public.h",
		"#include \"dependency.h\"\nstruct public_type {};\n",
	)
	writeBuildFile(t, project, "source.cpp", "#include \"public.h\"\n")
	withWorkingDirectory(t, project)
	output, err := sourceForwardHeaderPath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath() error = %v", err)
	}

	run := func(read bool) []bool {
		t.Helper()
		_, cached := inspectCachedBuildSource(t, root, project, "source.cpp", nil, nil, read)
		return cached
	}

	if cached := run(true); len(cached) != 1 || cached[0] {
		t.Fatalf("first source parse cache activity = %#v, want miss", cached)
	}
	want := "#pragma once\n\nstruct dependency;\nstruct public_type;\n"
	if got := readTestFile(t, output); got != want {
		t.Fatalf("forward output = %q, want %q", got, want)
	}
	recordPath, err := parseCachePath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("parseCachePath() error = %v", err)
	}
	record, ok, err := readParseCacheRecord(recordPath)
	if err != nil || !ok {
		t.Fatalf("readParseCacheRecord() = ok %v, error %v", ok, err)
	}
	wantDependencies := []string{dependency, public}
	if !reflect.DeepEqual(record.Dependencies, wantDependencies) {
		t.Fatalf("source parse cache dependencies = %#v, want %#v", record.Dependencies, wantDependencies)
	}
	if err := os.Remove(output); err != nil {
		t.Fatalf("remove generated forward header: %v", err)
	}
	if cached := run(true); len(cached) != 1 || !cached[0] {
		t.Fatalf("second source parse cache activity = %#v, want hit", cached)
	}
	if got := readTestFile(t, output); got != want {
		t.Fatalf("restored forward output = %q, want %q", got, want)
	}

	record.Forward = "corrupted forward\n"
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("encode corrupted parse cache record: %v", err)
	}
	if err := writeCacheRecord(recordPath, append(encoded, '\n')); err != nil {
		t.Fatalf("write corrupted parse cache record: %v", err)
	}
	if cached := run(true); len(cached) != 1 || cached[0] {
		t.Fatalf("corrupted source result cache activity = %#v, want miss", cached)
	}
	if got := readTestFile(t, output); got != want {
		t.Fatalf("regenerated forward output = %q, want %q", got, want)
	}

	writeBuildFile(t, project, "dependency.h", "#pragma once\nstruct changed_dependency {};\n")
	if cached := run(true); len(cached) != 1 || cached[0] {
		t.Fatalf("changed dependency source cache activity = %#v, want miss", cached)
	}
	changed := readTestFile(t, output)
	if !strings.Contains(changed, "struct changed_dependency;") || strings.Contains(changed, "struct dependency;") {
		t.Fatalf("changed source forward output = %q", changed)
	}
	if cached := run(false); len(cached) != 1 || cached[0] {
		t.Fatalf("no-cache source activity = %#v, want miss", cached)
	}
	if cached := run(true); len(cached) != 1 || !cached[0] {
		t.Fatalf("source activity after refresh = %#v, want hit", cached)
	}
}

func TestTestSourcesReportsCachedParsing(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "pass_test.cpp", "")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	run := func(noCache bool) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, -1, true, false, true)
		err := testSourcesWithProgress(
			root,
			t.TempDir(),
			"host",
			compiler,
			nil,
			nil,
			[]string{"pass_test.cpp"},
			1,
			true,
			false,
			true,
			progress,
			&stdout,
			io.Discard,
			noCache,
		)
		if err != nil {
			t.Fatalf("testSourcesWithProgress() error = %v", err)
		}
		return stdout.String()
	}

	if first := run(false); strings.Contains(first, "(CACHED)") {
		t.Fatalf("first test output = %q, want cache miss", first)
	}
	second := run(false)
	for _, want := range []string{
		"Parsing pass_test.cpp (CACHED)",
		"Compiling pass_test.cpp (CACHED)",
		"Linking pass_test (CACHED)",
		"Testing pass_test (CACHED)",
	} {
		if !strings.Contains(second, want) {
			t.Fatalf("second test output = %q, want %q", second, want)
		}
	}
	if forced := run(true); strings.Contains(forced, "(CACHED)") {
		t.Fatalf("forced test output = %q, want no cache hits", forced)
	}
}

func inspectCachedBuildSource(
	t *testing.T,
	root string,
	workingDirectory string,
	source string,
	cflags []string,
	entryPoints []string,
	read bool,
) (*buildResult, []bool) {
	t.Helper()
	var cached []bool
	results := inspectBuildSourcesWithCache(
		root,
		"host",
		"",
		nil,
		cflags,
		entryPoints,
		[]string{source},
		1,
		workingDirectory,
		func(_ string, hit bool) {
			cached = append(cached, hit)
		},
		newTestArtifactCache(t, read),
	)
	if len(results) != 1 || results[0] == nil {
		t.Fatalf("inspectBuildSourcesWithCache() results = %#v", results)
	}
	return results[0], cached
}

func newTestArtifactCache(t *testing.T, read bool) *artifactCache {
	t.Helper()
	cache, err := newArtifactCache(read)
	if err != nil {
		t.Fatalf("newArtifactCache() error = %v", err)
	}
	return cache
}

func cacheLogLineCount(t *testing.T, path string) int {
	t.Helper()
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatalf("read cache test log: %v", err)
	}
	return len(strings.Fields(string(contents)))
}
