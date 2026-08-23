package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseLibraryRecipeYAML(t *testing.T) {
	contents := []byte(`/* clang-format off */
/* hard.recipe.v1
source: "github.com/owner/library"
build_system: "cmake"
source_directory: "."
configure_arguments:
  - "-DBUILD_TESTING=OFF"
source_include_directories:
  - "."
include_directories:
  - "include"
static_libraries:
  - "lib/liblibrary.a"
*/
/* clang-format on */
#pragma once
#include <library.h>
`)

	got, found, err := parseLibraryRecipe(contents)
	if err != nil {
		t.Fatalf("parseLibraryRecipe() error = %v", err)
	}
	if !found {
		t.Fatal("parseLibraryRecipe() found = false")
	}
	want := libraryRecipe{
		Source:                   "github.com/owner/library",
		BuildSystem:              "cmake",
		SourceDirectory:          ".",
		ConfigureArguments:       []string{"-DBUILD_TESTING=OFF"},
		SourceIncludeDirectories: []string{"."},
		IncludeDirectories:       []string{"include"},
		StaticLibraries:          []string{"lib/liblibrary.a"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLibraryRecipe() = %#v, want %#v", got, want)
	}
}

func TestParseLibraryRecipeRejectsNonStrictYAML(t *testing.T) {
	tests := []struct {
		name     string
		document string
		want     string
	}{
		{
			name: "unknown field",
			document: validLibraryRecipeYAML() +
				"unknown: value\n",
			want: "field unknown not found",
		},
		{
			name: "duplicate key",
			document: validLibraryRecipeYAML() +
				"source: github.com/owner/other\n",
			want: "mapping key \"source\" already defined",
		},
		{
			name: "second document",
			document: validLibraryRecipeYAML() +
				"---\nsource: github.com/owner/other\n",
			want: "multiple YAML documents",
		},
		{
			name: "anchor",
			document: strings.Replace(
				validLibraryRecipeYAML(),
				"source: github.com/owner/library",
				"source: &source github.com/owner/library",
				1,
			),
			want: "anchors are not allowed",
		},
		{
			name: "alias",
			document: strings.Replace(
				validLibraryRecipeYAML(),
				"build_system: cmake",
				"build_system: *source",
				1,
			),
			want: "unknown anchor",
		},
		{
			name: "merge",
			document: "defaults: &defaults\n  build_system: cmake\n<<: *defaults\n" +
				validLibraryRecipeYAML(),
			want: "anchors are not allowed",
		},
		{
			name: "custom tag",
			document: strings.Replace(
				validLibraryRecipeYAML(),
				"source: github.com/owner/library",
				"source: !repository github.com/owner/library",
				1,
			),
			want: "tag is not allowed",
		},
		{
			name: "escaping path",
			document: strings.Replace(
				validLibraryRecipeYAML(),
				"source_directory: .",
				"source_directory: ../outside",
				1,
			),
			want: "path escapes its root",
		},
		{
			name: "managed compiler",
			document: strings.Replace(
				validLibraryRecipeYAML(),
				"configure_arguments: []",
				"configure_arguments: [-DCMAKE_CXX_COMPILER=other]",
				1,
			),
			want: "managed by hard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents := []byte("/* hard.recipe.v1\n" + tt.document + "*/\n#pragma once\n")
			_, _, err := parseLibraryRecipe(contents)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseLibraryRecipe() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseLibraryRecipeOnlyUsesLeadingComments(t *testing.T) {
	contents := []byte("#pragma once\n/* hard.recipe.v1\n" + validLibraryRecipeYAML() + "*/\n")
	_, found, err := parseLibraryRecipe(contents)
	if err != nil {
		t.Fatalf("parseLibraryRecipe() error = %v", err)
	}
	if found {
		t.Fatal("parseLibraryRecipe() found recipe after a C++ token")
	}

	contents = []byte(
		"/* hard.recipe.v1\n" + validLibraryRecipeYAML() + "*/\n" +
			"/* hard.recipe.v1\n" + validLibraryRecipeYAML() + "*/\n",
	)
	_, _, err = parseLibraryRecipe(contents)
	if err == nil || !strings.Contains(err.Error(), "multiple hard.recipe.v1 blocks") {
		t.Fatalf("parseLibraryRecipe() duplicate error = %v", err)
	}
}

func TestParseLibraryRecipeIgnoresOldMarker(t *testing.T) {
	contents := []byte("/* hard.library.v1\n" + validLibraryRecipeYAML() + "*/\n#pragma once\n")
	_, found, err := parseLibraryRecipe(contents)
	if err != nil {
		t.Fatalf("parseLibraryRecipe() error = %v", err)
	}
	if found {
		t.Fatal("parseLibraryRecipe() recognized the old hard.library.v1 marker")
	}
}

func TestLibraryManagerBuildsAndReusesCMakePackage(t *testing.T) {
	root := t.TempDir()
	workingDirectory := t.TempDir()
	repositoryRoot := filepath.Join(root, "source", "github.com", "owner", "library")
	writeBuildFile(t, repositoryRoot, "CMakeLists.txt", "cmake_minimum_required(VERSION 3.10)\n")
	writeBuildFile(t, repositoryRoot, "library.h", "#pragma once\n")
	header := filepath.Join(workingDirectory, "library.hard.h")
	writeBuildFile(
		t,
		workingDirectory,
		"library.hard.h",
		"/* hard.recipe.v1\n"+validLibraryRecipeYAML()+"*/\n#pragma once\n",
	)
	tools := t.TempDir()
	compiler := filepath.Join(tools, "custom-c++")
	if err := os.WriteFile(compiler, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	log := filepath.Join(t.TempDir(), "cmake.log")
	cmake := filepath.Join(tools, "cmake")
	cmakeScript := `#!/bin/sh
printf 'CXXFLAGS=%s\n' "$CXXFLAGS" >> "$CMAKE_LOG"
printf 'cmake' >> "$CMAKE_LOG"
for argument in "$@"; do printf ' %s' "$argument" >> "$CMAKE_LOG"; done
printf '\n' >> "$CMAKE_LOG"
case "$1" in
  -S)
    previous=''
    for argument in "$@"; do
      if [ "$previous" = '-B' ]; then build=$argument; fi
      case "$argument" in -DCMAKE_INSTALL_PREFIX=*) prefix=${argument#*=} ;; esac
      previous=$argument
    done
    mkdir -p "$build"
    printf '%s\n' "$prefix" > "$build/prefix"
    ;;
  --install)
    prefix=$(cat "$2/prefix")
    mkdir -p "$prefix/include" "$prefix/lib"
    cp "$LIBRARY_SOURCE/library.h" "$prefix/include/library.h"
    printf 'archive\n' > "$prefix/lib/liblibrary.a"
    ;;
esac
`
	if err := os.WriteFile(cmake, []byte(cmakeScript), 0o755); err != nil {
		t.Fatalf("write fake cmake: %v", err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CMAKE_LOG", log)
	t.Setenv("LIBRARY_SOURCE", repositoryRoot)
	t.Setenv("CXXFLAGS", "ambient-flags-must-not-leak")

	cache, err := newArtifactCache(true)
	if err != nil {
		t.Fatalf("newArtifactCache() error = %v", err)
	}
	manager := newLibraryManager(
		root,
		"host",
		compiler,
		2,
		true,
		false,
		workingDirectory,
		newGitHubSnapshotResolver(root, nil),
		cache,
		nil,
		io.Discard,
	)
	artifacts, err := manager.prepareHeaders([]string{header})
	if err != nil {
		t.Fatalf("prepareHeaders() error = %v", err)
	}
	if len(artifacts) != 1 || len(artifacts[0].cflags) != 1 || len(artifacts[0].archives) != 1 {
		t.Fatalf("prepareHeaders() artifacts = %#v", artifacts)
	}
	if !strings.HasSuffix(artifacts[0].cflags[0], filepath.Join("install", "include")) {
		t.Fatalf("installed include flag = %q", artifacts[0].cflags[0])
	}
	if !strings.HasSuffix(artifacts[0].archives[0], filepath.Join("install", "lib", "liblibrary.a")) {
		t.Fatalf("installed archive = %q", artifacts[0].archives[0])
	}

	firstLog := readTestFile(t, log)
	resolvedCompiler, err := filepath.EvalSymlinks(compiler)
	if err != nil {
		t.Fatalf("resolve fake compiler: %v", err)
	}
	for _, want := range []string{
		"CXXFLAGS=\n",
		"-DCMAKE_CXX_COMPILER=" + resolvedCompiler,
		"--parallel 2",
	} {
		if !strings.Contains(firstLog, want) {
			t.Errorf("cmake log does not contain %q:\n%s", want, firstLog)
		}
	}
	if got := strings.Count(firstLog, "cmake "); got != 3 {
		t.Fatalf("cmake invocation count = %d, want 3:\n%s", got, firstLog)
	}

	secondManager := newLibraryManager(
		root,
		"host",
		compiler,
		2,
		true,
		false,
		workingDirectory,
		newGitHubSnapshotResolver(root, nil),
		cache,
		nil,
		io.Discard,
	)
	if _, err := secondManager.prepareHeaders([]string{header}); err != nil {
		t.Fatalf("cached prepareHeaders() error = %v", err)
	}
	if got := readTestFile(t, log); got != firstLog {
		t.Fatalf("cached package reran cmake:\n%s", got)
	}
}

func TestLibraryManagerFetchUsesSourceIncludesWithoutEnvironment(t *testing.T) {
	root := t.TempDir()
	workingDirectory := t.TempDir()
	repositoryRoot := filepath.Join(root, "source", "github.com", "owner", "library")
	writeBuildFile(t, repositoryRoot, "library.h", "#pragma once\n")
	header := filepath.Join(workingDirectory, "library.hard.h")
	writeBuildFile(
		t,
		workingDirectory,
		"library.hard.h",
		"/* hard.recipe.v1\n"+validLibraryRecipeYAML()+"*/\n#pragma once\n",
	)
	manager := newLibraryManager(
		root,
		"",
		"",
		1,
		false,
		false,
		workingDirectory,
		newGitHubSnapshotResolver(root, nil),
		nil,
		nil,
		io.Discard,
	)
	artifacts, err := manager.prepareHeaders([]string{header})
	if err != nil {
		t.Fatalf("prepareHeaders() error = %v", err)
	}
	wantFlag := "-I" + repositoryRoot
	if len(artifacts) != 1 || !reflect.DeepEqual(artifacts[0].cflags, []string{wantFlag}) {
		t.Fatalf("source artifacts = %#v, want cflag %q", artifacts, wantFlag)
	}
	if _, err := os.Stat(filepath.Join(root, "env")); !os.IsNotExist(err) {
		t.Fatalf("fetch-only manager created environment tree: %v", err)
	}
}

func TestInspectBuildSourceTreatsUnavailableCachedLibraryAsMiss(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	repositoryRoot := filepath.Join(root, "source", "github.com", "owner", "library")
	writeBuildFile(t, repositoryRoot, "library.h", "#pragma once\n")
	writeBuildFile(t, project, "source.cpp", "#include \"library.hard.h\"\n")
	header := filepath.Join(project, "library.hard.h")
	writeBuildFile(
		t,
		project,
		"library.hard.h",
		"/* hard.recipe.v1\n"+validLibraryRecipeYAML()+"*/\n#pragma once\n",
	)

	firstCache, err := newArtifactCache(true)
	if err != nil {
		t.Fatalf("newArtifactCache() error = %v", err)
	}
	firstResolver := newGitHubSnapshotResolver(root, nil)
	firstManager := newLibraryManager(
		root,
		"host",
		"",
		1,
		false,
		false,
		project,
		firstResolver,
		firstCache,
		nil,
		io.Discard,
	)
	first := inspectBuildSourceWithCache(
		root,
		"host",
		"",
		firstResolver,
		[]string{"-std=c++20"},
		nil,
		buildJob{source: filepath.Join(project, "source.cpp")},
		project,
		nil,
		firstCache,
		firstManager,
	)
	if first.err != nil {
		t.Fatalf("first inspectBuildSourceWithCache() error = %v", first.err)
	}
	if !reflect.DeepEqual(first.libraryHeaders, []string{header}) {
		t.Fatalf("first library headers = %#v, want %#v", first.libraryHeaders, []string{header})
	}

	writeBuildFile(t, project, "source.cpp", "")
	if err := os.Remove(header); err != nil {
		t.Fatalf("remove stale library recipe: %v", err)
	}
	secondCache, err := newArtifactCache(true)
	if err != nil {
		t.Fatalf("newArtifactCache() error = %v", err)
	}
	secondResolver := newGitHubSnapshotResolver(root, nil)
	secondManager := newLibraryManager(
		root,
		"host",
		"",
		1,
		false,
		false,
		project,
		secondResolver,
		secondCache,
		nil,
		io.Discard,
	)
	var cacheHits []bool
	second := inspectBuildSourceWithCache(
		root,
		"host",
		"",
		secondResolver,
		[]string{"-std=c++20"},
		nil,
		buildJob{source: filepath.Join(project, "source.cpp")},
		project,
		func(_ string, cached bool) { cacheHits = append(cacheHits, cached) },
		secondCache,
		secondManager,
	)
	if second.err != nil {
		t.Fatalf("second inspectBuildSourceWithCache() error = %v", second.err)
	}
	if !reflect.DeepEqual(cacheHits, []bool{false}) {
		t.Fatalf("second cache hits = %#v, want cache miss", cacheHits)
	}
	if len(second.libraryHeaders) != 0 || len(second.libraries) != 0 {
		t.Fatalf("second library state = headers %#v, libraries %#v", second.libraryHeaders, second.libraries)
	}
}

func validLibraryRecipeYAML() string {
	return "source: github.com/owner/library\n" +
		"build_system: cmake\n" +
		"source_directory: .\n" +
		"configure_arguments: []\n" +
		"source_include_directories: [.]\n" +
		"include_directories: [include]\n" +
		"static_libraries: [lib/liblibrary.a]\n"
}
