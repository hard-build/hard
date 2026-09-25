package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestForwardSignaturesCompileWithOriginalDefinitions(t *testing.T) {
	for _, test := range []struct{ name, header, want, skipped string }{
		{"parameters", "template<typename T = int, class A = int> class Collection {};", "template <typename T, class A>", ""},
		{"enum", "namespace demo { enum class Mode { Fast }; template<Mode M> class Parser {}; enum class Unused : unsigned char { X }; }", "template <Mode M>", ""},
		{"enum_base", "using Integer = unsigned long; enum Flags : Integer { First = 1 }; template<Flags F> class Options {};", "enum Flags : unsigned long;", ""},
		{"anonymous_typedef", "typedef struct { int value; } Image, *ImagePointer; typedef enum { Value } Kind;", "#pragma once", ""},
		{"named_typedef", "typedef struct Named { int value; } Named;", "struct Named;", ""},
		{"requires", "template<class T> requires (sizeof(T) > 1) class Box {};", "requires ( sizeof ( T ) > 1 )", ""},
		{"concept", "template<class T> concept Good = sizeof(T)>1; template<Good T> class Box {};", "#pragma once", "Box"},
		{"unscoped", "enum Unfixed { X }; template<Unfixed F> class Box {};", "#pragma once", "Box"},
		{"macro", "#define LIMIT 1\ntemplate<class T> requires (sizeof(T)>LIMIT) class Box {};", "#pragma once", "Box"},
		{"comparison_default", "template<int N = (1 < 2 ? 3 : 4)> class Count {};", "template <int N>", ""},
		{"builtin_alias", "#include <cstddef>\ntemplate<std::size_t N> class Array {};", "class Array;", ""},
		{"nested_template", "template<template<class U = int> class C> class Nested {};", "class Nested;", ""},
		{"nested_default", "template<class> class Inner {}; template<class T = Inner<int>> class Outer {};", "class Outer;", ""},
		{"unused_enum", "namespace demo { enum class Unused : unsigned char { X }; }", "enum class Unused : unsigned char;", ""},
		{"enum_constant_constraint", "enum class Mode { Fast }; template<Mode M> requires (M == Mode::Fast) class Box {};", "enum class Mode : int;", "Box"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := t.TempDir()
			header := filepath.Join(project, "types.h")
			writeBuildFile(t, project, "types.h", test.header+"\n")
			writeBuildFile(t, project, "main.cpp", "#include \"types.h\"\nint main() { return 0; }\n")
			analysis, err := analyzeClangFile(filepath.Join(project, "main.cpp"), nil, []string{"-std=c++20"}, clangAnalysisOptions{})
			if err != nil {
				t.Fatal(err)
			}
			declarations, skipped := selectForwardDeclarations(analysis, []string{header}, project)
			contents := renderForwardDeclarations(declarations)
			if !strings.Contains(string(contents), test.want) {
				t.Fatalf("missing %q:\n%s\nskipped %v\nmetadata %#v", test.want, contents, skipped, analysis.declarations)
			}
			if test.skipped != "" && (strings.Contains(string(contents), "class "+test.skipped) || !strings.Contains(strings.Join(skipped, "\n"), test.skipped)) {
				t.Fatalf("unsafe declaration retained or unreported: %s; %v", contents, skipped)
			}
			forward := filepath.Join(project, "main.fwd.h")
			if err := os.WriteFile(forward, contents, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("c++", "-std=c++20", "-include", forward, "-fsyntax-only", filepath.Join(project, "main.cpp"))
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("compiler: %v\n%s\nforward:\n%s\nskipped: %v", err, output, contents, skipped)
			}
		})
	}
}

func TestForwardTemplateRecoveredAfterSemanticError(t *testing.T) {
	project := t.TempDir()
	header := filepath.Join(project, "types.h")
	analysis, err := analyzeClangFile(header, []byte("template<typename type = int, class allocator> class collection {};"), []string{"-std=c++20", "-x", "c++-header"}, clangAnalysisOptions{})
	if err != nil {
		t.Fatal(err)
	}
	declarations, skipped := selectForwardDeclarations(analysis, []string{header}, project)
	if len(declarations) != 1 {
		t.Fatalf("%#v; skipped %v", analysis.declarations, skipped)
	}
}

