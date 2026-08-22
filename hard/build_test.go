package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestSourceDependenciesUsesCFlags(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "source.cpp", "")
	writeBuildFile(t, root, "include/forced.h", "")

	fatal, got, err := sourceDependencies(
		nil,
		[]string{"-std=c++20", "-include", "include/forced.h"},
		"source.cpp",
		root,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	if want := []string{filepath.Join(root, "include", "forced.h")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sourceDependencies() = %#v, want %#v", got, want)
	}
}

func TestObjectFilePathMirrorsAbsoluteSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "home", "user", "project", "src", "file.cpp")

	got, err := objectFilePath(root, "target", source)
	if err != nil {
		t.Fatalf("objectFilePath() error = %v", err)
	}
	want := filepath.Join(root, "env", "target", "build", "home", "user", "project", "src", "file.cpp.o")
	if got != want {
		t.Fatalf("objectFilePath() = %q, want %q", got, want)
	}
}

func TestObjectFilePathPreservesSourceExtension(t *testing.T) {
	root := t.TempDir()
	tests := []string{"file.c", "file.cc", "file.cpp", "file.c++"}
	for _, name := range tests {
		source := filepath.Join(string(filepath.Separator), "project", name)
		got, err := objectFilePath(root, "host", source)
		if err != nil {
			t.Fatalf("objectFilePath(%q) error = %v", name, err)
		}
		want := filepath.Join(root, "env", "host", "build", "project", name+".o")
		if got != want {
			t.Errorf("objectFilePath(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestObjectFilePathRejectsEnvironmentEscape(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "project", "file.cpp")

	_, err := objectFilePath(root, "../outside", source)
	if err == nil {
		t.Fatal("objectFilePath() error = nil")
	}
	if !strings.Contains(err.Error(), "HARD_ENV escapes environment directory") {
		t.Fatalf("objectFilePath() error = %q", err)
	}
}

func TestBinaryArtifactPathMirrorsAbsoluteSourceWithoutExtension(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "home", "user", "project", "src", "file.cpp")

	got, err := binaryArtifactPath(root, "target", source)
	if err != nil {
		t.Fatalf("binaryArtifactPath() error = %v", err)
	}
	want := filepath.Join(root, "env", "target", "build", "home", "user", "project", "src", "file")
	if got != want {
		t.Fatalf("binaryArtifactPath() = %q, want %q", got, want)
	}
}

func TestBinaryArtifactPathRejectsEnvironmentEscape(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "project", "file.cpp")

	_, err := binaryArtifactPath(root, "../outside", source)
	if err == nil {
		t.Fatal("binaryArtifactPath() error = nil")
	}
	if !strings.Contains(err.Error(), "HARD_ENV escapes environment directory") {
		t.Fatalf("binaryArtifactPath() error = %q", err)
	}
}

func TestCompileSourceUsesCompilerAndCFlags(t *testing.T) {
	project := t.TempDir()
	writeBuildFile(t, project, "src/source.cpp", "")
	compiler := filepath.Join(t.TempDir(), "custom-c++")
	log := filepath.Join(t.TempDir(), "compiler.log")
	object := filepath.Join(t.TempDir(), "objects", "src", "source.cpp.o")
	forwards := []string{
		filepath.Join(t.TempDir(), "forward one.hpp"),
		filepath.Join(t.TempDir(), "forward-two.h"),
	}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"$COMPILER_LOG\"\n" +
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
	t.Setenv("COMPILER_LOG", log)

	fatal, err := compileSource(
		compiler,
		[]string{"-std=c++20", "-DOPTION=1"},
		forwards,
		"src/source.cpp",
		object,
		project,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("compileSource() error = %v", err)
	}
	if fatal {
		t.Fatal("compileSource() fatal = true")
	}
	wantLog := strings.Join([]string{
		"-std=c++20",
		"-DOPTION=1",
		"-include",
		forwards[0],
		"-include",
		forwards[1],
		"-c",
		"src/source.cpp",
		"-o",
		object,
		"",
	}, "\n")
	if got := readTestFile(t, log); got != wantLog {
		t.Fatalf("compiler arguments = %q, want %q", got, wantLog)
	}
	if got := readTestFile(t, object); got != "object\n" {
		t.Fatalf("object contents = %q", got)
	}
}

func TestCompileSourceBatchIncludesSourceForwardHeader(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "source.cpp", "")
	external := filepath.Join(
		root,
		"source",
		"github.com",
		"owner",
		"repository",
		"header.hpp",
	)
	writeBuildFile(t, root, "source/github.com/owner/repository/header.hpp", "")
	compiler := filepath.Join(t.TempDir(), "c++")
	script := "#!/bin/sh\n" +
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

	var stdout bytes.Buffer
	results, err := compileSourceBatch(
		root,
		"host",
		compiler,
		nil,
		[]string{"source.cpp"},
		[][]string{{external}},
		1,
		true,
		false,
		project,
		newProgressBar(&stdout, 1, true, false, true),
		nil,
	)
	if err != nil {
		t.Fatalf("compileSourceBatch() error = %v", err)
	}
	if err := compileResultsError(results); err != nil {
		t.Fatalf("compileSourceBatch() result error = %v", err)
	}
	forward, err := sourceForwardHeaderPath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath() error = %v", err)
	}
	if want := "-include " + forward; !strings.Contains(stdout.String(), want) {
		t.Fatalf("compile command does not contain %q: %q", want, stdout.String())
	}
	object, err := objectFilePath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("objectFilePath() error = %v", err)
	}
	if got := readTestFile(t, object); got != "object\n" {
		t.Fatalf("object contents = %q", got)
	}
}

