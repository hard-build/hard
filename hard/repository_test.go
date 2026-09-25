package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const firstCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const secondCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const nextCommit = "cccccccccccccccccccccccccccccccccccccccc"

func newRepositoryTestProxy(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	archives := map[string][]byte{
		firstCommit:  githubTestArchive(t, []githubTestArchiveEntry{{name: "first/first.h", typeflag: tar.TypeReg, mode: 0o644, contents: "#pragma once\n#include <github.com/demo/second/second.h>\ninline int dependency_value() { return second_value(); }\n"}}),
		secondCommit: githubTestArchive(t, []githubTestArchiveEntry{{name: "second/second.h", typeflag: tar.TypeReg, mode: 0o644, contents: "#pragma once\ninline int second_value() { return 1; }\n"}}),
		nextCommit:   githubTestArchive(t, []githubTestArchiveEntry{{name: "first/first.h", typeflag: tar.TypeReg, mode: 0o644, contents: "#pragma once\ninline int dependency_value() { return 2; }\n"}}),
	}
	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Header.Get("Accept") != "application/vnd.github+json" {
			t.Error("changed the proxy response format")
		}
		switch request.URL.Path {
		case "/v1/resolve":
			commit := firstCommit
			if strings.HasSuffix(request.URL.Query().Get("source"), "/second") {
				commit = secondCommit
			}
			if request.URL.Query().Get("ref") == "next" {
				commit = nextCommit
			}
			_ = json.NewEncoder(response).Encode(map[string]string{"commit": commit, "ref": "main"})
		case "/v1/snapshot":
			archive, ok := archives[request.URL.Query().Get("commit")]
			if !ok {
				http.Error(response, "unknown commit", 404)
				return
			}
			_, _ = response.Write(archive)
		default:
			http.Error(response, "unexpected upstream request", 500)
		}
	}))
	t.Cleanup(server.Close)
	return server, requests
}

func projectTestConfiguration(t *testing.T) configuration {
	t.Helper()
	runtime := t.TempDir()
	writeProjectTestFile(t, runtime, "hard.h", "#pragma once\n")
	return configuration{root: t.TempDir(), runtimeRoot: runtime, env: "host", cc: "c++", cflags: []string{"-std=c++20"}, entrypoints: []string{"main"}}
}

func runProjectTestCommand(configuration configuration, args ...string) (string, string, error) {
	var options projectOptions
	var stdout, stderr bytes.Buffer
	parsed, err := parseArguments(args, &stdout, &stderr, &options)
	if err == nil {
		err = runConfiguredCommand(parsed, options, configuration, strings.NewReader(""), &stdout, &stderr)
	}
	return stdout.String(), stderr.String(), err
}