func TestForwardMacroParametersFromAST(t *testing.T) {
	for _, test := range []struct {
		name, definitions, declaration, want string
		skipped                              bool
	}{
		{"template_template", "#define PARAM template <typename T> class", "template<PARAM Fixture, class TestSel, typename Types> class Box {};", "template <template <typename T> class Fixture, class TestSel, typename Types>", false},
		{"parameter_list", "#define PARAM template <typename T> class Fixture, class TestSel, typename Types", "template<PARAM> class Box {};", "template <template <typename T> class Fixture, class TestSel, typename Types>", false},
		{"type", "#define PARAM typename", "template<PARAM T = int> class Box {};", "template <typename T>", false},
		{"pack", "#define PARAM typename...", "template<PARAM Types> class Box {};", "template <typename ...Types>", false},
		{"value", "#define PARAM int", "template<PARAM N = (1 < 2 ? 3 : 4)> class Box {};", "template <int N>", false},
		{"enum", "enum class Mode { Fast };\n#define PARAM Mode", "template<PARAM M> class Box {};", "class Box;", false},
		{"default_only", "class Default {};\n#define PARAM Default", "template<class T = PARAM> class Box {};", "template <class T>", false},
		{"hidden_alias", "using Index = unsigned;\n#define PARAM template<Index N> class", "template<PARAM Fixture, class T> class Box {};", "#pragma once", true},
		{"hidden_concept", "template<class T> concept Good = sizeof(T)>1;\n#define PARAM template<Good T> class", "template<PARAM Fixture, class T> class Box {};", "#pragma once", true},
	} {
		for _, separate := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/separate_header=%t", test.name, separate), func(t *testing.T) {
				project := t.TempDir()
				definitions := test.definitions + "\n"
				headers := []string{filepath.Join(project, "types.h")}
				if separate {
					writeBuildFile(t, project, "macros.h", "#pragma once\n"+definitions)
					definitions = "#include \"macros.h\"\n"
					headers = append(headers, filepath.Join(project, "macros.h"))
				}
				writeBuildFile(t, project, "types.h", "#pragma once\n"+definitions+test.declaration+"\n")
				source := filepath.Join(project, "main.cpp")
				writeBuildFile(t, project, "main.cpp", "#include \"types.h\"\nint main() { return 0; }\n")
				before := clangParseCount()
				analysis, err := analyzeClangFile(source, nil, []string{"-std=c++20"}, clangAnalysisOptions{})
				if err != nil || clangAnalysisHasErrors(analysis) {
					t.Fatalf("analysis: %v; diagnostics: %+v", err, analysis.diagnostics)
				}
				declarations, skipped := selectForwardDeclarations(analysis, headers, project)
				contents := renderForwardDeclarations(declarations)
				if !strings.Contains(string(contents), test.want) || strings.Contains(string(contents), "PARAM") {
					t.Fatalf("missing expanded %q:\n%s\nskipped: %v", test.want, contents, skipped)
				}
				if test.skipped && (strings.Contains(string(contents), "class Box;") || !strings.Contains(strings.Join(skipped, "\n"), "Box")) {
					t.Fatalf("unsafe declaration retained or unreported:\n%s\nskipped: %v", contents, skipped)
				}
				if count := clangParseCount() - before; count != 1 {
					t.Fatalf("libclang parses = %d, want 1", count)
				}
				forward := filepath.Join(project, "main.fwd.h")
				if err := os.WriteFile(forward, contents, 0600); err != nil {
					t.Fatal(err)
				}
				command := exec.Command("c++", "-std=c++20", "-include", forward, "-fsyntax-only", source)
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("compiler: %v\n%s\nforward:\n%s", err, output, contents)
				}
			})
		}
	}
}

func TestBuildAnalysisUsesOneASTAndRestoresForward(t *testing.T) {
	project, root := t.TempDir(), t.TempDir()
	var header strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&header, "class Type%d {};\n", i)
	}
	writeBuildFile(t, project, "types.h", header.String())
	writeBuildFile(t, project, "main.cpp", "#include \"types.h\"\nint main() { return 0; }\n")
	var output bytes.Buffer
	progress := newProgressBar(&output, -1, true, false, true)
	source := filepath.Join(project, "main.cpp")
	inspect := func(read bool) buildResult {
		cache, err := newArtifactCache(read)
		if err != nil {
			t.Fatal(err)
		}
		manager := newLibraryManager(root, "host", "c++", 1, true, !read, project, nil, cache, progress, io.Discard)
		return inspectBuildSourceWithCache(root, "host", "", nil, []string{"-std=c++20"}, []string{"main"}, buildJob{source: source}, project, nil, cache, manager)
	}
	before := clangParseCount()
	first := inspect(true)
	if first.err != nil {
		t.Fatal(first.err)
	}
	if first.entrypoint != "main" || strings.Count(first.forward, "class Type") != 80 {
		t.Fatalf("analysis = %+v", first)
	}
	if progress.analyses.calls[source] != 1 || clangParseCount()-before != 1 {
		t.Fatalf("calls = %v\n%s", progress.analyses.calls, output.String())
	}
	forward, err := sourceForwardHeaderPath(root, "host", source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(forward); err != nil {
		t.Fatal(err)
	}
	second := inspect(true)
	if second.err != nil {
		t.Fatal(second.err)
	}
	restored, err := os.ReadFile(forward)
	if err != nil || string(restored) != first.forward || second.entrypoint != "main" {
		t.Fatalf("restore: %v %+v", err, second)
	}
	if progress.analyses.calls[source] != 1 || clangParseCount()-before != 1 {
		t.Fatalf("cache hit parsed again: %s", output.String())
	}
	third := inspect(false)
	if third.err != nil {
		t.Fatal(third.err)
	}
	if progress.analyses.calls[source] != 2 || clangParseCount()-before != 2 {
		t.Fatalf("no-cache calls = %v", progress.analyses.calls)
	}
}

