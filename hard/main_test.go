package main

import (
	"bytes"
	"reflect"
	"testing"
)

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
