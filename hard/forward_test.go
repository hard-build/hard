package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtractAndRenderForwardDeclarations(t *testing.T) {
	source := []byte(`#include "dependency.h"

class global;
struct data {};

namespace example {
class object {};
struct options;

namespace detail {
class helper;
}

void function() {
    class local;
}

class outer {
    class nested;
    struct nested_data {};
};
}

namespace {
class internal;
}

template <typename type = int, class allocator>
class collection {};

class global {};
`)

	declarations, err := extractForwardDeclarations(source)
	if err != nil {
		t.Fatalf("extractForwardDeclarations() error = %v", err)
	}
	got := string(renderForwardDeclarations(declarations))
	want := `#pragma once

class global;
struct data;
template <typename type, class allocator>
class collection;

namespace example
{
class object;
struct options;
class outer;
}

namespace example
{
namespace detail
{
class helper;
}
}
`
	if got != want {
		t.Fatalf("renderForwardDeclarations() =\n%s\nwant:\n%s", got, want)
	}
	for _, forbidden := range []string{"dependency", "local", "nested", "internal"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("forward declarations contain %q:\n%s", forbidden, got)
		}
	}
}

func TestExtractForwardDeclarationsRejectsInvalidSyntax(t *testing.T) {
	_, err := extractForwardDeclarations([]byte("class broken {\n"))
	if err == nil {
		t.Fatal("extractForwardDeclarations() error = nil")
	}
	if !strings.Contains(err.Error(), "invalid C++ syntax at") {
		t.Fatalf("extractForwardDeclarations() error = %q", err)
	}
}

func TestForwardHeaderPathMirrorsAbsoluteHeader(t *testing.T) {
	root := t.TempDir()
	header := filepath.Join(string(filepath.Separator), "home", "user", "project", "include", "file.hpp")

	got, err := forwardHeaderPath(root, "target", header)
	if err != nil {
		t.Fatalf("forwardHeaderPath() error = %v", err)
	}
	want := filepath.Join(root, "env", "target", "build", "home", "user", "project", "include", "file_fwd.hpp")
	if got != want {
		t.Fatalf("forwardHeaderPath() = %q, want %q", got, want)
	}
}

