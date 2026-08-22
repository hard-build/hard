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

func TestSourceForwardHeaderPathMirrorsAbsoluteSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "home", "user", "project", "src", "file.cpp")

	got, err := sourceForwardHeaderPath(root, "target", source)
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath() error = %v", err)
	}
	want := filepath.Join(root, "env", "target", "build", "home", "user", "project", "src", "file.cpp.fwd.h")
	if got != want {
		t.Fatalf("sourceForwardHeaderPath() = %q, want %q", got, want)
	}
}

func TestSourceForwardHeaderPathPreservesSourceExtension(t *testing.T) {
	root := t.TempDir()
	for _, input := range []string{"file.c", "file.cc", "file.cpp", "file.c++"} {
		source := filepath.Join(string(filepath.Separator), "project", input)
		got, err := sourceForwardHeaderPath(root, "host", source)
		if err != nil {
			t.Fatalf("sourceForwardHeaderPath(%q) error = %v", input, err)
		}
		want := filepath.Join(root, "env", "host", "build", "project", input+".fwd.h")
		if got != want {
			t.Errorf("sourceForwardHeaderPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSourceForwardHeaderPathRejectsEnvironmentEscape(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(string(filepath.Separator), "home", "user", "file.cpp")

	_, err := sourceForwardHeaderPath(root, "../outside", source)
	if err == nil {
		t.Fatal("sourceForwardHeaderPath() error = nil")
	}
	if !strings.Contains(err.Error(), "HARD_ENV escapes environment directory") {
		t.Fatalf("sourceForwardHeaderPath() error = %q", err)
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

func TestSourceForwardContentsUsesActiveNonSystemDependencies(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	writeBuildFile(t, project, "dependency.h", "struct dependency {};\n")
	writeBuildFile(t, project, "include/file.h", "#include \"../dependency.h\"\nnamespace project { class direct {}; }\n")
	writeBuildFile(
		t,
		project,
		"source.cpp",
		"#include <vector>\n#include \"include/file.h\"\nclass source_type {};\n",
	)
	analysis, err := analyzeClangDependencies("source.cpp", project, []string{"-std=c++20"})
	if err != nil {
		t.Fatalf("analyzeClangDependencies() error = %v", err)
	}
	dependencies, err := clangDependencyPaths(analysis, "source.cpp", project)
	if err != nil {
		t.Fatalf("clangDependencyPaths() error = %v", err)
	}
	output, err := sourceForwardHeaderPath(root, "host", filepath.Join(project, "source.cpp"))
	if err != nil {
		t.Fatalf("sourceForwardHeaderPath() error = %v", err)
	}
	contents, err := sourceForwardContents(output, analysis, dependencies, nil, project)
	if err != nil {
		t.Fatalf("sourceForwardContents() error = %v", err)
	}
	want := "#pragma once\n\nstruct dependency;\n\nnamespace project\n{\nclass direct;\n}\n"
	if got := string(contents); got != want {
		t.Fatalf("source forward contents = %q, want %q", got, want)
	}
	for _, forbidden := range []string{"source_type", "vector"} {
		if strings.Contains(string(contents), forbidden) {
			t.Fatalf("source forward contents contain %q: %q", forbidden, contents)
		}
	}
}

func TestSourceForwardContentsUsesTranslationUnitContext(t *testing.T) {
	project := t.TempDir()
	root := t.TempDir()
	writeBuildFile(
		t,
		project,
		"conditional.h",
		"#ifdef FIRST\nclass first_only {};\n#else\nclass second_only {};\n#endif\n",
	)
	writeBuildFile(t, project, "first.cpp", "#include \"conditional.h\"\n")
	writeBuildFile(t, project, "second.cpp", "#include \"conditional.h\"\n")

	tests := []struct {
		source    string
		cflags    []string
		want      string
		forbidden string
	}{
		{source: "first.cpp", cflags: []string{"-DFIRST"}, want: "class first_only;", forbidden: "second_only"},
		{source: "second.cpp", want: "class second_only;", forbidden: "first_only"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			analysis, err := analyzeClangDependencies(tt.source, project, tt.cflags)
			if err != nil {
				t.Fatalf("analyzeClangDependencies() error = %v", err)
			}
			dependencies, err := clangDependencyPaths(analysis, tt.source, project)
			if err != nil {
				t.Fatalf("clangDependencyPaths() error = %v", err)
			}
			output, err := sourceForwardHeaderPath(root, "host", filepath.Join(project, tt.source))
			if err != nil {
				t.Fatalf("sourceForwardHeaderPath() error = %v", err)
			}
			contents, err := sourceForwardContents(output, analysis, dependencies, tt.cflags, project)
			if err != nil {
				t.Fatalf("sourceForwardContents() error = %v", err)
			}
			if !strings.Contains(string(contents), tt.want) || strings.Contains(string(contents), tt.forbidden) {
				t.Fatalf("source forward contents = %q, want %q without %q", contents, tt.want, tt.forbidden)
			}
		})
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

func TestWriteForwardHeaderPreservesUnchangedRegularFile(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "output_fwd.h")
	contents := []byte("#pragma once\n")
	if err := writeForwardHeader(output, contents); err != nil {
		t.Fatalf("first writeForwardHeader() error = %v", err)
	}
	before, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat first forward header: %v", err)
	}

	if err := writeForwardHeader(output, contents); err != nil {
		t.Fatalf("second writeForwardHeader() error = %v", err)
	}
	after, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat second forward header: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("unchanged forward header was replaced")
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