func TestCompileSourceDisplayPath(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "local.cpp", "")
	writeBuildFile(
		t,
		root,
		"source/github.com/hard-build/library/application/application.cpp",
		"",
	)
	if err := os.Symlink(
		filepath.FromSlash("github.com/hard-build/library"),
		filepath.Join(root, "source", "hard"),
	); err != nil {
		t.Fatalf("create well-known source alias: %v", err)
	}
	external := filepath.Join(
		root,
		"source",
		"github.com",
		"hard-build",
		"library",
		"application",
		"application.cpp",
	)
	alias := filepath.Join(root, "source", "hard", "application", "application.cpp")
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "local", source: "local.cpp", want: "local.cpp"},
		{
			name:   "canonical external",
			source: external,
			want:   "github.com/hard-build/library/application/application.cpp",
		},
		{
			name:   "well-known alias",
			source: alias,
			want:   "github.com/hard-build/library/application/application.cpp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compileSourceDisplayPath(root, tt.source, project)
			if got != tt.want {
				t.Fatalf("compileSourceDisplayPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompileSourceBatchDisplaysCanonicalGitHubPath(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	external := filepath.Join(
		root,
		"source",
		"github.com",
		"hard-build",
		"library",
		"application",
		"application.cpp",
	)
	writeBuildFile(
		t,
		root,
		"source/github.com/hard-build/library/application/application.cpp",
		"",
	)
	source, err := filepath.Rel(project, external)
	if err != nil {
		t.Fatalf("make external source relative: %v", err)
	}
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	results, err := compileSourceBatch(
		root,
		"host",
		installBuildCompiler(t),
		nil,
		[]string{source},
		[][]string{nil},
		1,
		true,
		false,
		project,
		newProgressBar(&stdout, 1, true, false, true),
		nil,
	)
	if err != nil {
		t.Fatalf("compileSourceBatch() error = %v", err)
	}
	if err := compileResultsError(results); err != nil {
		t.Fatalf("compileSourceBatch() result error = %v", err)
	}
	output := stdout.String()
	wantProgress := "[1/1] Compiling github.com/hard-build/library/application/application.cpp\n"
	if !strings.HasPrefix(output, wantProgress) {
		t.Fatalf("compile progress = %q, want prefix %q", output, wantProgress)
	}
	if wantCommandSource := "-c " + source + " "; !strings.Contains(output, wantCommandSource) {
		t.Fatalf("compile command does not contain %q: %q", wantCommandSource, output)
	}
}

func TestCompileSourceBatchReturnsPerSourceResults(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "good.cpp", "")
	writeBuildFile(t, project, "broken.cpp", "")
	compiler := filepath.Join(t.TempDir(), "c++")
	script := "#!/bin/sh\n" +
		"source=''\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tcase \"$argument\" in *.cpp) source=$argument ;; esac\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"if [ \"$source\" = broken.cpp ]; then\n" +
		"\tprintf 'broken diagnostic\\n' >&2\n" +
		"\texit 7\n" +
		"fi\n" +
		"printf 'object\\n' > \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	withWorkingDirectory(t, project)

	results, err := compileSourceBatch(
		root,
		"host",
		compiler,
		nil,
		[]string{"good.cpp", "broken.cpp"},
		[][]string{nil, nil},
		2,
		false,
		true,
		project,
		newProgressBar(io.Discard, 2, false, true, true),
		nil,
	)
	if err != nil {
		t.Fatalf("compileSourceBatch() error = %v", err)
	}
	if len(results) != 2 || results[0] == nil || results[1] == nil {
		t.Fatalf("compileSourceBatch() results = %#v", results)
	}
	if results[0].err != nil {
		t.Fatalf("good compile error = %v", results[0].err)
	}
	if results[1].err == nil || !strings.Contains(results[1].err.Error(), "compile broken.cpp") {
		t.Fatalf("broken compile error = %v", results[1].err)
	}
	if got := string(results[1].diagnostics); got != "broken diagnostic\n" {
		t.Fatalf("broken compile diagnostics = %q", got)
	}
	object, err := objectFilePath(root, "host", "good.cpp")
	if err != nil {
		t.Fatalf("objectFilePath() error = %v", err)
	}
	if got := readTestFile(t, object); got != "object\n" {
		t.Fatalf("good object = %q", got)
	}
}

func TestCompileArgumentsWithoutForwardHeaders(t *testing.T) {
	got := compileArguments([]string{"-O2"}, nil, "source.cpp", "source.cpp.o")
	want := []string{"-O2", "-c", "source.cpp", "-o", "source.cpp.o"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compileArguments() = %#v, want %#v", got, want)
	}
}

func TestRenderCompileCommandQuotesShellArguments(t *testing.T) {
	got := string(renderCompileCommand(
		"custom c++",
		[]string{"-std=c++20", "-DNAME=O'Reilly", "-DVALUE=two words", ""},
		[]string{"/tmp/forward header.hpp", "/tmp/O'Reilly_fwd.h"},
		"src/file name.cpp",
		"/tmp/output file.cpp.o",
	))
	want := "'custom c++' -std=c++20 '-DNAME=O'\"'\"'Reilly' '-DVALUE=two words' '' " +
		"-include '/tmp/forward header.hpp' -include '/tmp/O'\"'\"'Reilly_fwd.h' " +
		"-c 'src/file name.cpp' -o '/tmp/output file.cpp.o'\n"
	if got != want {
		t.Fatalf("renderCompileCommand() = %q, want %q", got, want)
	}
}

func TestBuildSourcesOutputModes(t *testing.T) {
	tests := []struct {
		name      string
		verbose   bool
		silent    bool
		want      []string
		forbidden []string
	}{
		{
			name:      "normal",
			want:      []string{"\r[1/?] Parsing ", "\r[2/5] Compiling ", "\r[3/5] Compiling ", "\r[4/5] Linking first", "\r[5/5] Copying first"},
			forbidden: []string{" -c "},
		},
		{
			name:    "verbose",
			verbose: true,
			want: []string{
				"[1/?] Parsing first.cpp",
				"[1/?] Parsing second.cpp",
				"[2/5] Compiling ",
				"[3/5] Compiling ",
				"[4/5] Linking first",
				"[5/5] Copying first",
				"Compiling first.cpp\n",
				"Compiling second.cpp\n",
			},
			forbidden: []string{"\r"},
		},
		{
			name:      "silent",
			verbose:   true,
			silent:    true,
			forbidden: []string{"[", "first.cpp", "second.cpp", "include/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeBuildFile(
				t,
				root,
				"first.cpp",
				"#include \"include/common.h\"\n#include \"include/first.h\"\nint main() { return 0; }\n",
			)
			writeBuildFile(
				t,
				root,
				"second.cpp",
				"#include \"include/common.h\"\n#include \"include/second.h\"\n",
			)
			writeBuildFile(t, root, "include/common.h", "")
			writeBuildFile(t, root, "include/first.h", "")
			writeBuildFile(t, root, "include/second.h", "")
			compiler := installBuildCompiler(t)
			cflags := []string{"-DOPTION=1"}
			withWorkingDirectory(t, root)

			var stdout bytes.Buffer
			if err := buildSources(
				root,
				"host",
				compiler,
				cflags,
				nil,
				[]string{"main"},
				[]string{"first.cpp", "second.cpp"},
				"",
				2,
				tt.verbose,
				tt.silent,
				true,
				&stdout,
				io.Discard,
			); err != nil {
				t.Fatalf("buildSources() error = %v", err)
			}

			output := stdout.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("buildSources() output does not contain %q: %q", want, output)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(output, forbidden) {
					t.Errorf("buildSources() output contains %q: %q", forbidden, output)
				}
			}
			for _, header := range []string{"common.h", "first.h", "second.h"} {
				path := filepath.Join(root, "include", header) + "\n"
				if strings.Contains(output, path) {
					t.Errorf("buildSources() output contains header path %q: %q", path, output)
				}
			}
			if tt.name == "normal" && strings.Count(output, "\n") != 1 {
				t.Errorf("buildSources() normal output has multiple lines: %q", output)
			}
			for _, source := range []string{"first.cpp", "second.cpp"} {
				object, err := objectFilePath(root, "host", source)
				if err != nil {
					t.Fatalf("objectFilePath(%q) error = %v", source, err)
				}
				if got := readTestFile(t, object); got != "object\n" {
					t.Fatalf("generated object for %s = %q", source, got)
				}
				forward, err := sourceForwardHeaderPath(root, "host", source)
				if err != nil {
					t.Fatalf("sourceForwardHeaderPath(%q) error = %v", source, err)
				}
				if got := readTestFile(t, forward); got != "#pragma once\n" {
					t.Fatalf("generated forward header for %s = %q", source, got)
				}
				command := string(renderCompileCommand(compiler, cflags, []string{forward}, source, object))
				if tt.verbose && !tt.silent {
					if !strings.Contains(output, source+"\n"+command) {
						t.Errorf("buildSources() output does not contain command after %q: %q", source, output)
					}
				} else if strings.Contains(output, command) {
					t.Errorf("buildSources() output contains command %q: %q", command, output)
				}
			}
			if got := readTestFile(t, filepath.Join(root, "first")); got != "object\n" {
				t.Fatalf("delivered binary = %q", got)
			}
		})
	}
}

