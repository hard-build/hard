package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAnalyzeClangFileCollectsIncludesDeclarationsAndFunctions(t *testing.T) {
	directory := t.TempDir()
	writeClangTestFile(t, directory, "transitive.hpp", "namespace project { struct transitive {}; }\n")
	writeClangTestFile(t, directory, "direct.hpp", "#include \"transitive.hpp\"\nnamespace project { class direct {}; }\n")
	source := writeClangTestFile(
		t,
		directory,
		"source.cpp",
		"#include <vector>\n#include \"direct.hpp\"\nint main() { return 0; }\n",
	)

	analysis, err := analyzeClangFile(
		source,
		nil,
		[]string{"-x", "c++", "-std=c++20", "-I" + directory},
		clangAnalysisOptions{},
	)
	if err != nil {
		t.Fatalf("analyzeClangFile() error = %v", err)
	}

	includes := make(map[string]clangInclude)
	for _, include := range analysis.includes {
		includes[include.spelling] = include
	}
	for _, spelling := range []string{"vector", "direct.hpp", "transitive.hpp"} {
		if _, ok := includes[spelling]; !ok {
			t.Errorf("includes do not contain %q: %#v", spelling, analysis.includes)
		}
	}
	if include := includes["vector"]; !include.system {
		t.Errorf("vector include system = false: %#v", include)
	}
	for _, spelling := range []string{"direct.hpp", "transitive.hpp"} {
		include := includes[spelling]
		if include.system {
			t.Errorf("%s include system = true", spelling)
		}
		if include.target == "" {
			t.Errorf("%s include target is empty", spelling)
		}
	}

	var declarations []string
	for _, declaration := range analysis.declarations {
		if declaration.name == "direct" || declaration.name == "transitive" {
			declarations = append(
				declarations,
				clangQualifiedName(declaration.namespaces, declaration.name),
			)
		}
	}
	sort.Strings(declarations)
	if want := []string{"project::direct", "project::transitive"}; !reflect.DeepEqual(declarations, want) {
		t.Errorf("declarations = %#v, want %#v", declarations, want)
	}

	foundMain := false
	for _, function := range analysis.functions {
		if function.name == "main" && function.definition && function.global {
			foundMain = true
		}
	}
	if !foundMain {
		t.Errorf("global main definition was not found: %#v", analysis.functions)
	}
}

func TestAnalyzeClangFilePreservesUnresolvedIncludeSpelling(t *testing.T) {
	directory := t.TempDir()
	source := writeClangTestFile(
		t,
		directory,
		"source.cpp",
		"#define DEPENDENCY <github.com/owner/repository/include/header.hpp>\n#include DEPENDENCY\n",
	)

	analysis, err := analyzeClangFile(
		source,
		nil,
		[]string{"-x", "c++", "-std=c++20", "-I" + directory},
		clangAnalysisOptions{skipFunctionBodies: true},
	)
	if err != nil {
		t.Fatalf("analyzeClangFile() error = %v", err)
	}

	for _, include := range analysis.includes {
		if include.spelling == "github.com/owner/repository/include/header.hpp" {
			if include.target != "" {
				t.Errorf("unresolved include target = %q", include.target)
			}
			return
		}
	}
	t.Fatalf("unresolved include was not found: %#v; diagnostics: %#v", analysis.includes, analysis.diagnostics)
}

func TestAnalyzeClangFileResolvesMacroNamespacesAndTemplates(t *testing.T) {
	directory := t.TempDir()
	header := writeClangTestFile(
		t,
		directory,
		"types.hpp",
		"#define ABI_NAMESPACE json_abi_v1\n"+
			"namespace library { inline namespace ABI_NAMESPACE {\n"+
			"template <typename... Types> struct pack {};\n"+
			"template <typename Type, typename Nested = pack<pack<Type>>, int Size = 4> class value {};\n"+
			"}}\n",
	)

	analysis, err := analyzeClangFile(
		header,
		nil,
		[]string{"-x", "c++-header", "-std=c++20"},
		clangAnalysisOptions{skipFunctionBodies: true},
	)
	if err != nil {
		t.Fatalf("analyzeClangFile() error = %v", err)
	}

	declarations := make(map[string]clangDeclaration)
	for _, declaration := range analysis.declarations {
		declarations[declaration.name] = declaration
	}
	value, ok := declarations["value"]
	if !ok {
		t.Fatalf("value template declaration was not found: %#v", analysis.declarations)
	}
	if got, want := clangQualifiedName(value.namespaces, value.name),
		"library::json_abi_v1::value"; got != want {
		t.Errorf("qualified name = %q, want %q", got, want)
	}
	if len(value.namespaces) != 2 || !value.namespaces[1].inline {
		t.Errorf("namespaces = %#v", value.namespaces)
	}
	if want := []string{
		"typename Type",
		"typename Nested = pack < pack < Type >>",
		"int Size = 4",
	}; !reflect.DeepEqual(value.templates, want) {
		t.Errorf("value templates = %#v, want %#v", value.templates, want)
	}
	pack, ok := declarations["pack"]
	if !ok {
		t.Fatalf("parameter-pack declaration was not found: %#v", analysis.declarations)
	}
	if want := []string{"typename ... Types"}; !reflect.DeepEqual(pack.templates, want) {
		t.Errorf("pack templates = %#v, want %#v", pack.templates, want)
	}
}

func TestSourceDependenciesRespectsConditionalIncludesAndCFlags(t *testing.T) {
	directory := t.TempDir()
	writeClangTestFile(t, directory, "enabled.hpp", "#pragma once\n")
	writeClangTestFile(t, directory, "disabled.hpp", "#pragma once\n")
	writeClangTestFile(
		t,
		directory,
		"source.cpp",
		"#if ENABLED\n#include \"enabled.hpp\"\n#else\n#include \"disabled.hpp\"\n#endif\n",
	)

	fatal, dependencies, err := sourceDependencies(
		nil,
		[]string{"-DENABLED=1"},
		"source.cpp",
		directory,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	if want := []string{filepath.Join(directory, "enabled.hpp")}; !reflect.DeepEqual(dependencies, want) {
		t.Fatalf("sourceDependencies() = %#v, want %#v", dependencies, want)
	}
}

func TestSourceDependenciesCanonicalizesAndDeduplicatesSymlinks(t *testing.T) {
	directory := t.TempDir()
	writeClangTestFile(t, directory, "include/real.hpp", "#pragma once\n")
	if err := os.Symlink("include/real.hpp", filepath.Join(directory, "alias.hpp")); err != nil {
		t.Fatalf("create header symlink: %v", err)
	}
	writeClangTestFile(
		t,
		directory,
		"source.cpp",
		"#include \"alias.hpp\"\n#include \"include/real.hpp\"\n",
	)

	fatal, dependencies, err := sourceDependencies(
		nil,
		nil,
		"source.cpp",
		directory,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	if want := []string{filepath.Join(directory, "include", "real.hpp")}; !reflect.DeepEqual(dependencies, want) {
		t.Fatalf("sourceDependencies() = %#v, want %#v", dependencies, want)
	}
}

func clangQualifiedName(namespaces []forwardNamespace, name string) string {
	parts := make([]string, 0, len(namespaces)+1)
	for _, namespace := range namespaces {
		parts = append(parts, namespace.name)
	}
	parts = append(parts, name)
	return strings.Join(parts, "::")
}

func writeClangTestFile(t *testing.T, directory, name, contents string) string {
	t.Helper()
	path := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