func TestPinnedProjectFetchBuildUpdateAndLocked(t *testing.T) {
	server, requests := newRepositoryTestProxy(t)
	t.Setenv("HARD_PROXY", server.URL)
	root := t.TempDir()
	withWorkingDirectory(t, root)
	writeProjectTestFile(t, root, "app.cpp", "#include <github.com/demo/first/first.h>\nint main() { return dependency_value() == 1 ? 0 : 42; }\n")
	configuration := projectTestConfiguration(t)
	// A legacy tree must never satisfy a pinned include.
	writeProjectTestFile(t, configuration.root, "source/github.com/demo/first/first.h", "#error stale legacy snapshot\n")
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock", "--no-color"); err != nil || diagnostics != "" {
		t.Fatalf("fetch: %v\n%s\n%s", err, out, diagnostics)
	}
	project, err := readProjectFile(filepath.Join(root, projectFilename))
	if err != nil || len(project.Repositories) != 2 {
		t.Fatalf("transitive record: %#v, %v", project, err)
	}
	if requests.Load() != 4 {
		t.Fatalf("requests: %d", requests.Load())
	}
	if matches, _ := filepath.Glob(filepath.Join(configuration.root, "project", "*", "env")); len(matches) != 0 {
		t.Fatalf("fetch created artifacts: %v", matches)
	}
	before, _ := os.ReadFile(project.filename)
	if out, diagnostics, err := runProjectTestCommand(configuration, "run", "--locked", "--no-color"); err != nil || diagnostics != "" {
		t.Fatalf("run: %v\n%s\n%s", err, out, diagnostics)
	}
	out, _, err := runProjectTestCommand(configuration, "run", "--locked", "--no-color", "-v")
	if err != nil || !strings.Contains(out, "(CACHED)") {
		t.Fatalf("cached run: %v\n%s", err, out)
	}
	after, _ := os.ReadFile(project.filename)
	if !bytes.Equal(before, after) || requests.Load() != 4 {
		t.Fatal("locked changed file or refreshed refs")
	}
	writeProjectTestFile(t, root, "app.cpp", "#include <github.com/demo/first/first.h>\nint main() { return dependency_value() == 2 ? 0 : 42; }\n")
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--update=github.com/demo/first@next"); err != nil {
		t.Fatalf("update: %v\n%s\n%s", err, out, diagnostics)
	}
	updated, err := readProjectFile(project.filename)
	if err != nil || updated.Repositories["github.com/demo/first"].Commit != nextCommit || updated.Repositories["github.com/demo/second"] != project.Repositories["github.com/demo/second"] {
		t.Fatalf("targeted update: %v, %#v", err, updated)
	}
	if out, diagnostics, err := runProjectTestCommand(configuration, "run", "--locked"); err != nil {
		t.Fatalf("updated run: %v\n%s\n%s", err, out, diagnostics)
	}
	writeProjectTestFile(t, root, "app.cpp", "#include <github.com/demo/third/first.h>\nint main() { return 0; }\n")
	requestCount := requests.Load()
	if _, _, err := runProjectTestCommand(configuration, "build", "--locked"); err == nil || !strings.Contains(err.Error(), "--locked") {
		t.Fatalf("missing locked dependency: %v", err)
	}
	if requests.Load() != requestCount {
		t.Fatal("locked resolved an unrecorded dependency")
	}
	if out, diagnostics, err := runProjectTestCommand(configuration, "build"); err != nil {
		t.Fatalf("automatic addition: %v\n%s\n%s", err, out, diagnostics)
	}
	added, err := readProjectFile(project.filename)
	if err != nil || len(added.Repositories) != 3 || added.Repositories["github.com/demo/first"] != updated.Repositories["github.com/demo/first"] {
		t.Fatalf("addition changed existing pin: %v", err)
	}
	// A fresh cache is allowed in locked mode, but ref resolution is not.
	configuration.root = t.TempDir()
	requestCount = requests.Load()
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--locked"); err != nil {
		t.Fatalf("locked download: %v\n%s\n%s", err, out, diagnostics)
	}
	if requests.Load()-requestCount != 2 {
		t.Fatalf("expected only two reachable snapshot downloads, got %d", requests.Load()-requestCount)
	}
}