func TestBuildSourcesExcludesEnvironmentSupportFromSourceForward(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	supportDirectory := t.TempDir()
	supportHeader := filepath.Join(supportDirectory, "hard.h")
	writeBuildFile(t, supportDirectory, "hard.h", "#pragma once\nstruct hard_support {};\n")
	environmentHeader := filepath.Join(root, "env", "host", "hard.h")
	if err := os.MkdirAll(filepath.Dir(environmentHeader), 0o755); err != nil {
		t.Fatalf("create environment directory: %v", err)
	}
	if err := os.Symlink(supportHeader, environmentHeader); err != nil {
		t.Fatalf("create environment support-header symlink: %v", err)
	}
	writeBuildFile(t, project, "hard.h", "#pragma once\nstruct project_type {};\n")
	writeBuildFile(t, project, "source.cpp", "#include \"hard.h\"\n")
	compiler := installBuildCompiler(t)
	withWorkingDirectory(t, project)

	cflags := []string{"-include", environmentHeader}
	var stdout bytes.Buffer
	if err := buildSources(
		root,
		"host",
		compiler,
		cflags,
		nil,
		nil,
		[]string{"source.cpp"},
		"",
		1,
		true,
		false,
		true,
		&stdout,
		io.Discard,
	); err != nil {
		t.Fatalf("buildSources() error = %v", err)
	}

	sourceForward, err := sourceForwardHeaderPath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath(source.cpp) error = %v", err)
	}
	forwardContents := readTestFile(t, sourceForward)
	if !strings.Contains(forwardContents, "struct project_type;") {
		t.Fatalf("source forward header = %q, want project_type", forwardContents)
	}
	if strings.Contains(forwardContents, "hard_support") {
		t.Fatalf("source forward header = %q, want no hard_support", forwardContents)
	}

	object, err := objectFilePath(root, "host", "source.cpp")
	if err != nil {
		t.Fatalf("objectFilePath(source.cpp) error = %v", err)
	}
	output := stdout.String()
	wantCommand := string(renderCompileCommand(
		compiler,
		cflags,
		[]string{sourceForward},
		"source.cpp",
		object,
	))
	for _, want := range []string{"Compiling source.cpp\n" + wantCommand} {
		if !strings.Contains(output, want) {
			t.Errorf("buildSources() output does not contain %q: %q", want, output)
		}
	}
	for _, forbidden := range []string{
		"Parsing hard.h",
		"Parsing " + buildParsingDisplayPath(root, supportHeader, project),
	} {
		if strings.Contains(output, forbidden) {
			t.Errorf("buildSources() output contains %q: %q", forbidden, output)
		}
	}
}

