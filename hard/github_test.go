package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type githubTestArchiveEntry struct {
	name     string
	typeflag byte
	linkname string
	contents string
	mode     int64
	pax      map[string]string
}

type githubProgressBuffer struct {
	mutex  sync.Mutex
	output bytes.Buffer
}

func (buffer *githubProgressBuffer) Write(contents []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.output.Write(contents)
}

func (buffer *githubProgressBuffer) String() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.output.String()
}

func TestGitHubRepositoriesFromDependencies(t *testing.T) {
	got := githubRepositoriesFromDependencies([]string{
		"source.cpp",
		"github.com/nlohmann/json/single_include/nlohmann/json.hpp",
		"github.com/google/googletest/googletest/include/gtest/gtest.h",
		"github.com/nlohmann/json/include/nlohmann/json.hpp",
		"hard/application/application.h",
		"hard/variable/variable.h",
		"/tmp/github.com/ignored/repository/header.h",
		"/tmp/hard/ignored.h",
		"github.com/incomplete",
		"github.com/../invalid/header.h",
		"hard/",
	})
	want := []githubRepository{
		{owner: "nlohmann", name: "json"},
		{owner: "google", name: "googletest"},
		{owner: "hard-build", name: "library"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("githubRepositoriesFromDependencies() = %#v, want %#v", got, want)
	}
}

func TestGitHubSnapshotResolverDownloadsOnceAndCaches(t *testing.T) {
	root := t.TempDir()
	archive := githubTestArchive(t, []githubTestArchiveEntry{
		{
			name:     "pax_global_header",
			typeflag: tar.TypeXGlobalHeader,
			pax:      map[string]string{"comment": "github.com"},
		},
		{name: "owner-repository-sha/", typeflag: tar.TypeDir, mode: 0o755},
		{
			name:     "owner-repository-sha/include/header.hpp",
			typeflag: tar.TypeReg,
			contents: "#pragma once\n",
			mode:     0o644,
		},
		{
			name:     "owner-repository-sha/current.hpp",
			typeflag: tar.TypeSymlink,
			linkname: "include/header.hpp",
			mode:     0o777,
		},
	})
	var requests atomic.Int32
	var progressOutput githubProgressBuffer
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if got, want := progressOutput.String(),
			"[1/1] Downloading github.com/owner/repository\n"; got != want {
			t.Errorf("progress before request = %q, want %q", got, want)
		}
		if request.URL.Path != "/repos/owner/repository/tarball" {
			t.Errorf("request path = %q", request.URL.Path)
		}
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept = %q", request.Header.Get("Accept"))
		}
		if request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
			t.Errorf("X-GitHub-Api-Version = %q", request.Header.Get("X-GitHub-Api-Version"))
		}
		if request.Header.Get("User-Agent") != "hard/1.0" {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(archive)
	}))
	defer server.Close()

	progress := newProgressBar(&progressOutput, 1, true, false, true)
	resolver := newGitHubSnapshotResolverWithClient(
		root,
		server.Client(),
		server.URL,
		progress,
	)
	repository := githubRepository{owner: "owner", name: "repository"}
	errors := make(chan error, 8)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errors <- resolver.ensure(repository)
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("ensure() error = %v", err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}

	directory, err := githubRepositoryDirectory(root, "owner", "repository")
	if err != nil {
		t.Fatalf("githubRepositoryDirectory() error = %v", err)
	}
	header := filepath.Join(directory, "include", "header.hpp")
	if got := readTestFile(t, header); got != "#pragma once\n" {
		t.Fatalf("downloaded header = %q", got)
	}
	link, err := os.Readlink(filepath.Join(directory, "current.hpp"))
	if err != nil {
		t.Fatalf("read downloaded symlink: %v", err)
	}
	if link != filepath.FromSlash("include/header.hpp") {
		t.Fatalf("downloaded symlink = %q", link)
	}
	if _, err := os.Lstat(filepath.Join(directory, ".git")); !os.IsNotExist(err) {
		t.Fatalf("downloaded snapshot contains .git: %v", err)
	}

	if err := resolver.ensure(repository); err != nil {
		t.Fatalf("cached ensure() error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("cached request count = %d, want 1", got)
	}
	if got, want := resolver.downloadProgressEntries(),
		[]string{"Downloading github.com/owner/repository"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("download progress entries = %#v, want %#v", got, want)
	}
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}
	if got, want := progressOutput.String(),
		"[1/1] Downloading github.com/owner/repository\n"; got != want {
		t.Fatalf("download progress = %q, want %q", got, want)
	}
}

func TestGitHubSnapshotResolverUsesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	directory, err := githubRepositoryDirectory(root, "owner", "repository")
	if err != nil {
		t.Fatalf("githubRepositoryDirectory() error = %v", err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create cached repository: %v", err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.Error(response, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	resolver := newGitHubSnapshotResolverWithClient(root, server.Client(), server.URL, nil)
	if err := resolver.ensure(githubRepository{owner: "owner", name: "repository"}); err != nil {
		t.Fatalf("ensure() error = %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
}

func TestGitHubSnapshotResolverCreatesWellKnownAlias(t *testing.T) {
	root := t.TempDir()
	archive := githubTestArchive(t, []githubTestArchiveEntry{
		{name: "hard-build-library-sha/", typeflag: tar.TypeDir, mode: 0o755},
		{
			name:     "hard-build-library-sha/application/application.h",
			typeflag: tar.TypeReg,
			contents: "#pragma once\n",
			mode:     0o644,
		},
	})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/repos/hard-build/library/tarball" {
			t.Errorf("request path = %q", request.URL.Path)
		}
		_, _ = response.Write(archive)
	}))
	defer server.Close()

	resolver := newGitHubSnapshotResolverWithClient(root, server.Client(), server.URL, nil)
	repository := githubRepository{owner: "hard-build", name: "library"}
	if err := resolver.ensure(repository); err != nil {
		t.Fatalf("ensure() error = %v", err)
	}
	destination, err := githubRepositoryDirectory(root, "hard-build", "library")
	if err != nil {
		t.Fatalf("githubRepositoryDirectory() error = %v", err)
	}
	alias := filepath.Join(root, "source", "hard")
	target, err := os.Readlink(alias)
	if err != nil {
		t.Fatalf("read well-known alias: %v", err)
	}
	if want := filepath.FromSlash("github.com/hard-build/library"); target != want {
		t.Fatalf("well-known alias = %q, want %q", target, want)
	}
	resolvedHeader, err := realAbsolutePath(
		filepath.Join(alias, "application", "application.h"),
		root,
	)
	if err != nil {
		t.Fatalf("resolve well-known header: %v", err)
	}
	wantHeader := filepath.Join(destination, "application", "application.h")
	if resolvedHeader != wantHeader {
		t.Fatalf("resolved well-known header = %q, want %q", resolvedHeader, wantHeader)
	}

	if err := resolver.ensure(repository); err != nil {
		t.Fatalf("cached ensure() error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
}

func TestGitHubSnapshotResolverRejectsConflictingWellKnownAlias(t *testing.T) {
	root := t.TempDir()
	destination, err := githubRepositoryDirectory(root, "hard-build", "library")
	if err != nil {
		t.Fatalf("githubRepositoryDirectory() error = %v", err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatalf("create cached repository: %v", err)
	}
	alias := filepath.Join(root, "source", "hard")
	if err := os.Mkdir(alias, 0o755); err != nil {
		t.Fatalf("create conflicting alias: %v", err)
	}

	resolver := newGitHubSnapshotResolverWithClient(
		root,
		http.DefaultClient,
		"http://unused",
		nil,
	)
	err = resolver.ensure(githubRepository{owner: "hard-build", name: "library"})
	if err == nil || !strings.Contains(err.Error(), "alias is not a symbolic link") {
		t.Fatalf("ensure() error = %v", err)
	}
}

func TestGitHubSnapshotResolverRejectsNonDirectoryCache(t *testing.T) {
	root := t.TempDir()
	directory, err := githubRepositoryDirectory(root, "owner", "repository")
	if err != nil {
		t.Fatalf("githubRepositoryDirectory() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0o755); err != nil {
		t.Fatalf("create cache parent: %v", err)
	}
	if err := os.WriteFile(directory, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("write invalid cache: %v", err)
	}

	resolver := newGitHubSnapshotResolverWithClient(
		root,
		http.DefaultClient,
		"http://unused",
		nil,
	)
	err = resolver.ensure(githubRepository{owner: "owner", name: "repository"})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("ensure() error = %v", err)
	}
}

func TestExtractGitHubSnapshotRejectsUnsafeEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []githubTestArchiveEntry
		want    string
	}{
		{
			name: "path traversal",
			entries: []githubTestArchiveEntry{
				{name: "repository-sha/", typeflag: tar.TypeDir, mode: 0o755},
				{
					name:     "repository-sha/../../outside",
					typeflag: tar.TypeReg,
					contents: "outside\n",
					mode:     0o644,
				},
			},
			want: "escapes root",
		},
		{
			name: "symlink traversal",
			entries: []githubTestArchiveEntry{
				{name: "repository-sha/", typeflag: tar.TypeDir, mode: 0o755},
				{
					name:     "repository-sha/link",
					typeflag: tar.TypeSymlink,
					linkname: "../outside",
					mode:     0o777,
				},
			},
			want: "symlink target escapes archive",
		},
		{
			name: "git metadata",
			entries: []githubTestArchiveEntry{
				{name: "repository-sha/", typeflag: tar.TypeDir, mode: 0o755},
				{name: "repository-sha/.git/", typeflag: tar.TypeDir, mode: 0o755},
				{
					name:     "repository-sha/.git/config",
					typeflag: tar.TypeReg,
					contents: "metadata\n",
					mode:     0o644,
				},
			},
			want: "forbidden .git path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination := t.TempDir()
			archive := githubTestArchive(t, tt.entries)
			err := extractGitHubSnapshot(bytes.NewReader(archive), destination)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("extractGitHubSnapshot() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSourceDependenciesDownloadsTransitiveGitHubSnapshots(t *testing.T) {
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

	resolver := newGitHubSnapshotResolverWithClient(root, server.Client(), server.URL, nil)
	var diagnostics bytes.Buffer
	fatal, got, err := sourceDependencies(
		resolver,
		[]string{"-I" + filepath.Join(root, "source")},
		"source.cpp",
		project,
		&diagnostics,
	)
	if err != nil {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	want := []string{firstHeader, secondHeader}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sourceDependencies() = %#v, want %#v", got, want)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("sourceDependencies() diagnostics = %q", diagnostics.String())
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("request count = %d, want 2", got)
	}

	if _, _, err := sourceDependencies(
		resolver,
		[]string{"-I" + filepath.Join(root, "source")},
		"source.cpp",
		project,
		io.Discard,
	); err != nil {
		t.Fatalf("cached sourceDependencies() error = %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("cached request count = %d, want 2", got)
	}
}

func TestSourceDependenciesPreservesNonGitHubFailure(t *testing.T) {
	project := t.TempDir()
	writeBuildFile(t, project, "source.cpp", "#include <missing/header.hpp>\n")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.Error(response, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

	resolver := newGitHubSnapshotResolverWithClient(
		t.TempDir(),
		server.Client(),
		server.URL,
		nil,
	)
	var diagnostics bytes.Buffer
	fatal, _, err := sourceDependencies(
		resolver,
		nil,
		"source.cpp",
		project,
		&diagnostics,
	)
	if err == nil || !strings.Contains(err.Error(), "unresolved include: missing/header.hpp") {
		t.Fatalf("sourceDependencies() error = %v", err)
	}
	if fatal {
		t.Fatal("sourceDependencies() fatal = true")
	}
	if got := diagnostics.String(); !strings.Contains(got, "'missing/header.hpp' file not found") {
		t.Fatalf("sourceDependencies() diagnostics = %q, want missing-header diagnostic", got)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("request count = %d, want 0", got)
	}
}

func githubTestArchive(t *testing.T, entries []githubTestArchiveEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{
			Name:       entry.name,
			Typeflag:   entry.typeflag,
			Linkname:   entry.linkname,
			Mode:       entry.mode,
			Size:       int64(len(entry.contents)),
			PAXRecords: entry.pax,
		}
		if entry.typeflag != tar.TypeReg && entry.typeflag != tar.TypeRegA {
			header.Size = 0
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatalf("write archive header %s: %v", entry.name, err)
		}
		if header.Size != 0 {
			if _, err := archive.Write([]byte(entry.contents)); err != nil {
				t.Fatalf("write archive contents %s: %v", entry.name, err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close tar archive: %v", err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatalf("close gzip archive: %v", err)
	}
	return output.Bytes()
}