func TestPinnedFailureDoesNotWriteProject(t *testing.T) {
	server, _ := newRepositoryTestProxy(t)
	t.Setenv("HARD_PROXY", server.URL)
	root := t.TempDir()
	withWorkingDirectory(t, root)
	filename := writeProjectTestFile(t, root, projectFilename, "version: 1\nrepositories: {}\n")
	writeProjectTestFile(t, root, "app.cpp", "#include <github.com/demo/first/first.h>\n#include <missing.h>\n")
	before, _ := os.ReadFile(filename)
	if _, _, err := runProjectTestCommand(projectTestConfiguration(t), "fetch"); err == nil {
		t.Fatal("accepted unresolved local header")
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(before, after) {
		t.Fatalf("failure wrote dependency file: %s", after)
	}
}

func TestRepositoryChecksumAndReplacementPolicy(t *testing.T) {
	server, _ := newRepositoryTestProxy(t)
	var external repositoryConfiguration
	external.Proxy.URL = server.URL
	provider := newRepositoryProvider(external)
	filename := writeProjectTestFile(t, t.TempDir(), projectFilename, "version: 1\nrepositories: {}\n")
	project, _ := readProjectFile(filename)
	session, err := newDependencySession(project, t.TempDir(), projectOptions{}, provider)
	if err != nil {
		t.Fatal(err)
	}
	pin, err := provider.resolve("github.com/demo/first", "main")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, checksum, err := session.obtain(pin, nil)
	if err != nil {
		t.Fatal(err)
	}
	pin.Checksum = checksum
	bad := pin
	bad.Checksum = "sha256:" + strings.Repeat("0", 64)
	if _, _, err := session.obtain(bad, nil); err == nil {
		t.Fatal("accepted mismatching recorded checksum")
	}
	writeProjectTestFile(t, snapshot, "first.h", "corrupted\n")
	if _, _, err := session.obtain(pin, nil); err == nil {
		t.Fatal("accepted corrupted cache")
	}
	pin.Checksum = ""
	if _, _, err := session.obtain(pin, nil); err == nil {
		t.Fatal("adopted corrupted cache for a newly recorded dependency")
	}
	project.Repositories = map[string]repositoryPin{"github.com/demo/first": {Source: "github.com/demo/first", Ref: "main", Commit: firstCommit, Checksum: checksum}}
	provider.configuration.Replace = map[string]repositoryReplacement{"github.com/demo/first": {Source: "git.corp.example/third-party/first", Ref: "next"}}
	if _, err := newDependencySession(project, t.TempDir(), projectOptions{}, provider); err == nil {
		t.Fatal("replacement silently changed an existing pin")
	}
	updated, err := newDependencySession(project, t.TempDir(), projectOptions{updates: []string{"github.com/demo/first@next"}}, provider)
	if err != nil || updated.pins["github.com/demo/first"].Source != "git.corp.example/third-party/first" {
		t.Fatalf("explicit fork update: %v", err)
	}
}

func TestCorporateAuthenticationAndNoFallback(t *testing.T) {
	var upstream atomic.Int32
	var status atomic.Int32
	status.Store(401)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/repos/") {
			upstream.Add(1)
		}
		if request.Header.Get("Authorization") != "Bearer fixture-credential" {
			t.Error("missing scoped authorization")
		}
		http.Error(response, "fixture-credential must not enter errors", int(status.Load()))
	}))
	defer server.Close()
	t.Setenv("HARD_AUTH_TEST", "fixture-credential")
	config := writeProjectTestFile(t, t.TempDir(), "config.yaml", "proxy:\n  url: "+server.URL+"\n  fallback: true\nauth:\n  "+strings.TrimPrefix(server.URL, "http://")+":\n    token_env: HARD_AUTH_TEST\n")
	t.Setenv("HARD_CONFIG", config)
	configuration, err := loadRepositoryConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	provider := newRepositoryProvider(configuration)
	provider.githubURL = server.URL
	for _, code := range []int32{401, 403, 500} {
		status.Store(code)
		_, err := provider.resolve("github.com/demo/first", "main")
		if err == nil || strings.Contains(err.Error(), "fixture-credential") {
			t.Fatalf("unsafe HTTP error: %v", err)
		}
	}
	if upstream.Load() != 0 {
		t.Fatal("authentication/server error escaped to upstream")
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, target.URL+"/?token=fixture-credential", 302)
	}))
	defer redirect.Close()
	provider.configuration.Proxy.URL = redirect.URL
	if _, err := provider.resolve("github.com/demo/first", "main"); err == nil || strings.Contains(err.Error(), "fixture-credential") {
		t.Fatalf("redirect: %v", err)
	}
	if redirected.Load() != 0 {
		t.Fatal("proxy redirected outside its origin")
	}
}

func TestParallelProjectsUseDifferentImmutableViews(t *testing.T) {
	server, _ := newRepositoryTestProxy(t)
	var external repositoryConfiguration
	external.Proxy.URL = server.URL
	configuration := projectTestConfiguration(t)
	source := writeProjectTestFile(t, t.TempDir(), "app.cpp", "#include <github.com/demo/first/first.h>\nint main() { return dependency_value(); }\n")
	var sessions []*dependencySession
	var roots []string
	var outputs []string
	for _, ref := range []string{"main", "next"} {
		filename := writeProjectTestFile(t, t.TempDir(), projectFilename, "version: 1\nrepositories: {}\n")
		project, _ := readProjectFile(filename)
		session, err := newDependencySession(project, configuration.root, projectOptions{}, newRepositoryProvider(external))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"first", "second"} {
			pin, err := session.provider.resolve("github.com/demo/"+name, map[bool]string{true: ref, false: "main"}[name == "first"])
			if err != nil {
				t.Fatal(err)
			}
			session.pins["github.com/demo/"+name] = pin
			session.requested["github.com/demo/"+name] = true
		}
		view, err := session.view(configuration, nil)
		if err != nil {
			t.Fatal(err)
		}
		sessions, roots = append(sessions, session), append(roots, view)
		outputs = append(outputs, filepath.Join(t.TempDir(), "app"))
	}
	if roots[0] == roots[1] {
		t.Fatal("different pins share a view")
	}
	var wait sync.WaitGroup
	errorsByBuild := make([]error, 2)
	for index := range sessions {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			progress := newProgressBar(io.Discard, -1, false, true, true)
			resolver := newGitHubSnapshotResolver(roots[index], progress)
			resolver.session = sessions[index]
			errorsByBuild[index] = buildSourcesWithProgressExecutable(roots[index], configuration.runtimeRoot, "host", "", "c++", effectiveCFlags(configuration.cflags, roots[index], configuration.runtimeRoot), nil, []string{"main"}, []string{source}, outputs[index], 2, false, true, progress, io.Discard, false, resolver)
		}(index)
	}
	wait.Wait()
	for index, err := range errorsByBuild {
		if err != nil {
			t.Fatal(err)
		}
		var exit *exec.ExitError
		if err := exec.Command(outputs[index]).Run(); !errors.As(err, &exit) || exit.ExitCode() != index+1 {
			t.Fatalf("binary %d mixed dependencies: %v", index, err)
		}
	}
}