func TestBuildSourcesContinuesSearchProgressThroughParsing(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "source.cpp", "int main() { return 0; }\n")
	withWorkingDirectory(t, root)

	var stdout bytes.Buffer
	progress := newProgressBar(&stdout, -1, true, false, true)
	progress.updateStep("Searching source files")
	if err := buildSourcesWithProgress(
		root,
		"host",
		installBuildCompiler(t),
		nil,
		nil,
		[]string{"main"},
		[]string{"source.cpp"},
		"",
		1,
		true,
		false,
		progress,
		io.Discard,
		false,
	); err != nil {
		t.Fatalf("buildSourcesWithProgress() error = %v", err)
	}

	output := stdout.String()
	wants := []string{
		"[1/?] Searching source files\n",
		"[1/?] Parsing source.cpp\n",
		"[2/4] Compiling source.cpp\n",
		"[3/4] Linking source\n",
		"[4/4] Copying source\n",
	}
	previous := -1
	for _, want := range wants {
		index := strings.Index(output, want)
		if index < 0 {
			t.Fatalf("build output does not contain %q: %q", want, output)
		}
		if index <= previous {
			t.Fatalf("build output order is wrong for %q: %q", want, output)
		}
		previous = index
	}
}

func TestBuildSourcesWritesCompilerErrorsInSilentMode(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "source.cpp", "")
	compiler := filepath.Join(t.TempDir(), "c++")
	script := "#!/bin/sh\n" +
		"printf 'compile error\\n' >&2\n" +
		"exit 7\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	withWorkingDirectory(t, root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := buildSources(
		root,
		"host",
		compiler,
		nil,
		nil,
		nil,
		[]string{"source.cpp"},
		"",
		1,
		true,
		true,
		false,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("buildSources() error = nil")
	}
	if stdout.Len() != 0 {
		t.Fatalf("buildSources() stdout = %q, want empty output", stdout.String())
	}
	if got := stderr.String(); got != "compile error\n" {
		t.Fatalf("buildSources() stderr = %q, want compile error", got)
	}
}

func TestBuildSourcesStopsWhenCompilerIsMissing(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "first.cpp", "")
	writeBuildFile(t, root, "second.cpp", "")
	withWorkingDirectory(t, root)

	var stdout bytes.Buffer
	err := buildSources(
		root,
		"host",
		"missing-c++",
		nil,
		nil,
		nil,
		[]string{"first.cpp", "second.cpp"},
		"",
		1,
		false,
		false,
		true,
		&stdout,
		io.Discard,
	)
	if err == nil {
		t.Fatal("buildSources() error = nil")
	}
	if !strings.Contains(err.Error(), "missing-c++") {
		t.Fatalf("buildSources() error = %q, want compiler name", err)
	}
	if !strings.Contains(stdout.String(), "Compiling first.cpp") {
		t.Fatalf("buildSources() stdout = %q, want first compilation progress", stdout.String())
	}
}

