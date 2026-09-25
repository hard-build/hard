package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func graphRecipe(t *testing.T, root, name string, dependencies ...string) string {
	t.Helper()
	contents := validLibraryRecipeYAML()
	if len(dependencies) > 0 {
		contents += "dependencies:\n"
		for _, dependency := range dependencies {
			contents += fmt.Sprintf("  - %q\n", dependency)
		}
	}
	writeBuildFile(t, root, name+".hard", contents)
	return filepath.Join(root, name+".hard")
}

func TestRecipeCompanionsAllHeaderExtensions(t *testing.T) {
	for _, extension := range []string{".h", ".hh", ".hpp", ".h++", ".HPP"} {
		t.Run(extension, func(t *testing.T) {
			root := t.TempDir()
			expected := graphRecipe(t, root, "demo")
			header := filepath.Join(root, "demo.hard"+extension)
			writeBuildFile(t, root, filepath.Base(header), "#pragma once\n")
			got, err := libraryDescriptorForHeader(header, root)
			if err != nil || got != expected {
				t.Fatalf("companion: %q, %v", got, err)
			}
		})
	}
	root := t.TempDir()
	header := filepath.Join(root, "missing.hard.hpp")
	writeBuildFile(t, root, filepath.Base(header), "")
	if _, err := libraryDescriptorForHeader(header, root); err == nil || !strings.Contains(err.Error(), "requires sibling missing.hard") {
		t.Fatalf("missing descriptor: %v", err)
	}
}

func TestRecipeGraphCombinesYAMLAndActiveIncludes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"leaf", "right", "unused"} {
		graphRecipe(t, root, name)
		writeBuildFile(t, root, name+".hard.h", "#pragma once\n")
	}
	graphRecipe(t, root, "left", "leaf.hard")
	graphRecipe(t, root, "parent", "left.hard", "./left.hard")
	writeBuildFile(t, root, "parent.hard.hpp", "#pragma once\n#include <vendor_missing.h>\n#include \"bridge.h\"\n#if 0\n#include \"unused.hard.h\"\n#endif\n")
	writeBuildFile(t, root, "bridge.h", "#include \"right.hard.h\"\n")
	source := filepath.Join(root, "main.cpp")
	writeBuildFile(t, root, "main.cpp", "#include \"parent.hard.hpp\"\n")
	analysis, err := analyzeClangDependencies(source, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := clangDependencyPaths(analysis, source, root)
	if err != nil {
		t.Fatal(err)
	}
	manager := newLibraryManager("", "", "", 1, false, false, root, nil, nil, nil, io.Discard)
	graph, headers, err := manager.discoverLibraries(analysis, dependencies, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph) != 4 || len(headers) != 2 {
		t.Fatalf("graph=%+v; headers=%v", graph, headers)
	}
	var names []string
	for _, node := range graph {
		names = append(names, filepath.Base(node.Descriptor))
	}
	if !reflect.DeepEqual(names, []string{"leaf.hard", "left.hard", "right.hard", "parent.hard"}) {
		t.Fatal(names)
	}
	if !reflect.DeepEqual(graph[3].Dependencies, []int{1, 2}) {
		t.Fatal(graph)
	}
	// Descriptor-only references never inspect adjacent wrappers.
	writeBuildFile(t, root, "left.hard.h", "#include \"unused.hard.h\"\n")
	repeat, _, err := manager.discoverLibraries(analysis, dependencies, nil)
	if err != nil || !reflect.DeepEqual(repeat, graph) {
		t.Fatalf("standalone descriptor changed graph: %+v %v", repeat, err)
	}
	graphRecipe(t, root, "leaf", "parent.hard")
	if _, _, err := manager.discoverLibraries(analysis, dependencies, nil); err == nil || !strings.Contains(err.Error(), "recipe dependency cycle:") {
		t.Fatalf("cycle: %v", err)
	}
}

func TestRecipeReferenceIncludeSearchAndAliases(t *testing.T) {
	root, project := t.TempDir(), t.TempDir()
	repository := filepath.Join(root, "source", "github.com", "hard-build", "recipe")
	descriptor := graphRecipe(t, repository, "leaf")
	manager := newLibraryManager(root, "", "", 1, false, false, project, newGitHubSnapshotResolver(root, nil), nil, nil, io.Discard)
	referring := graphRecipe(t, project, "parent")
	for _, reference := range []string{"recipe/leaf.hard", "github.com/hard-build/recipe/leaf.hard"} {
		got, err := manager.resolveRecipeReference(reference, referring, nil)
		if err != nil || got != descriptor {
			t.Fatalf("%s: %s, %v", reference, got, err)
		}
	}
	local := graphRecipe(t, project, "leaf")
	got, err := manager.resolveRecipeReference("leaf.hard", referring, []string{"-I" + repository})
	if err != nil || got != local {
		t.Fatalf("quoted include priority: %s, %v", got, err)
	}
	include := t.TempDir()
	alternate := graphRecipe(t, include, "alternate")
	got, err = manager.resolveRecipeReference("alternate.hard", referring, []string{"-iquote", include})
	if err != nil || got != alternate {
		t.Fatalf("include directories: %s, %v", got, err)
	}
}