func TestGitHubPinnedProviderAndExplicitProxyFallback(t *testing.T) {
	var requests []string
	var mutex sync.Mutex
	archive := githubTestArchive(t, []githubTestArchiveEntry{{name: "arbitrary-root/header.h", typeflag: tar.TypeReg, mode: 0o644, contents: "// pinned\n"}})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		requests = append(requests, request.URL.EscapedPath())
		mutex.Unlock()
		accept := "application/vnd.github+json"
		if strings.Contains(request.URL.Path, "/commits/") {
			accept = "application/vnd.github.sha"
		}
		if request.Header.Get("Accept") != accept {
			t.Errorf("Accept for %s: %q, want %q", request.URL.Path, request.Header.Get("Accept"), accept)
		}
		switch request.URL.EscapedPath() {
		case "/repos/demo/first":
			_, _ = io.WriteString(response, `{"default_branch":"feature/branch"}`)
		case "/repos/demo/first/commits/feature%2Fbranch":
			_, _ = io.WriteString(response, firstCommit+"\n")
		case "/repos/demo/first/tarball/" + firstCommit:
			_, _ = response.Write(archive)
		default:
			http.Error(response, "not in mirror", 404)
		}
	}))
	defer server.Close()
	var configuration repositoryConfiguration
	configuration.Proxy.URL = server.URL + "/mirror"
	provider := newRepositoryProvider(configuration)
	provider.githubURL = server.URL
	if _, err := provider.resolve("github.com/demo/first", ""); err == nil {
		t.Fatal("implicit fallback")
	}
	provider.configuration.Proxy.Fallback = true
	pin, err := provider.resolve("github.com/demo/first", "")
	if err != nil || pin.Commit != firstCommit || pin.Ref != "feature/branch" {
		t.Fatalf("GitHub resolution: %#v, %v", pin, err)
	}
	input, err := provider.snapshot(pin)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	contents, err := io.ReadAll(input)
	if err != nil || !bytes.Equal(contents, archive) {
		t.Fatalf("GitHub pinned archive: %v", err)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(requests) != 6 || requests[len(requests)-1] != "/repos/demo/first/tarball/"+firstCommit {
		t.Fatalf("requests: %v", requests)
	}
}

func TestGitHubRevisionSHAResponse(t *testing.T) {
	for _, test := range []struct {
		name, ref, path string
	}{
		{"default branch", "", "feature%2Fbranch"},
		{"branch", "feature/branch", "feature%2Fbranch"},
		{"tag", "10.0.0", "10.0.0"},
		{"qualified tag", "tags/10.0.0", "tags%2F10.0.0"},
		{"commit", firstCommit, firstCommit},
	} {
		t.Run(test.name, func(t *testing.T) {
			var metadata, commits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Header.Get("User-Agent") != "hard" || request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
					t.Error("changed GitHub request headers")
				}
				switch request.URL.EscapedPath() {
				case "/repos/demo/first":
					metadata.Add(1)
					if request.Header.Get("Accept") != "application/vnd.github+json" {
						t.Error("repository metadata no longer requests JSON")
					}
					_, _ = io.WriteString(response, `{"default_branch":"feature/branch"}`)
				case "/repos/demo/first/commits/" + test.path:
					commits.Add(1)
					if request.Header.Get("Accept") == "application/vnd.github.sha" {
						_, _ = io.WriteString(response, firstCommit+"\n")
						return
					}
					// Large commit patches must not be downloaded just to resolve a ref.
					t.Error("commit resolution did not request the SHA response")
					_ = json.NewEncoder(response).Encode(map[string]any{"sha": firstCommit, "files": []map[string]string{{"patch": strings.Repeat("x", 1<<20)}}})
				default:
					t.Errorf("unexpected GitHub request: %s", request.URL.EscapedPath())
					http.NotFound(response, request)
				}
			}))
			defer server.Close()
			provider := newRepositoryProvider(repositoryConfiguration{})
			provider.githubURL = server.URL
			pin, err := provider.resolve("github.com/demo/first", test.ref)
			ref := test.ref
			if ref == "" {
				ref = "feature/branch"
			}
			if err != nil || pin != (repositoryPin{Source: "github.com/demo/first", Ref: ref, Commit: firstCommit}) {
				t.Fatalf("SHA resolution: %#v, %v", pin, err)
			}
			if commits.Load() != 1 || metadata.Load() != map[bool]int32{true: 1, false: 0}[test.ref == ""] {
				t.Fatalf("unexpected request counts: metadata=%d commits=%d", metadata.Load(), commits.Load())
			}
		})
	}
}