func TestSourceDependenciesUsesTransitiveAndForcedIncludesButNotSystemHeaders(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "source.cpp", "#include <vector>\n#include \"include/direct.h\"\n")
	writeBuildFile(t, root, "include/direct.h", "#include \"transitive.h\"\n")
	writeBuildFile(t, root, "include/transitive.h", "#pragma once\nstruct Transitive {};\n")
	writeBuildFile(t, root, "include/forced.h", "#pragma once\nstruct Forced {};\n")

	fatal, got, err := sourceDependencies(
		nil,
		[]string{"-std=c++20", "-include", "include/forced.h"},
		"source.cpp",
		root,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	sort.Strings(got)
	want := []string{
		filepath.Join(root, "include", "direct.h"),
		filepath.Join(root, "include", "forced.h"),
		filepath.Join(root, "include", "transitive.h"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sourceDependencies() = %#v, want %#v", got, want)
	}
}

func TestImplementationSourceForHeaderMatchesSupportedExtensionCase(t *testing.T) {
	project := t.TempDir()
	writeBuildFile(t, project, "library.hpp", "")
	writeBuildFile(t, project, "library.CPP", "")

	got, err := implementationSourceForHeader(filepath.Join(project, "library.hpp"), project)
	if err != nil {
		t.Fatalf("implementationSourceForHeader() error = %v", err)
	}
	if got != "library.CPP" {
		t.Fatalf("implementationSourceForHeader() = %q, want library.CPP", got)
	}
}

func TestImplementationSourceForHeaderRejectsAmbiguousImplementations(t *testing.T) {
	project := t.TempDir()
	writeBuildFile(t, project, "library.h", "")
	writeBuildFile(t, project, "library.c", "")
	writeBuildFile(t, project, "library.cpp", "")

	_, err := implementationSourceForHeader(filepath.Join(project, "library.h"), project)
	if err == nil {
		t.Fatal("implementationSourceForHeader() error = nil")
	}
	for _, want := range []string{"library.h", "library.c", "library.cpp"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("implementationSourceForHeader() error = %q, want %q", err, want)
		}
	}
}

func TestPlanLinkJobsUsesTransitiveImplementationObjects(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	for _, path := range []string{
		"app.cpp",
		"container/container.cpp",
		"container/container.h",
		"component/component.cpp",
		"component/component.h",
		"unused.cpp",
		"unused.h",
	} {
		writeBuildFile(t, project, path, "")
	}
	withWorkingDirectory(t, project)

	sources := []string{
		"app.cpp",
		"container/container.cpp",
		"component/component.cpp",
		"unused.cpp",
	}
	dependencies := [][]string{
		{filepath.Join(project, "container", "container.h")},
		{
			filepath.Join(project, "container", "container.h"),
			filepath.Join(project, "component", "component.h"),
		},
		{
			filepath.Join(project, "component", "component.h"),
			filepath.Join(project, "container", "container.h"),
		},
		{filepath.Join(project, "unused.h")},
	}
	objectIndexes, err := resolveLinkObjectIndexes(
		sources,
		dependencies,
		[]string{"main", "", "", ""},
		0,
		project,
	)
	if err != nil {
		t.Fatalf("resolveLinkObjectIndexes() error = %v", err)
	}
	if want := []int{0, 1, 2}; !reflect.DeepEqual(objectIndexes, want) {
		t.Fatalf("resolveLinkObjectIndexes() = %#v, want %#v", objectIndexes, want)
	}

	tasks, err := planLinkJobs(
		root,
		"host",
		sources,
		dependencies,
		[]string{"main", "", "", ""},
		len(sources),
		"bin/",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("planLinkJobs() returned %d tasks, want 1", len(tasks))
	}

	wantObjects := make([]string, 0, 3)
	for _, source := range sources[:3] {
		object, err := objectFilePath(root, "host", source)
		if err != nil {
			t.Fatalf("objectFilePath(%q) error = %v", source, err)
		}
		wantObjects = append(wantObjects, object)
	}
	if !reflect.DeepEqual(tasks[0].objects, wantObjects) {
		t.Fatalf("link objects = %#v, want %#v", tasks[0].objects, wantObjects)
	}
	wantDestination := filepath.Join(project, "bin", "app")
	if tasks[0].destination != wantDestination {
		t.Fatalf("link destination = %q, want %q", tasks[0].destination, wantDestination)
	}
	if tasks[0].display != filepath.Join("bin", "app") {
		t.Fatalf("link display = %q, want bin/app", tasks[0].display)
	}
}

func TestPlanLinkJobsSkipsOtherEntrySources(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	for _, path := range []string{"app.cpp", "service.cpp", "service.h"} {
		writeBuildFile(t, project, path, "")
	}
	withWorkingDirectory(t, project)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp", "service.cpp"},
		[][]string{{filepath.Join(project, "service.h")}, {filepath.Join(project, "service.h")}},
		[]string{"main", "main"},
		2,
		"bin/",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("planLinkJobs() returned %d tasks, want 2", len(tasks))
	}
	for _, task := range tasks {
		if len(task.objects) != 1 {
			t.Errorf("link objects for %s = %#v, want only its own object", task.source, task.objects)
		}
	}
}

func TestPlanLinkJobsDoesNotTargetDiscoveredEntrySource(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	for _, path := range []string{"app.cpp", "service.cpp", "service.h"} {
		writeBuildFile(t, project, path, "")
	}
	withWorkingDirectory(t, project)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp", "service.cpp"},
		[][]string{{filepath.Join(project, "service.h")}, nil},
		[]string{"main", "main"},
		1,
		"bin/",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("planLinkJobs() returned %d tasks, want 1", len(tasks))
	}
	if tasks[0].source != "app.cpp" {
		t.Fatalf("link source = %q, want app.cpp", tasks[0].source)
	}
	if len(tasks[0].objects) != 1 {
		t.Fatalf("link objects = %#v, want only root entry object", tasks[0].objects)
	}
}

func TestPlanLinkJobsRejectsAmbiguousImplementation(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	for _, path := range []string{"app.cpp", "library.c", "library.cpp", "library.h"} {
		writeBuildFile(t, project, path, "")
	}
	withWorkingDirectory(t, project)

	_, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp", "library.c", "library.cpp"},
		[][]string{{filepath.Join(project, "library.h")}, nil, nil},
		[]string{"main", "", ""},
		3,
		"",
		project,
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	for _, want := range []string{"library.h", "library.c", "library.cpp"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("planLinkJobs() error = %q, want %q", err, want)
		}
	}
}

