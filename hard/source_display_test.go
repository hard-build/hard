package main

import (
	"archive/tar"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinnedSourceDisplayPaths(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "project")
	local := writeProjectTestFile(t, project, "local.cpp", "")
	cache := filepath.Join(workspace, "cache [data]")
	view := filepath.Join(cache, "project", "selection")
	snapshot := filepath.Join(cache, "snapshot", repositoryDigest([]byte("git.corp.example/fork/library")), firstCommit)
	external := writeProjectTestFile(t, snapshot, "src/value.cpp", "")
	otherRevision := writeProjectTestFile(t, filepath.Dir(snapshot), secondCommit+"/src/value.cpp", "")
	prefixSibling := writeProjectTestFile(t, snapshot+"-other", "src/value.cpp", "")
	alias := filepath.Join(view, "source", "github.com", "hard-build", "library")
	if err := os.MkdirAll(filepath.Dir(alias), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureWellKnownGitHubRepositoryAlias(alias, snapshot); err != nil {
		t.Fatal(err)
	}
	wellKnown := filepath.Join(view, "source", "hard")
	if err := ensureWellKnownGitHubRepositoryAlias(wellKnown, alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(filepath.Dir(alias), "broken")); err != nil {
		t.Fatal(err)
	}
	rootAlias := filepath.Join(workspace, "view-alias")
	if err := os.Symlink(view, rootAlias); err != nil {
		t.Fatal(err)
	}
	relative := func(path string) string {
		t.Helper()
		result, err := filepath.Rel(project, path)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	const label = "github.com/hard-build/library/src/value.cpp"
	cases := []struct {
		name, source, compile, parsing string
	}{
		{"snapshot", external, label, label},
		{"relative snapshot", relative(external), label, label},
		{"repository alias", filepath.Join(alias, "src/value.cpp"), label, label},
		{"well-known alias", filepath.Join(wellKnown, "src/value.cpp"), label, label},
		{"local", "local.cpp", "local.cpp", "local.cpp"},
		{"absolute local", local, local, "local.cpp"},
		{"unselected revision", otherRevision, otherRevision, relative(otherRevision)},
		{"snapshot prefix sibling", prefixSibling, prefixSibling, relative(prefixSibling)},
	}
	for _, root := range []struct{ name, path string }{
		{"absolute root", view}, {"relative root", relative(view)}, {"symlinked root", rootAlias},
	} {
		for _, tc := range cases {
			t.Run(root.name+"/"+tc.name, func(t *testing.T) {
				if got := compileSourceDisplayPath(root.path, tc.source, project); got != tc.compile {
					t.Errorf("compile label = %q, want %q", got, tc.compile)
				}
				if got := buildParsingDisplayPath(root.path, tc.source, project); got != tc.parsing {
					t.Errorf("parsing label = %q, want %q", got, tc.parsing)
				}
			})
		}
	}
}

func TestPinnedSourceProgress(t *testing.T) {
	archive := githubTestArchive(t, []githubTestArchiveEntry{
		{name: "library/value.h", typeflag: tar.TypeReg, mode: 0o644, contents: "#pragma once\nint value();\n"},
		{name: "library/value.cpp", typeflag: tar.TypeReg, mode: 0o644, contents: "#include \"value.h\"\nint value() { return 7; }\n"},
	})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/resolve":
			_ = json.NewEncoder(response).Encode(map[string]string{"commit": firstCommit, "ref": "main"})
		case "/v1/snapshot":
			_, _ = response.Write(archive)
		default:
			http.Error(response, "unexpected request", 500)
		}
	}))
	defer server.Close()
	t.Setenv("HARD_PROXY", server.URL)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/library/value.h>\nint main() { return value() == 7 ? 0 : 1; }\n")
	configuration := projectTestConfiguration(t)
	const label = "github.com/demo/library/value.cpp"
	for _, args := range [][]string{
		{"fetch", "--lock", "--no-color", "-v"},
		{"build", "--locked", "--no-color", "-v"},
		{"run", "--locked", "--no-color", "-v"},
	} {
		out, diagnostics, err := runProjectTestCommand(configuration, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
		}
		if !strings.Contains(out, "Parsing "+label) {
			t.Errorf("%v: missing canonical parsing label:\n%s", args, out)
		}
		if args[0] != "fetch" && !strings.Contains(out, "Compiling "+label) {
			t.Errorf("%v: missing canonical compile label:\n%s", args, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if (strings.Contains(line, "] Parsing ") || strings.Contains(line, "] Compiling ")) && strings.Contains(line, "snapshot/") {
				t.Errorf("%v: snapshot path in progress label: %s", args, line)
			}
		}
		if args[0] == "build" {
			snapshotSource := filepath.Join(configuration.root, "snapshot", repositoryDigest([]byte("github.com/demo/library")), firstCommit, "value.cpp")
			if !strings.Contains(out, " -c "+quoteShellArgument(snapshotSource)+" -o ") {
				t.Errorf("compiler command lost physical source path:\n%s", out)
			}
		}
		if args[0] == "run" {
			for _, action := range []string{"Parsing", "Compiling"} {
				if !strings.Contains(out, action+" "+label+" (CACHED)") {
					t.Errorf("missing canonical cached %s label:\n%s", action, out)
				}
			}
		}
	}
}
