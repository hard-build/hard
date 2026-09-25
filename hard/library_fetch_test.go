package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFetchMissingIncludesRespectPackageBoundary(t *testing.T) {
	project, vendor := t.TempDir(), t.TempDir()
	writeBuildFile(t, project, "main.cpp", "")
	writeBuildFile(t, vendor, "library.h", "")
	writeBuildFile(t, vendor, "library.hard.h", "")
	view := filepath.Join(t.TempDir(), "source-view")
	if err := os.Symlink(vendor, view); err != nil {
		t.Fatal(err)
	}
	artifacts := []libraryArtifact{{sourceDirectory: vendor}}
	for _, test := range []struct {
		name  string
		edges []clangInclude
		want  []string
	}{
		{"generated", []clangInclude{{source: filepath.Join(view, "library.h"), spelling: "generated.h"}}, []string{}},
		{"wrapper_inside_vendor", []clangInclude{{source: filepath.Join(vendor, "library.hard.h"), spelling: "generated.h"}}, []string{"generated.h"}},
		{"same_spelling_in_project", []clangInclude{{source: filepath.Join(vendor, "library.h"), spelling: "generated.h"}, {source: filepath.Join(project, "main.cpp"), spelling: "generated.h"}}, []string{"generated.h"}},
		{"recipe_header", []clangInclude{{source: filepath.Join(vendor, "library.h"), spelling: "missing.hard.hpp"}}, []string{"missing.hard.hpp"}},
		{"descriptor", []clangInclude{{source: filepath.Join(vendor, "library.h"), spelling: "missing.hard"}}, []string{"missing.hard"}},
		{"github", []clangInclude{{source: filepath.Join(vendor, "library.h"), spelling: "github.com/demo/missing/value.h"}}, []string{"github.com/demo/missing/value.h"}},
		{"well_known", []clangInclude{{source: filepath.Join(vendor, "library.h"), spelling: "recipe/missing.hard.h"}}, []string{"recipe/missing.hard.h"}},
		{"unknown_source", []clangInclude{{spelling: "generated.h"}}, []string{"generated.h"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := fetchUnresolvedIncludes(clangAnalysis{includes: test.edges}, artifacts, project)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestFetchRecipeWithGeneratedPublicHeader(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	vendor, archive := inheritedTestSnapshot(t, "github.com/owner/library", "main", firstCommit, map[string]string{
		"CMakeLists.txt": "cmake_minimum_required(VERSION 3.16)\nproject(library LANGUAGES CXX)\nconfigure_file(generated.h.in generated.h @ONLY)\nadd_library(library STATIC library.cpp)\ntarget_include_directories(library PRIVATE ${CMAKE_CURRENT_BINARY_DIR})\ninstall(TARGETS library ARCHIVE DESTINATION lib)\ninstall(FILES library.h DESTINATION include)\nif(NOT BROKEN_INSTALL)\ninstall(FILES ${CMAKE_CURRENT_BINARY_DIR}/generated.h DESTINATION include)\nendif()\n",
		"generated.h.in": "#pragma once\n#define LIBRARY_API\n#define LIBRARY_VALUE 7\n",
		"library.h":      "#pragma once\n#include \"generated.h\"\n" + strings.Repeat("LIBRARY_API int library_value();\n", 30),
		"library.cpp":    "#include \"library.h\"\nint library_value(){return LIBRARY_VALUE;}\n",
	})
	proxy.add(vendor, archive)
	configuration := projectTestConfiguration(t)
	compiler := configuration.cc
	configuration.cc = "compiler-must-not-run"
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{vendor.Source: vendor}))
	graphRecipe(t, project, "parent", "leaf.hard")
	graphRecipe(t, project, "leaf")
	writeProjectTestFile(t, project, "parent.hard.h", "#pragma once\n#include <library.h>\n#include \"leaf.hard.h\"\n")
	writeProjectTestFile(t, project, "leaf.hard.h", "#pragma once\n")
	source := "#include \"parent.hard.h\"\nint main(){return library_value() == 7 ? 0 : 1;}\n"
	writeProjectTestFile(t, project, "main.cpp", source)
	for _, pass := range []struct{ cached, noCache bool }{{false, false}, {true, false}, {false, true}, {true, false}} {
		args := []string{"fetch", "main.cpp", "--locked", "--no-color", "-v"}
		if pass.noCache {
			args = append(args, "--no-cache")
		}
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil || diagnostics != "" || strings.Contains(out, "Parsing main.cpp (CACHED)") != pass.cached {
			t.Fatalf("fetch %+v: %v\n%s\n%s", pass, err, out, diagnostics)
		}
	}
	record, ok, err := readParseCacheRecord(projectFetchRecord(t, configuration, project, "main.cpp"))
	if err != nil || !ok || len(record.LibraryGraph) != 3 {
		t.Fatalf("lost recipe after incomplete public header: %+v, %v", record.LibraryGraph, err)
	}
	foundLeaf := false
	for _, node := range record.LibraryGraph {
		if node.Header == filepath.Join(project, "leaf.hard.h") {
			foundLeaf = true
		}
	}
	if !foundLeaf {
		t.Fatal("missing included leaf wrapper")
	}
	if packages, _ := filepath.Glob(filepath.Join(configuration.root, "project", "*", "github.com", "owner", "library", "package")); len(packages) != 0 {
		t.Fatalf("fetch created packages: %v", packages)
	}
	// A missing project include must not be hidden by the identical vendor spelling.
	writeProjectTestFile(t, project, "main.cpp", source+"#include \"generated.h\"\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "main.cpp", "--locked"); err == nil || !strings.Contains(err.Error(), "generated.h") {
		t.Fatalf("missing project header hidden: %v", err)
	}
	writeProjectTestFile(t, project, "main.cpp", source)
	writeProjectTestFile(t, project, "parent.hard.h", "#include <library.h>\n#include \"missing.hard.h\"\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "main.cpp", "--locked"); err == nil || !strings.Contains(err.Error(), "missing.hard.h") {
		t.Fatalf("missing recipe hidden: %v", err)
	}
	writeProjectTestFile(t, project, "parent.hard.h", "#pragma once\n#include <library.h>\n#include \"leaf.hard.h\"\n")
	configuration.cc = compiler
	out, diagnostics, err := runProjectTestCommand(configuration, "run", "main.cpp", "--locked", "--no-color")
	if err != nil {
		t.Fatalf("build after fetch: %v\n%s\n%s", err, out, diagnostics)
	}
	// Fetch success must not relax verification of the installed public headers.
	writeProjectTestFile(t, project, "parent.hard.h", "#pragma once\n#include <library.h>\n")
	writeProjectTestFile(t, project, "parent.hard", strings.Replace(validLibraryRecipeYAML(), "configure_arguments: []", "configure_arguments: [-DBROKEN_INSTALL=ON]", 1))
	if _, _, err := runProjectTestCommand(configuration, "run", "main.cpp", "--locked"); err == nil || !strings.Contains(err.Error(), "generated.h") {
		t.Fatalf("build accepted missing installed header: %v", err)
	}
}