func TestPreparedLibraryFlagsReachFirstAnalysis(t *testing.T) {
	project, root, installed := t.TempDir(), t.TempDir(), t.TempDir()
	writeBuildFile(t, installed, "library.h", "enum class Mode { Fast }; template<Mode M> class Parser {};\n")
	header := filepath.Join(project, "library.hard.h")
	writeBuildFile(t, project, "library.hard", validLibraryRecipeYAML())
	writeBuildFile(t, project, "library.hard.h", "#include <library.h>\n")
	source := filepath.Join(project, "main.cpp")
	writeBuildFile(t, project, "main.cpp", "#include \"library.hard.h\"\nint main(){return 0;}\n")
	var output bytes.Buffer
	inspect := func() (buildResult, int) {
		progress := newProgressBar(&output, -1, true, false, true)
		cache := newTestArtifactCache(t, true)
		manager := newLibraryManager(root, "host", "c++", 1, true, false, project, nil, cache, progress, io.Discard)
		encoded, _ := json.Marshal([]string{validLibraryRecipeYAML(), ""})
		key := sha256.Sum256(append([]byte(filepath.Join(project, "library.hard")+"\x00"), encoded...))
		manager.results[hex.EncodeToString(key[:])] = libraryArtifact{key: header, header: header, cflags: []string{"-I" + installed}}
		before := clangParseCount()
		activity := func(path string, cached bool) {
			progress.updateStep("Parsing " + progress.path(path))
		}
		result := inspectBuildSourceWithCache(root, "host", "", nil, []string{"-std=c++20"}, []string{"main"}, buildJob{source: source}, project, activity, cache, manager)
		return result, int(clangParseCount() - before)
	}
	if result, count := inspect(); result.err != nil || count != 2 {
		t.Fatalf("initial: %v, calls %d\n%s", result.err, count, output.String())
	}
	want := "[1/?] Parsing main.cpp\n[1/?] Parsing main.cpp (library includes updated)\n[1/?] Generating main.cpp.fwd.h\n"
	if output.String() != want {
		t.Fatalf("initial progress = %q, want %q", output.String(), want)
	}
	output.Reset()
	writeBuildFile(t, project, "main.cpp", "#include \"library.hard.h\"\nint main(){return 1;}\n")
	if result, count := inspect(); result.err != nil || count != 1 || !strings.Contains(result.forward, "class Parser;") {
		t.Fatalf("prepared: %+v, calls %d\n%s", result, count, output.String())
	}
	want = "[1/?] Parsing main.cpp\n[1/?] Generating main.cpp.fwd.h\n"
	if output.String() != want {
		t.Fatalf("prepared progress = %q, want %q", output.String(), want)
	}
	writeBuildFile(t, project, "main.cpp", "int main(){return 0;}\n")
	if result, count := inspect(); result.err != nil || count != 2 || len(result.libraries) != 0 || len(result.cflags) != 1 {
		t.Fatalf("removed: %+v, calls %d\n%s", result, count, output.String())
	}
}

func TestColdTransitiveLibrariesAnalyzeThreeTimes(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	leaf, leafArchive := inheritedTestSnapshot(t, "github.com/demo/libB2", "main", firstCommit, map[string]string{"b.h": "#pragma once\nnamespace B { enum class Mode { Fast }; }\n"})
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/libA1", "main", firstCommit, map[string]string{"a.h": "#pragma once\n#include <github.com/demo/libB2/b.h>\nnamespace A { template<B::Mode M> class Parser {}; }\n"})
	proxy.add(leaf, leafArchive)
	proxy.add(parent, parentArchive)
	configuration := projectTestConfiguration(t)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "hard.yaml", "version: 1\nrepositories: {}\n")
	writeProjectTestFile(t, project, "main.cpp", "#include <github.com/demo/libA1/a.h>\nint main(){return 0;}\n")
	before := clangParseCount()
	out := runDiscoveryCommand(t, configuration, "build", "-v", "--no-color")
	if clangParseCount()-before != 3 {
		t.Fatalf("actual libclang calls: %d", clangParseCount()-before)
	}
	if strings.Count(out, "Parsing main.cpp") != 3 || strings.Count(out, "Parsing main.cpp (dependencies updated)") != 2 || strings.Count(out, "Generating main.cpp.fwd.h") != 1 {
		t.Fatalf("cold analysis progress:\n%s", out)
	}
	out = runDiscoveryCommand(t, configuration, "build", "-v", "--no-color")
	if clangParseCount()-before != 3 {
		t.Fatal("cache hit called libclang")
	}
	if strings.Contains(out, "Generating ") || !strings.Contains(out, "Parsing main.cpp (CACHED)") {
		t.Fatalf("warm analysis:\n%s", out)
	}
}
