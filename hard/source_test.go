package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMatchesSource(t *testing.T) {
	tests := []struct {
		name    string
		command string
		path    string
		want    bool
		wantErr bool
	}{
		{name: "build c", command: "build", path: "source.c", want: true},
		{name: "build cc", command: "build", path: "source.cc", want: true},
		{name: "build cpp", command: "build", path: "source.cpp", want: true},
		{name: "build c++", command: "build", path: "source.c++", want: true},
		{name: "build uppercase extension", command: "build", path: "source.CPP", want: true},
		{name: "build excludes test", command: "build", path: "source_test.cpp"},
		{name: "build excludes uppercase test", command: "build", path: "source_TEST.CPP"},
		{name: "build excludes header", command: "build", path: "source.hpp"},
		{name: "build excludes cxx", command: "build", path: "source.cxx"},
		{name: "fetch source", command: "fetch", path: "source.cpp", want: true},
		{name: "fetch test source", command: "fetch", path: "source_TEST.CPP", want: true},
		{name: "fetch excludes header", command: "fetch", path: "source.hpp"},
		{name: "fetch excludes cxx", command: "fetch", path: "source.cxx"},
		{name: "format source", command: "format", path: "source.cpp", want: true},
		{name: "format test source", command: "format", path: "source_TEST.CPP", want: true},
		{name: "format h", command: "format", path: "source.h", want: true},
		{name: "format hh", command: "format", path: "source.hh", want: true},
		{name: "format hpp", command: "format", path: "source.hpp", want: true},
		{name: "format h++", command: "format", path: "source.h++", want: true},
		{name: "format uppercase header", command: "format", path: "source.HPP", want: true},
		{name: "format excludes hxx", command: "format", path: "source.hxx"},
		{name: "test lowercase suffix", command: "test", path: "source_test.cpp", want: true},
		{name: "test uppercase suffix", command: "test", path: "source_TEST.cpp", want: true},
		{name: "test mixed-case suffix and extension", command: "test", path: "source_TeSt.CPP", want: true},
		{name: "test excludes source", command: "test", path: "source.cpp"},
		{name: "test excludes header", command: "test", path: "source_test.hpp"},
		{name: "test excludes cxx", command: "test", path: "source_test.cxx"},
		{name: "unknown command", command: "unknown", path: "source.cpp", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchesSource(tt.command, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("matchesSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("matchesSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiscoverSourcesRecursively(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root,
		"01.c",
		"02.cc",
		"03.cpp",
		"04.c++",
		"05_test.cpp",
		"06_TEST.CC",
		"07.h",
		"08.hh",
		"09.hpp",
		"10.h++",
		"11.cxx",
		"12.hxx",
		"nested/13.cpp",
		"nested/14_TeSt.c++",
		"nested/15.hpp",
		"nested/16.txt",
	)

	tests := []struct {
		command string
		want    []string
	}{
		{
			command: "build",
			want: []string{
				"01.c", "02.cc", "03.cpp", "04.c++", "nested/13.cpp",
			},
		},
		{
			command: "fetch",
			want: []string{
				"01.c", "02.cc", "03.cpp", "04.c++", "05_test.cpp",
				"06_TEST.CC", "nested/13.cpp", "nested/14_TeSt.c++",
			},
		},
		{
			command: "format",
			want: []string{
				"01.c", "02.cc", "03.cpp", "04.c++", "05_test.cpp",
				"06_TEST.CC", "07.h", "08.hh", "09.hpp", "10.h++",
				"nested/13.cpp", "nested/14_TeSt.c++", "nested/15.hpp",
			},
		},
		{
			command: "test",
			want:    []string{"05_test.cpp", "06_TEST.CC", "nested/14_TeSt.c++"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got, err := discoverSourcesFrom(tt.command, nil, root)
			if err != nil {
				t.Fatalf("discoverSourcesFrom() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDiscoverSourcesUsesOnlyExplicitPaths(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root,
		"selected.cpp",
		"ignored.cpp",
		"group/01.cpp",
		"group/02.h",
		"group/03_test.cpp",
		"outside/not-selected.cpp",
		"note.txt",
	)

	got, err := discoverSourcesFrom(
		"format",
		[]string{"selected.cpp", "group", "note.txt"},
		root,
	)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{
		"selected.cpp",
		"group/01.cpp",
		"group/02.h",
		"group/03_test.cpp",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesPrintsAbsoluteInputAsRelativePath(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "src/source.cpp")

	input := filepath.Join(root, "src", "source.cpp")
	got, err := discoverSourcesFrom("build", []string{input}, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"src/source.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesRemovesDuplicatesByRealPath(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "real.cpp")
	if err := os.Symlink("real.cpp", filepath.Join(root, "alias.cpp")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := discoverSourcesFrom(
		"build",
		[]string{"alias.cpp", "real.cpp", filepath.Join(root, "real.cpp")},
		root,
	)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"alias.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesFollowsExplicitDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "target/nested/source.cpp")
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := discoverSourcesFrom("build", []string{"link"}, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"link/nested/source.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesFollowsNestedDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "external/source.cpp")
	if err := os.Mkdir(filepath.Join(root, "selected"), 0o755); err != nil {
		t.Fatalf("create selected directory: %v", err)
	}
	if err := os.Symlink("../external", filepath.Join(root, "selected", "dependency")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := discoverSourcesFrom("build", []string{"selected"}, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"selected/dependency/source.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesStopsDirectorySymlinkCycle(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "tree/nested/source.cpp")
	if err := os.Symlink("..", filepath.Join(root, "tree", "nested", "back")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := discoverSourcesFrom("build", []string{"tree"}, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"tree/nested/source.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesScansRealDirectoryOnce(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "target/source.cpp")
	if err := os.Symlink("target", filepath.Join(root, "alias")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := discoverSourcesFrom("build", []string{"alias", "target"}, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	want := []string{"alias/source.cpp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverSourcesFrom() = %#v, want %#v", got, want)
	}
}

func TestDiscoverSourcesRejectsMissingPath(t *testing.T) {
	root := t.TempDir()
	_, err := discoverSourcesFrom("build", []string{"missing"}, root)
	if err == nil {
		t.Fatal("discoverSourcesFrom() error = nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("discoverSourcesFrom() error = %q, want it to contain missing", err)
	}
}

func TestDiscoverSourcesReturnsEmptyResult(t *testing.T) {
	root := t.TempDir()
	writeSourceFiles(t, root, "README.md", "source.cxx", "source_test.hpp")

	got, err := discoverSourcesFrom("build", nil, root)
	if err != nil {
		t.Fatalf("discoverSourcesFrom() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("discoverSourcesFrom() = %#v, want no sources", got)
	}
}

func writeSourceFiles(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		path = filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
