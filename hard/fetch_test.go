package main

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFetchSourcesWithEmptySelectionDoesNothing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := fetchSources(
		t.TempDir(),
		nil,
		nil,
		0,
		true,
		false,
		true,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("fetchSources() error = %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("fetchSources() output = stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestFetchSourcesContinuesSearchProgressThroughParsing(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "source.cpp", "")
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	progress := newProgressBar(&stdout, -1, true, false, true)
	progress.updateStep("Searching source files")
	if err := fetchSourcesWithProgress(
		root,
		nil,
		[]string{"source.cpp"},
		1,
		progress,
		io.Discard,
	); err != nil {
		t.Fatalf("fetchSourcesWithProgress() error = %v", err)
	}

	want := strings.Join([]string{
		"[1/?] Searching source files",
		"[1/?] Parsing source.cpp",
		"",
	}, "\n")
	if got := stdout.String(); got != want {
		t.Fatalf("fetch progress = %q, want %q", got, want)
	}
}

func TestFetchSourceDependenciesDownloadsWithoutCompiling(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(
		t,
		project,
		"source.cpp",
		"#include <github.com/owner/first/include/first.hpp>\n",
	)
	firstHeader := filepath.Join(
		root,
		"source",
		"github.com",
		"owner",
		"first",
		"include",
		"first.hpp",
	)
	secondHeader := filepath.Join(
		root,
		"source",
		"github.com",
		"owner",
		"second",
		"include",
		"second.hpp",
	)
	archives := map[string][]byte{
		"/repos/owner/first/tarball": githubTestArchive(t, []githubTestArchiveEntry{
			{name: "owner-first-sha/", typeflag: tar.TypeDir, mode: 0o755},
			{
				name:     "owner-first-sha/include/first.hpp",
				typeflag: tar.TypeReg,
				contents: "#include <github.com/owner/second/include/second.hpp>\n",
				mode:     0o644,
			},
		}),
		"/repos/owner/second/tarball": githubTestArchive(t, []githubTestArchiveEntry{
			{name: "owner-second-sha/", typeflag: tar.TypeDir, mode: 0o755},
			{
				name:     "owner-second-sha/include/second.hpp",
				typeflag: tar.TypeReg,
				contents: "#pragma once\n",
				mode:     0o644,
			},
		}),
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		archive, ok := archives[request.URL.Path]
		if !ok {
			http.NotFound(response, request)
			return
		}
		requests.Add(1)
		_, _ = response.Write(archive)
	}))
	defer server.Close()

	var output bytes.Buffer
	progress := newProgressBar(&output, -1, true, false, true)
	progress.updateStep("Searching source files")
	resolver := newGitHubSnapshotResolverWithClient(
		root,
		server.Client(),
		server.URL,
		progress,
	)
	var diagnostics bytes.Buffer
	activity := func(path string) {
		progress.updateStep("Parsing " + buildParsingDisplayPath(root, path, project))
	}
	err := fetchSourceDependenciesWithActivity(
		resolver,
		[]string{"-I" + filepath.Join(root, "source")},
		[]string{"source.cpp"},
		2,
		project,
		&diagnostics,
		activity,
	)
	if err != nil {
		t.Fatalf("fetchSourceDependencies() error = %v", err)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("fetchSourceDependencies() diagnostics = %q", diagnostics.String())
	}
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}
	wantOutput := strings.Join([]string{
		"[1/?] Searching source files",
		"[1/?] Parsing source.cpp",
		"[1/?] Downloading github.com/owner/first",
		"[1/?] Downloading github.com/owner/second",
		"",
	}, "\n")
	if got := output.String(); got != wantOutput {
		t.Fatalf("fetch progress = %q, want %q", got, wantOutput)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}
	if got := readTestFile(t, firstHeader); !strings.Contains(got, "github.com/owner/second") {
		t.Fatalf("first downloaded header = %q", got)
	}
	if got := readTestFile(t, secondHeader); got != "#pragma once\n" {
		t.Fatalf("second downloaded header = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "env")); !os.IsNotExist(err) {
		t.Fatalf("fetch created environment build artifacts: %v", err)
	}
	var cachedOutput bytes.Buffer
	cachedProgress := newProgressBar(&cachedOutput, -1, true, false, true)
	cachedProgress.updateStep("Searching source files")
	cachedResolver := newGitHubSnapshotResolverWithClient(
		root,
		server.Client(),
		server.URL,
		cachedProgress,
	)
	cachedActivity := func(path string) {
		cachedProgress.updateStep("Parsing " + buildParsingDisplayPath(root, path, project))
	}
	err = fetchSourceDependenciesWithActivity(
		cachedResolver,
		[]string{"-I" + filepath.Join(root, "source")},
		[]string{"source.cpp"},
		2,
		project,
		&diagnostics,
		cachedActivity,
	)
	if err != nil {
		t.Fatalf("cached fetchSourceDependencies() error = %v", err)
	}
	if got := cachedResolver.downloadProgressEntries(); len(got) != 0 {
		t.Fatalf("cached fetch progress entries = %#v, want empty", got)
	}
	if err := cachedProgress.finish(); err != nil {
		t.Fatalf("cached progress.finish() error = %v", err)
	}
	wantCachedOutput := strings.Join([]string{
		"[1/?] Searching source files",
		"[1/?] Parsing source.cpp",
		"",
	}, "\n")
	if got := cachedOutput.String(); got != wantCachedOutput {
		t.Fatalf("cached fetch progress = %q, want %q", got, wantCachedOutput)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("cached request count = %d, want 2", got)
	}
}

func TestFetchSourceDependenciesRejectsInvalidJobs(t *testing.T) {
	err := fetchSourceDependencies(nil, nil, []string{"source.cpp"}, 0, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "jobs must be positive") {
		t.Fatalf("fetchSourceDependencies() error = %v", err)
	}
}