func TestPlanLinkJobsRejectsExactOutputForMultipleEntries(t *testing.T) {
	_, err := planLinkJobs(
		t.TempDir(),
		"host",
		[]string{"first.cpp", "second.cpp"},
		[][]string{nil, nil},
		[]string{"main", "main"},
		2,
		"application",
		t.TempDir(),
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	if !strings.Contains(err.Error(), "requires exactly one entry source") {
		t.Fatalf("planLinkJobs() error = %q", err)
	}
}

func TestPlanLinkJobsRejectsInternalBinaryCollision(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.c", "")
	writeBuildFile(t, project, "app.cpp", "")
	withWorkingDirectory(t, project)

	_, err := planLinkJobs(
		root,
		"host",
		[]string{"app.c", "app.cpp"},
		[][]string{nil, nil},
		[]string{"main", "main"},
		2,
		"bin/",
		project,
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	if !strings.Contains(err.Error(), "binary artifact collision") {
		t.Fatalf("planLinkJobs() error = %q", err)
	}
}

func TestPlanLinkJobsPreservesSourceDirectoriesInDeliveryPath(t *testing.T) {
	tests := []struct {
		name   string
		output string
		prefix string
	}{
		{name: "default"},
		{name: "output directory", output: "bin/", prefix: "bin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			project := t.TempDir()
			writeBuildFile(t, project, "first/app.cpp", "")
			writeBuildFile(t, project, "second/app.cpp", "")
			withWorkingDirectory(t, project)

			tasks, err := planLinkJobs(
				root,
				"host",
				[]string{"first/app.cpp", "second/app.cpp"},
				[][]string{nil, nil},
				[]string{"main", "main"},
				2,
				tt.output,
				project,
			)
			if err != nil {
				t.Fatalf("planLinkJobs() error = %v", err)
			}
			if len(tasks) != 2 {
				t.Fatalf("planLinkJobs() returned %d tasks, want 2", len(tasks))
			}
			for index, directory := range []string{"first", "second"} {
				relative := filepath.Join(tt.prefix, directory, "app")
				want := filepath.Join(project, relative)
				if tasks[index].destination != want {
					t.Errorf("link destination %d = %q, want %q", index, tasks[index].destination, want)
				}
				if tasks[index].display != relative {
					t.Errorf("link display %d = %q, want %q", index, tasks[index].display, relative)
				}
			}
		})
	}
}

func TestPlanLinkJobsMirrorsOutsideSourceInOutputDirectory(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	outside := t.TempDir()
	source := filepath.Join(outside, "nested", "app.cpp")
	writeBuildFile(t, outside, "nested/app.cpp", "")
	withWorkingDirectory(t, project)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{source},
		[][]string{nil},
		[]string{"main"},
		1,
		"bin/",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	volume := filepath.VolumeName(source)
	mirrored := strings.TrimLeft(source[len(volume):], string(filepath.Separator))
	want := filepath.Join(project, "bin", strings.TrimSuffix(mirrored, filepath.Ext(mirrored)))
	if tasks[0].destination != want {
		t.Fatalf("link destination = %q, want %q", tasks[0].destination, want)
	}
}

func TestPlanLinkJobsUsesExistingOutputDirectory(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "")
	if err := os.Mkdir(filepath.Join(project, "bin"), 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	withWorkingDirectory(t, project)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"bin",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	want := filepath.Join(project, "bin", "app")
	if tasks[0].destination != want {
		t.Fatalf("link destination = %q, want %q", tasks[0].destination, want)
	}
}

func TestPlanLinkJobsRejectsFileUsedAsOutputDirectory(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "")
	writeBuildFile(t, project, "bin", "not a directory")
	withWorkingDirectory(t, project)

	_, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"bin/",
		project,
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	if !strings.Contains(err.Error(), "output directory is not a directory") {
		t.Fatalf("planLinkJobs() error = %q", err)
	}
}

func TestPlanLinkJobsRejectsEmptyBinaryName(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, ".cpp", "")
	withWorkingDirectory(t, project)

	_, err := planLinkJobs(
		root,
		"host",
		[]string{".cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"",
		project,
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	if !strings.Contains(err.Error(), "cannot derive binary name") {
		t.Fatalf("planLinkJobs() error = %q", err)
	}
}

func TestPlanLinkJobsDefaultsOutputBesideEntrySource(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "nested/app.cpp", "")
	withWorkingDirectory(t, project)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{"nested/app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"",
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	want := filepath.Join(project, "nested", "app")
	if tasks[0].destination != want {
		t.Fatalf("link destination = %q, want %q", tasks[0].destination, want)
	}
}

func TestPlanLinkJobsDisplaysAbsoluteOutputAsAbsolute(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "")
	withWorkingDirectory(t, project)
	output := filepath.Join(t.TempDir(), "bin") + string(filepath.Separator)

	tasks, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		output,
		project,
	)
	if err != nil {
		t.Fatalf("planLinkJobs() error = %v", err)
	}
	if tasks[0].display != tasks[0].destination {
		t.Fatalf("link display = %q, want absolute destination %q", tasks[0].display, tasks[0].destination)
	}
}