func TestGitHubRevisionSHAValidation(t *testing.T) {
	for _, test := range []struct {
		name, body, commit string
		status             int
		truncated          bool
	}{
		{name: "SHA1", body: firstCommit, commit: firstCommit},
		{name: "SHA256", body: strings.Repeat("a", 64), commit: strings.Repeat("a", 64)},
		{name: "whitespace", body: " \t" + firstCommit + "\r\n", commit: firstCommit},
		{name: "limit", body: firstCommit + strings.Repeat(" ", 1024-len(firstCommit)), commit: firstCommit},
		{name: "empty"},
		{name: "abbreviated", body: firstCommit[:7]},
		{name: "uppercase", body: strings.ToUpper(firstCommit)},
		{name: "nonhex", body: strings.Repeat("z", 40)},
		{name: "JSON", body: `{"sha":"` + firstCommit + `"}`},
		{name: "trailing data", body: firstCommit + "\nfixture-credential"},
		{name: "oversized", body: firstCommit + strings.Repeat(" ", 1025-len(firstCommit))},
		{name: "hidden trailing data", body: firstCommit + strings.Repeat(" ", 2048) + "fixture-credential"},
		{name: "truncated", body: firstCommit, truncated: true},
		{name: "not found", status: http.StatusNotFound, body: "fixture-credential"},
		{name: "unauthorized", status: http.StatusUnauthorized, body: "fixture-credential"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if test.truncated {
					response.Header().Set("Content-Length", "100")
				}
				if test.status != 0 {
					response.WriteHeader(test.status)
				}
				_, _ = io.WriteString(response, test.body)
			}))
			defer server.Close()
			provider := newRepositoryProvider(repositoryConfiguration{})
			provider.githubURL = server.URL
			pin, err := provider.resolve("github.com/demo/first", "release")
			if test.commit != "" {
				if err != nil || pin.Commit != test.commit {
					t.Fatalf("valid SHA rejected: %#v, %v", pin, err)
				}
				return
			}
			if err == nil || pin.Commit != "" || strings.Contains(err.Error(), "fixture-credential") || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("invalid response accepted or disclosed: %#v, %v", pin, err)
			}
			if test.status != 0 {
				var status *repositoryHTTPError
				if !errors.As(err, &status) || status.status != test.status {
					t.Fatalf("lost HTTP status: %v", err)
				}
			}
		})
	}
}

func TestGitHubRevisionRedirectAuthorization(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(map[bool]string{false: "anonymous destination", true: "scoped destination"}[scoped], func(t *testing.T) {
			var redirected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				redirected.Add(1)
				want := ""
				if scoped {
					want = "Bearer destination-fixture"
				}
				if request.Header.Get("Authorization") != want || request.Header.Get("Accept") != "application/vnd.github.sha" {
					t.Error("redirect changed the media type or credential scope")
				}
				_, _ = io.WriteString(response, firstCommit)
			}))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer origin-fixture" {
					t.Error("missing origin credentials")
				}
				http.Redirect(response, request, target.URL+"/commit", http.StatusFound)
			}))
			defer origin.Close()
			contents := "auth:\n  " + strings.TrimPrefix(origin.URL, "http://") + ":\n    token_env: HARD_AUTH_ORIGIN\n"
			if scoped {
				contents += "  " + strings.TrimPrefix(target.URL, "http://") + ":\n    token_env: HARD_AUTH_DESTINATION\n"
			}
			t.Setenv("HARD_CONFIG", writeProjectTestFile(t, t.TempDir(), "config.yaml", contents))
			t.Setenv("HARD_PROXY", "")
			t.Setenv("HARD_AUTH_ORIGIN", "origin-fixture")
			t.Setenv("HARD_AUTH_DESTINATION", "destination-fixture")
			configuration, err := loadRepositoryConfiguration()
			if err != nil {
				t.Fatal(err)
			}
			provider := newRepositoryProvider(configuration)
			provider.githubURL = origin.URL
			pin, err := provider.resolve("github.com/demo/first", "release")
			if err != nil || pin.Commit != firstCommit || redirected.Load() != 1 {
				t.Fatalf("redirected resolution: %#v, %v (requests=%d)", pin, err, redirected.Load())
			}
		})
	}
}