func TestForwardHeaderPathPreservesExtension(t *testing.T) {
	root := t.TempDir()
	tests := map[string]string{
		"file.h":   "file_fwd.h",
		"file.hh":  "file_fwd.hh",
		"file.hpp": "file_fwd.hpp",
		"file.h++": "file_fwd.h++",
		"file":     "file_fwd",
	}
	for input, wantName := range tests {
		header := filepath.Join(string(filepath.Separator), "project", input)
		got, err := forwardHeaderPath(root, "host", header)
		if err != nil {
			t.Fatalf("forwardHeaderPath(%q) error = %v", input, err)
		}
		want := filepath.Join(root, "env", "host", "build", "project", wantName)
		if got != want {
			t.Errorf("forwardHeaderPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestForwardHeaderPathRejectsEnvironmentEscape(t *testing.T) {
	root := t.TempDir()
	header := filepath.Join(string(filepath.Separator), "home", "user", "file.h")

	_, err := forwardHeaderPath(root, "../outside", header)
	if err == nil {
		t.Fatal("forwardHeaderPath() error = nil")
	}
	if !strings.Contains(err.Error(), "HARD_ENV escapes environment directory") {
		t.Fatalf("forwardHeaderPath() error = %q", err)
	}
}

func TestRenderForwardDeclarationsSupportsInlineAndNestedNamespaces(t *testing.T) {
	source := []byte("inline namespace version { class api; }\nnamespace project::detail { struct data; }\n")
	declarations, err := extractForwardDeclarations(source)
	if err != nil {
		t.Fatalf("extractForwardDeclarations() error = %v", err)
	}
	want := "#pragma once\n\ninline namespace version\n{\nclass api;\n}\n\nnamespace project\n{\nnamespace detail\n{\nstruct data;\n}\n}\n"
	if got := string(renderForwardDeclarations(declarations)); got != want {
		t.Fatalf("renderForwardDeclarations() = %q, want %q", got, want)
	}
}

func TestGenerateForwardDeclarationsUsesOnlyEachHeaderContents(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	dependency := filepath.Join(project, "dependency.h")
	header := filepath.Join(project, "include", "file.h")
	writeBuildFile(t, project, "dependency.h", "struct dependency {};\n")
	writeBuildFile(t, project, "include/file.h", "#include \"../dependency.h\"\nnamespace project { class direct {}; }\n")

	if err := generateForwardDeclarations(root, "host", []string{header, dependency, header}, 2); err != nil {
		t.Fatalf("generateForwardDeclarations() error = %v", err)
	}

	headerOutput, err := forwardHeaderPath(root, "host", header)
	if err != nil {
		t.Fatalf("forwardHeaderPath(header) error = %v", err)
	}
	dependencyOutput, err := forwardHeaderPath(root, "host", dependency)
	if err != nil {
		t.Fatalf("forwardHeaderPath(dependency) error = %v", err)
	}
	if got := readTestFile(t, headerOutput); got != "#pragma once\n\nnamespace project\n{\nclass direct;\n}\n" {
		t.Fatalf("generated header = %q", got)
	}
	if got := readTestFile(t, dependencyOutput); got != "#pragma once\n\nstruct dependency;\n" {
		t.Fatalf("generated dependency = %q", got)
	}
}

func TestGenerateForwardDeclarationsPreservesHeaderExtensions(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	first := filepath.Join(project, "file.h")
	second := filepath.Join(project, "file.hpp")
	writeBuildFile(t, project, "file.h", "class first;\n")
	writeBuildFile(t, project, "file.hpp", "class second;\n")

	if err := generateForwardDeclarations(root, "host", []string{first, second}, 2); err != nil {
		t.Fatalf("generateForwardDeclarations() error = %v", err)
	}
	firstOutput, err := forwardHeaderPath(root, "host", first)
	if err != nil {
		t.Fatalf("forwardHeaderPath(first) error = %v", err)
	}
	secondOutput, err := forwardHeaderPath(root, "host", second)
	if err != nil {
		t.Fatalf("forwardHeaderPath(second) error = %v", err)
	}
	if filepath.Base(firstOutput) != "file_fwd.h" {
		t.Fatalf("first output = %q", firstOutput)
	}
	if filepath.Base(secondOutput) != "file_fwd.hpp" {
		t.Fatalf("second output = %q", secondOutput)
	}
	if got := readTestFile(t, firstOutput); got != "#pragma once\n\nclass first;\n" {
		t.Fatalf("generated .h forward header = %q", got)
	}
	if got := readTestFile(t, secondOutput); got != "#pragma once\n\nclass second;\n" {
		t.Fatalf("generated .hpp forward header = %q", got)
	}
}

func TestGenerateForwardDeclarationsReportsHeaderBeforeParsing(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	header := filepath.Join(project, "broken.h")
	writeBuildFile(t, project, "broken.h", "class broken {\n")

	var activities []string
	err := generateForwardDeclarationsWithFlagsAndActivity(
		root,
		"host",
		[]string{header},
		nil,
		1,
		project,
		func(path string) {
			activities = append(activities, path)
		},
	)
	if err == nil {
		t.Fatal("generateForwardDeclarationsWithFlagsAndActivity() error = nil")
	}
	want := []string{header}
	if !reflect.DeepEqual(activities, want) {
		t.Fatalf("forward parsing activities = %#v, want %#v", activities, want)
	}
}

func TestGenerateForwardHeaderPreservesExistingFileOnParseError(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	header := filepath.Join(project, "broken.h")
	writeBuildFile(t, project, "broken.h", "class broken {\n")
	output, err := forwardHeaderPath(root, "host", header)
	if err != nil {
		t.Fatalf("forwardHeaderPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatalf("create output directory: %v", err)
	}
	if err := os.WriteFile(output, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("write existing output: %v", err)
	}

	err = generateForwardHeader(header, output)
	if err == nil {
		t.Fatal("generateForwardHeader() error = nil")
	}
	if got := readTestFile(t, output); got != "existing\n" {
		t.Fatalf("existing output = %q", got)
	}
}

func TestWriteForwardHeaderRemovesTemporaryFileWhenRenameFails(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "output_fwd.h")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatalf("create conflicting output directory: %v", err)
	}

	err := writeForwardHeader(output, []byte("#pragma once\n"))
	if err == nil {
		t.Fatal("writeForwardHeader() error = nil")
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil {
		t.Fatalf("read output directory: %v", readErr)
	}
	if len(entries) != 1 || entries[0].Name() != "output_fwd.h" {
		t.Fatalf("output directory entries = %#v", entries)
	}
}

func TestStripTemplateDefault(t *testing.T) {
	tests := map[string]string{
		"typename type = int":                                 "typename type",
		"class allocator":                                     "class allocator",
		"int size = value<left == right>":                     "int size",
		"template<class item = int> class container = vector": "template<class item = int> class container",
	}
	for input, want := range tests {
		if got := stripTemplateDefault(input); got != want {
			t.Errorf("stripTemplateDefault(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAppendForwardNamespaceDoesNotModifyInput(t *testing.T) {
	input := []forwardNamespace{{name: "outer"}}
	got := appendForwardNamespace(input, forwardNamespace{name: "inner"})
	want := []forwardNamespace{{name: "outer"}, {name: "inner"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendForwardNamespace() = %#v, want %#v", got, want)
	}
	if len(input) != 1 {
		t.Fatalf("input = %#v", input)
	}
}
