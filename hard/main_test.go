package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveRuntimeRootFollowsExecutableSymlink(t *testing.T) {
	runtimeRoot := t.TempDir()
	executable := filepath.Join(runtimeRoot, "hard")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	binDirectory := t.TempDir()
	link := filepath.Join(binDirectory, "hard")
	if err := os.Symlink(executable, link); err != nil {
		t.Skipf("cannot create executable symlink: %v", err)
	}

	got, err := resolveRuntimeRoot(link)
	if err != nil {
		t.Fatalf("resolveRuntimeRoot() error = %v", err)
	}
	if got != runtimeRoot {
		t.Fatalf("resolveRuntimeRoot() = %q, want %q", got, runtimeRoot)
	}
}

func TestResolveRuntimeRootRejectsMissingExecutable(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "missing-hard")
	_, err := resolveRuntimeRoot(executable)
	if err == nil {
		t.Fatal("resolveRuntimeRoot() error = nil")
	}
	if !strings.Contains(err.Error(), executable) {
		t.Fatalf("resolveRuntimeRoot() error = %q, want executable path", err)
	}
}

func TestDiscoverSourcesReportsSearchActivity(t *testing.T) {
	project := t.TempDir()
	writeBuildFile(t, project, "source.cpp", "")
	writeBuildFile(t, project, "source_test.cpp", "")
	writeBuildFile(t, project, "header.h", "")
	withWorkingDirectory(t, project)

	commands := []struct {
		name    string
		sources []string
	}{
		{name: "build", sources: []string{"source.cpp"}},
		{name: "fetch", sources: []string{"source.cpp", "source_test.cpp"}},
		{name: "run", sources: []string{"source.cpp"}},
		{name: "format", sources: []string{"header.h", "source.cpp", "source_test.cpp"}},
		{name: "test", sources: []string{"source_test.cpp"}},
	}
	modes := []struct {
		name    string
		verbose bool
		silent  bool
		want    string
	}{
		{name: "normal", want: "\r[1/?] Searching source files\n"},
		{name: "verbose", verbose: true, want: "[1/?] Searching source files\n"},
		{name: "silent", verbose: true, silent: true},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			for _, mode := range modes {
				t.Run(mode.name, func(t *testing.T) {
					var output bytes.Buffer
					progress := newProgressBar(&output, -1, mode.verbose, mode.silent, true)
					sources, err := discoverSourcesWithProgress(command.name, nil, progress)
					if err != nil {
						t.Fatalf("discoverSourcesWithProgress() error = %v", err)
					}
					if !reflect.DeepEqual(sources, command.sources) {
						t.Fatalf("discoverSourcesWithProgress() = %#v, want %#v", sources, command.sources)
					}
					if err := progress.finish(); err != nil {
						t.Fatalf("progress.finish() error = %v", err)
					}
					if got := output.String(); got != mode.want {
						t.Fatalf("search progress = %q, want %q", got, mode.want)
					}
				})
			}
		})
	}
}