func TestPlanLinkJobsRejectsSymlinkDestination(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "")
	writeBuildFile(t, project, "existing", "binary")
	if err := os.Symlink("existing", filepath.Join(project, "app")); err != nil {
		t.Skipf("cannot create output symlink: %v", err)
	}
	withWorkingDirectory(t, project)

	_, err := planLinkJobs(
		root,
		"host",
		[]string{"app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"",
		project,
	)
	if err == nil {
		t.Fatal("planLinkJobs() error = nil")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("planLinkJobs() error = %q", err)
	}
}

func TestLinkArgumentsUseOrdinaryLinking(t *testing.T) {
	objects := []string{"entry.o", "library.o"}
	ldflags := []string{"-static-libgcc", "-lcustom"}
	got := linkArguments(ldflags, objects, "application")
	want := []string{"entry.o", "library.o", "-static-libgcc", "-lcustom", "-o", "application"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linkArguments() = %#v, want %#v", got, want)
	}
	for _, forbidden := range []string{"-nostartfiles", "-Wl,-e,_start"} {
		if slicesContain(got, forbidden) {
			t.Fatalf("linkArguments() contains %q: %#v", forbidden, got)
		}
	}
}

func TestRenderLinkCommandQuotesShellArguments(t *testing.T) {
	got := string(renderLinkCommand(
		"custom c++",
		[]string{"-Wl,-rpath,/path with space", "-lO'Reilly"},
		[]string{"/tmp/entry file.o"},
		"/tmp/output file",
	))
	want := "'custom c++' '/tmp/entry file.o' '-Wl,-rpath,/path with space' " +
		"'-lO'\"'\"'Reilly' -o '/tmp/output file'\n"
	if got != want {
		t.Fatalf("renderLinkCommand() = %q, want %q", got, want)
	}
}

func TestLinkSourcesOutputModes(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		silent  bool
	}{
		{name: "normal"},
		{name: "verbose", verbose: true},
		{name: "silent", verbose: true, silent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			project := t.TempDir()
			writeBuildFile(t, project, "app.cpp", "")
			withWorkingDirectory(t, project)
			compiler := installBuildCompiler(t)
			ldflags := []string{"-Lcustom path", "-lcustom"}

			var stdout bytes.Buffer
			progress := newProgressBar(&stdout, 2, tt.verbose, tt.silent, true)
			if err := linkSourcesWithCache(
				root,
				"host",
				compiler,
				ldflags,
				[]string{"app.cpp"},
				[][]string{nil},
				[]string{"main"},
				1,
				"bin/",
				2,
				tt.verbose,
				tt.silent,
				project,
				progress,
				io.Discard,
				nil,
			); err != nil {
				t.Fatalf("linkSources() error = %v", err)
			}
			if err := progress.finish(); err != nil {
				t.Fatalf("progress.finish() error = %v", err)
			}

			destination := filepath.Join(project, "bin", "app")
			if got := readTestFile(t, destination); got != "object\n" {
				t.Fatalf("copied binary = %q", got)
			}
			artifact, err := binaryArtifactPath(root, "host", "app.cpp")
			if err != nil {
				t.Fatalf("binaryArtifactPath() error = %v", err)
			}
			command := string(renderLinkCommand(compiler, ldflags, []string{mustObjectPath(t, root, "app.cpp")}, artifact))
			output := stdout.String()
			switch {
			case tt.silent:
				if output != "" {
					t.Fatalf("linkSources() output = %q, want empty", output)
				}
			case tt.verbose:
				want := "[1/2] Linking bin/app\n" + command + "[2/2] Copying bin/app\n"
				if !strings.Contains(output, want) {
					t.Fatalf("linkSources() output = %q, want verbose command", output)
				}
			default:
				if !strings.Contains(output, "\r[1/2] Linking bin/app\r[2/2] Copying bin/app\n") {
					t.Fatalf("linkSources() output = %q, want one-line progress", output)
				}
				if strings.Contains(output, command) {
					t.Fatalf("linkSources() output contains verbose command: %q", output)
				}
			}
		})
	}
}

func TestLinkSourcesBuildsMultipleBinaries(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "first/app.cpp", "")
	writeBuildFile(t, project, "second/app.cpp", "")
	withWorkingDirectory(t, project)

	progress := newProgressBar(io.Discard, 4, false, true, true)
	err := linkSourcesWithCache(
		root,
		"host",
		installBuildCompiler(t),
		nil,
		[]string{"first/app.cpp", "second/app.cpp"},
		[][]string{nil, nil},
		[]string{"main", "main"},
		2,
		"bin/",
		2,
		false,
		true,
		project,
		progress,
		io.Discard,
		nil,
	)
	if err != nil {
		t.Fatalf("linkSources() error = %v", err)
	}
	for _, directory := range []string{"first", "second"} {
		path := filepath.Join(project, "bin", directory, "app")
		if got := readTestFile(t, path); got != "object\n" {
			t.Fatalf("copied binary %s = %q", path, got)
		}
	}
}