func TestRecipeArchiveOrderAcrossRootsAndDiamond(t *testing.T) {
	leaf := libraryArtifact{key: "leaf", archives: []string{"leaf.a"}}
	left := libraryArtifact{key: "left", archives: []string{"left.a"}, dependencies: []string{"leaf"}}
	right := libraryArtifact{key: "right", archives: []string{"right.a"}, dependencies: []string{"leaf"}}
	parent := libraryArtifact{key: "parent", archives: []string{"parent.a"}, dependencies: []string{"left", "right"}}
	unrelated := libraryArtifact{key: "unrelated", archives: []string{"unrelated.a"}}
	got := libraryArchivesByIndexes([][]libraryArtifact{{leaf}, {leaf, left, right, parent}, {unrelated}}, []int{0, 1})
	want := []string{"parent.a", "left.a", "right.a", "leaf.a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRecipePreprocessingVariantsAndBootstrap(t *testing.T) {
	root := t.TempDir()
	header := filepath.Join(root, "value.hard.hpp")
	writeBuildFile(t, root, "value.hard.hpp", "#pragma once\n#include <absent_vendor.h>\ninline int recipe_value(){return VALUE;}\n")
	source := filepath.Join(root, "main.cpp")
	manager := newLibraryManager("", "", "c++", 1, true, false, root, nil, nil, nil, io.Discard)
	preprocess := func(value string, flags []string) string {
		t.Helper()
		writeBuildFile(t, root, "main.cpp", value+"#include \"value.hard.hpp\"\n")
		visits, err := manager.preprocessLibraryVisits(source, flags, []string{header}, []string{"absent_vendor.h"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return visits[0].Context
	}
	first := preprocess("#define VALUE 1\n", nil)
	second := preprocess("#define VALUE 2\n", nil)
	if first == second {
		t.Fatal("source macros collapsed distinct preprocessed code")
	}
	third := preprocess("", []string{"-DVALUE=1"})
	if first != third {
		t.Fatal("equal expanded code should share the context")
	}
	writeBuildFile(t, root, "value.hard.hpp", "#pragma once\n#include <absent_vendor.h>\n")
	if context := preprocess("", nil); context != "" {
		t.Fatalf("directive-only wrapper: %q", context)
	}
}

func TestRecipeDependencyVariantInvalidatesParentPackage(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	manager, header := sharedLibraryManager(t, configuration, vendor, false, io.Discard)
	descriptor := strings.TrimSuffix(header, ".h")
	graph := []libraryNode{{Descriptor: descriptor, Context: "leaf-one"}, {Descriptor: descriptor, Context: "parent", Dependencies: []int{0}}}
	first, err := manager.prepareGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	graph[0].Context = "leaf-two"
	second, err := manager.prepareGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	if first[0].key == second[0].key || first[1].key == second[1].key {
		t.Fatal("dependency context did not invalidate parent")
	}
	repeat, err := manager.prepareGraph(graph)
	if err != nil || !reflect.DeepEqual(repeat, second) {
		t.Fatalf("variants not reusable: %v", err)
	}
}

func TestLibraryInstallManifestInternalSymlinks(t *testing.T) {
	root := t.TempDir()
	writeBuildFile(t, root, "lib/library.a", "archive")
	alias := filepath.Join(root, "lib", "alias.a")
	if err := os.Symlink("library.a", alias); err != nil {
		t.Fatal(err)
	}
	first, err := libraryInstallManifestFiles(root)
	if err != nil || len(first) != 2 {
		t.Fatalf("manifest: %v %v", first, err)
	}
	writeBuildFile(t, root, "lib/library.a", "changed archive")
	second, err := libraryInstallManifestFiles(root)
	if err != nil || reflect.DeepEqual(first, second) {
		t.Fatalf("symlink target mutation not detected: %v", err)
	}
	outside := t.TempDir()
	writeBuildFile(t, outside, "other.a", "outside")
	if err := os.Symlink(filepath.Join(outside, "other.a"), filepath.Join(root, "escape.a")); err != nil {
		t.Fatal(err)
	}
	if _, err := libraryInstallManifestFiles(root); err == nil {
		t.Fatal("accepted escaping installed symlink")
	}
}

func TestRecipeRepeatedIncludesRetainSeparateDependencyVariants(t *testing.T) {
	for _, compiler := range []string{"c++", "clang++-18"} {
		t.Run(compiler, func(t *testing.T) {
			if _, err := exec.LookPath(compiler); err != nil {
				t.Skip(err)
			}
			testRecipeRepeatedIncludes(t, compiler)
		})
	}
}

func testRecipeRepeatedIncludes(t *testing.T, compiler string) {
	root := t.TempDir()
	for _, name := range []string{"one", "two", "shared", "parent"} {
		graphRecipe(t, root, name)
		writeBuildFile(t, root, name+".hard.hpp", "#pragma once\n")
	}
	writeBuildFile(t, root, "bridge.h", "#pragma once\n#include \"shared.hard.hpp\"\n")
	writeBuildFile(t, root, "parent.hard.hpp", "#if VALUE == 1\n#include \"one.hard.hpp\"\n#else\n#include \"two.hard.hpp\"\n#endif\n#include \"bridge.h\"\n")
	view := filepath.Join(t.TempDir(), "view")
	if err := os.Symlink(root, view); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(view, "main.cpp")
	writeBuildFile(t, root, "main.cpp", "#include \"bridge.h\"\n#define VALUE 1\n#include \"parent.hard.hpp\"\n#undef VALUE\n#define VALUE 2\n#include \"parent.hard.hpp\"\n")
	analysis, err := analyzeClangDependencies(source, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	dependencies, err := clangDependencyPaths(analysis, source, root)
	if err != nil {
		t.Fatal(err)
	}
	var headers []string
	for _, path := range dependencies {
		if isLibraryHeader(path) {
			headers = append(headers, path)
		}
	}
	manager := newLibraryManager("", "", compiler, 1, true, false, root, nil, nil, nil, io.Discard)
	analysis.libraryVisits, err = manager.preprocessLibraryVisits(source, nil, headers, nil, analysis.includes)
	if err != nil {
		t.Fatal(err)
	}
	graph, _, err := manager.discoverLibraries(analysis, dependencies, nil)
	if err != nil {
		t.Fatal(err)
	}
	var variants [][]string
	for _, node := range graph {
		if filepath.Base(node.Descriptor) != "parent.hard" {
			continue
		}
		if node.Context != "" {
			t.Fatalf("include directives became own code: %+v", node)
		}
		var names []string
		for _, index := range node.Dependencies {
			names = append(names, filepath.Base(graph[index].Descriptor))
		}
		variants = append(variants, names)
	}
	want := [][]string{{"one.hard", "shared.hard"}, {"two.hard", "shared.hard"}}
	if !reflect.DeepEqual(variants, want) {
		t.Fatalf("got %v, want %v; visits=%+v", variants, want, analysis.libraryVisits)
	}
}

func TestRecipeDescriptorChangesInvalidateAnalysisAndGraph(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{vendor.Source: vendor}))
	writeProjectTestFile(t, project, "main.cpp", "#include \"parent.hard.hpp\"\nint main(){return vendor_value() == 7 ? 0 : 1;}\n")
	writeProjectTestFile(t, project, "parent.hard.hpp", "#pragma once\n#include <vendor.h>\n")
	descriptor := "version: 1\nsource: github.com/demo/vendor\nbuild_system: cmake\nsource_directory: .\nsource_include_directories: [.]\ninclude_directories: [include]\nstatic_libraries: [lib/libvendor.a]\n"
	writeProjectTestFile(t, project, "parent.hard", descriptor)
	check := func(cached bool, extra ...string) {
		t.Helper()
		args := append([]string{"run", "main.cpp", "--locked", "-v", "--no-color"}, extra...)
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil || strings.Contains(out, "Parsing main.cpp (CACHED)") != cached {
			t.Fatalf("cached=%t: %v\n%s\n%s", cached, err, out, diagnostics)
		}
	}
	check(false)
	check(true)
	writeProjectTestFile(t, project, "leaf.hard", descriptor)
	writeProjectTestFile(t, project, "parent.hard", descriptor+"dependencies: [leaf.hard]\n")
	check(false)
	check(true)
	check(false, "--no-cache")
	check(true)
	writeProjectTestFile(t, project, "parent.hard", descriptor)
	if err := os.Remove(filepath.Join(project, "leaf.hard")); err != nil {
		t.Fatal(err)
	}
	check(false)
	check(true)
}

func TestRecipeBootstrapWithoutVendorFunctionMacros(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	vendor, archive := inheritedTestSnapshot(t, "github.com/owner/library", "main", firstCommit, map[string]string{
		"CMakeLists.txt": "cmake_minimum_required(VERSION 3.16)\nproject(library LANGUAGES CXX)\nadd_library(library STATIC library.cpp)\ninstall(TARGETS library ARCHIVE DESTINATION lib)\ninstall(FILES library.h DESTINATION include)\n",
		"library.h":      "#pragma once\n#define VENDOR_AT_LEAST(v) ((v) <= 1)\nint library_value();\n",
		"library.cpp":    "int library_value(){return 7;}\n",
	})
	proxy.add(vendor, archive)
	configuration := projectTestConfiguration(t)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{vendor.Source: vendor}))
	graphRecipe(t, project, "library")
	writeProjectTestFile(t, project, "library.hard.h", "#pragma once\n#include <library.h>\n#if VENDOR_AT_LEAST(1)\ninline int wrapped_value(){return library_value();}\n#endif\n")
	writeProjectTestFile(t, project, "main.cpp", "#include \"library.hard.h\"\nint main(){return wrapped_value() == 7 ? 0 : 1;}\n")
	out, diagnostics, err := runProjectTestCommand(configuration, "run", "main.cpp", "--locked", "--no-color")
	if err != nil {
		t.Fatalf("bootstrap: %v\n%s\n%s", err, out, diagnostics)
	}
}