func TestLinkSourcesWritesCompilerErrorsInSilentMode(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "")
	withWorkingDirectory(t, project)
	compiler := filepath.Join(t.TempDir(), "c++")
	script := "#!/bin/sh\n" +
		"printf 'link error\\n' >&2\n" +
		"exit 7\n"
	if err := os.WriteFile(compiler, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake linker: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	progress := newProgressBar(&stdout, 2, true, true, false)
	err := linkSourcesWithCache(
		root,
		"host",
		compiler,
		nil,
		[]string{"app.cpp"},
		[][]string{nil},
		[]string{"main"},
		1,
		"",
		1,
		true,
		true,
		project,
		progress,
		&stderr,
		nil,
	)
	if err == nil {
		t.Fatal("linkSources() error = nil")
	}
	if stdout.Len() != 0 {
		t.Fatalf("linkSources() stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != "link error\n" {
		t.Fatalf("linkSources() stderr = %q, want link error", got)
	}
}

func TestCopyBinaryReplacesRegularFileAndPreservesMode(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "linked")
	destination := filepath.Join(directory, "delivered")
	if err := os.WriteFile(source, []byte("new binary\n"), 0o751); err != nil {
		t.Fatalf("write source binary: %v", err)
	}
	if err := os.WriteFile(destination, []byte("old binary\n"), 0o600); err != nil {
		t.Fatalf("write destination binary: %v", err)
	}

	if err := copyBinary(source, destination); err != nil {
		t.Fatalf("copyBinary() error = %v", err)
	}
	if got := readTestFile(t, destination); got != "new binary\n" {
		t.Fatalf("copied binary = %q", got)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat copied binary: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o751); got != want {
		t.Fatalf("copied binary mode = %v, want %v", got, want)
	}
	matches, err := filepath.Glob(filepath.Join(directory, ".delivered.*"))
	if err != nil {
		t.Fatalf("glob temporary binaries: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary binaries remain: %#v", matches)
	}
}

func TestBuildSourcesDiscoversDependencyObjectAndRunsBinary(t *testing.T) {
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skipf("c++ is unavailable: %v", err)
	}
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "library.h", "#pragma once\nint answer();\n")
	writeBuildFile(t, project, "library.cpp", "#include \"library.h\"\nint answer() { return 42; }\n")
	writeBuildFile(t, project, "app.cpp", "#include \"library.h\"\nint main() { return answer() == 42 ? 0 : 1; }\n")
	withWorkingDirectory(t, project)

	output := filepath.Join(project, "dist", "application")
	if err := buildSources(
		root,
		"integration",
		compiler,
		[]string{"-std=c++20"},
		[]string{"-std=c++20"},
		[]string{"main", "_start"},
		[]string{"app.cpp"},
		output,
		2,
		false,
		true,
		true,
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("buildSources() error = %v", err)
	}
	if err := exec.Command(output).Run(); err != nil {
		t.Fatalf("run linked binary: %v", err)
	}
	object, err := objectFilePath(root, "integration", "library.cpp")
	if err != nil {
		t.Fatalf("objectFilePath() error = %v", err)
	}
	if _, err := os.Stat(object); err != nil {
		t.Fatalf("stat discovered dependency object: %v", err)
	}
	artifact, err := binaryArtifactPath(root, "integration", "app.cpp")
	if err != nil {
		t.Fatalf("binaryArtifactPath() error = %v", err)
	}
	for _, path := range []string{artifact, output} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat binary %s: %v", path, err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("binary %s is not executable: %v", path, info.Mode())
		}
	}
}

func TestBuildSourcesDiscoversCircularDependencyObjects(t *testing.T) {
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skipf("c++ is unavailable: %v", err)
	}
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "a.h", "#pragma once\n#include \"b.h\"\nint a();\n")
	writeBuildFile(t, project, "b.h", "#pragma once\n#include \"a.h\"\nint b();\n")
	writeBuildFile(t, project, "a.cpp", "#include \"a.h\"\nint a() { return b(); }\n")
	writeBuildFile(t, project, "b.cpp", "#include \"b.h\"\nint b() { return 42; }\n")
	writeBuildFile(t, project, "app.cpp", "#include \"a.h\"\nint main() { return a() == 42 ? 0 : 1; }\n")
	withWorkingDirectory(t, project)

	output := filepath.Join(project, "application")
	if err := buildSources(
		root,
		"integration",
		compiler,
		[]string{"-std=c++20"},
		[]string{"-std=c++20"},
		[]string{"main", "_start"},
		[]string{"app.cpp"},
		output,
		2,
		false,
		true,
		true,
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("buildSources() error = %v", err)
	}
	if err := exec.Command(output).Run(); err != nil {
		t.Fatalf("run linked binary: %v", err)
	}
	for _, source := range []string{"app.cpp", "a.cpp", "b.cpp"} {
		object, err := objectFilePath(root, "integration", source)
		if err != nil {
			t.Fatalf("objectFilePath(%q) error = %v", source, err)
		}
		if _, err := os.Stat(object); err != nil {
			t.Fatalf("stat object for %s: %v", source, err)
		}
	}
}

func mustObjectPath(t *testing.T, root, source string) string {
	t.Helper()
	object, err := objectFilePath(root, "host", source)
	if err != nil {
		t.Fatalf("objectFilePath() error = %v", err)
	}
	return object
}

func slicesContain(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func installBuildCompiler(t *testing.T) string {
	t.Helper()
	compiler := filepath.Join(t.TempDir(), "c++")
	script := "#!/bin/sh\n" +
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
	return compiler
}

func writeBuildFile(t *testing.T, root, path, contents string) {
	t.Helper()
	path = filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func withWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("determine working directory: %v", err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
