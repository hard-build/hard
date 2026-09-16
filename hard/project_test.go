package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectSchema(t *testing.T) {
	for _, contents := range []string{
		"{}", "version: 2", "version: '1'", "version: 1\ntarget: host",
		"version: 1\nversion: 1", "version: 1\nformat: ''", "version: 1\nformat: null",
		"version: 1\nexclude: null", "version: 1\nexclude: [123]",
		"version: 1\nexclude: [/tmp]", "version: 1\nexclude: ['../outside']",
		"version: 1\nexclude: ['**/build']", "version: 1\nrepositories: null",
		"version: 1\nrepositories: []", "version: 1\n---\nversion: 1",
		"version: 1\nexclude: &excluded [build]",
		"version: 1\nrepositories: {github.com/a/b: {source: github.com/a/b}}",
	} {
		t.Run(contents, func(t *testing.T) {
			filename := writeProjectTestFile(t, t.TempDir(), projectFilename, contents+"\n")
			if _, err := readProjectFile(filename); err == nil {
				t.Fatalf("accepted invalid project: %s", contents)
			}
		})
	}
	filename := writeProjectTestFile(t, t.TempDir(), projectFilename, "version: 1\nformat: format.v1\nexclude: []\nrepositories: {}\n")
	project, err := readProjectFile(filename)
	if err != nil || !project.recorded {
		t.Fatalf("read valid project: %v, %#v", err, project)
	}
}

func TestProjectRejectsPathsField(t *testing.T) {
	for _, value := range []string{"[src, tests]", "[]", "null"} {
		t.Run(value, func(t *testing.T) {
			filename := writeProjectTestFile(t, t.TempDir(), projectFilename, "version: 1\npaths: "+value+"\n")
			if _, err := readProjectFile(filename); err == nil || !strings.Contains(err.Error(), "field paths not found") {
				t.Fatalf("paths must be an unknown field: %v", err)
			}
		})
	}
}

func TestProjectSearchAndSourceSelection(t *testing.T) {
	root := t.TempDir()
	filename := writeProjectTestFile(t, root, projectFilename, "version: 1\nexclude: [src/generated]\n")
	writeProjectTestFile(t, root, "src/a.cpp", "")
	explicit := writeProjectTestFile(t, root, "src/generated/b.cpp", "")
	writeProjectTestFile(t, root, "outside.cpp", "")
	working := filepath.Join(root, "src")
	if found, err := findProjectFile(working); err != nil || found != filename {
		t.Fatalf("find project: %s, %v", found, err)
	}
	project, err := readProjectFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	paths, excluded := project.sourcePaths(arguments{paths: []string{"."}})
	sources, err := discoverSourcesFrom("build", paths, working, excluded)
	if err != nil || !reflect.DeepEqual(sources, []string{"a.cpp"}) {
		t.Fatalf("selection: %#v, %v", sources, err)
	}
	paths, excluded = project.sourcePaths(arguments{paths: []string{"../outside.cpp", "."}})
	sources, err = discoverSourcesFrom("build", paths, working, excluded)
	if err != nil || !reflect.DeepEqual(sources, []string{"../outside.cpp", "a.cpp"}) {
		t.Fatalf("cwd-relative CLI paths: %#v, %v", sources, err)
	}
	sources, err = discoverSourcesFrom("build", []string{explicit}, working, excluded)
	if err != nil || !reflect.DeepEqual(sources, []string{"generated/b.cpp"}) {
		t.Fatalf("explicit excluded file: %#v, %v", sources, err)
	}
	writeProjectTestFile(t, working, ".git", "gitdir: elsewhere\n")
	if found, err := findProjectFile(working); err != nil || found != "" {
		t.Fatalf("crossed Git boundary: %s, %v", found, err)
	}
}

func TestProjectCLIOptions(t *testing.T) {
	for _, args := range [][]string{
		{"fetch", "--lock", "--locked"}, {"fetch", "--lock", "--update=github.com/a/b@main"},
		{"fetch", "--locked", "--update=github.com/a/b@main"},
		{"build", "--lock"}, {"format", "--locked"}, {"run", "--update=github.com/a/b@main"},
	} {
		if _, err := parseArguments(args, io.Discard, io.Discard); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
	var options projectOptions
	if _, err := parseArguments([]string{"format", "--format=format.v1", "."}, io.Discard, io.Discard, &options); err != nil || !options.explicitFormat {
		t.Fatalf("explicit metadata: %#v, %v", options, err)
	}
	parsed, err := parseArguments([]string{"run", "--", "--locked", "arg"}, io.Discard, io.Discard, &options)
	if err != nil || options.locked || !reflect.DeepEqual(parsed.paths, []string{"."}) || !reflect.DeepEqual(parsed.programArguments, []string{"--locked", "arg"}) {
		t.Fatalf("run argument boundary: %#v, %#v, %v", options, parsed, err)
	}
	if _, err := parseArguments([]string{"fetch", "--lock"}, io.Discard, io.Discard, &options); err != nil || !options.lock {
		t.Fatalf("lock options: %#v, %v", options, err)
	}
}

func testRepositoryPin() repositoryPin {
	return repositoryPin{Source: "github.com/a/b", Ref: "main", Commit: strings.Repeat("a", 40), Checksum: "sha256:" + strings.Repeat("b", 64)}
}

func TestProjectPreservesManualBytesAndComments(t *testing.T) {
	for _, original := range []string{
		"# Project\nversion: 1\nformat: 'format.v1' # custom spelling\nexclude: [build, bin]\n",
		"# Project\nversion: 1\nrepositories: {} # pins\n\n# Selection\nexclude: [build, bin]\nformat: 'format.v1' # custom spelling\n",
		"# Project\nversion: 1\nformat: 'format.v1' # custom spelling\nexclude: [build, bin]\n... # End\n",
		"# Project\nversion: 1\nformat: 'format.v1' # custom spelling\nexclude: [build, bin]\nrepositories: {}\n... # End\n",
	} {
		filename := writeProjectTestFile(t, t.TempDir(), projectFilename, original)
		project, err := readProjectFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := project.writeRepositories(map[string]repositoryPin{"github.com/a/b": testRepositoryPin()}); err != nil {
			t.Fatal(err)
		}
		contents, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		for _, exact := range []string{"# Project\nversion: 1\n", "format: 'format.v1' # custom spelling\n", "exclude: [build, bin]\n"} {
			if !bytes.Contains(contents, []byte(exact)) {
				t.Fatalf("lost manual bytes %q in %s", exact, contents)
			}
		}
		if strings.Contains(original, "# Selection") && bytes.Count(contents, []byte("# Selection")) != 1 {
			t.Fatalf("lost or duplicated comment: %s", contents)
		}
		if strings.Contains(original, "... # End") && !bytes.HasSuffix(contents, []byte("... # End\n")) {
			t.Fatalf("lost document-end marker: %s", contents)
		}
		reloaded, err := readProjectFile(filename)
		if err != nil || reloaded.Repositories["github.com/a/b"] != testRepositoryPin() {
			t.Fatalf("reload: %v, %s", err, contents)
		}
		writeProjectTestFile(t, filepath.Dir(filename), projectFilename, "version: 1\n# concurrent edit\n")
		if err := reloaded.writeRepositories(reloaded.Repositories); err == nil {
			t.Fatal("overwrote concurrent edit")
		}
	}
}

func TestProjectDefaultsAndExcludedFiles(t *testing.T) {
	project := t.TempDir()
	withWorkingDirectory(t, project)
	filename := writeProjectTestFile(t, project, projectFilename, "version: 1\nformat: project-style\nexclude: [excluded.cpp]\n")
	writeProjectTestFile(t, project, "excluded.cpp", "#include <must-not-be-resolved.h>\n")
	configuration := projectTestConfiguration(t)
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock"); err != nil {
		t.Fatalf("default source traversal bypassed exclusion: %v\n%s\n%s", err, out, diagnostics)
	}
	recorded, err := readProjectFile(filename)
	if err != nil || !recorded.recorded || len(recorded.Repositories) != 0 {
		t.Fatalf("empty recorded selection: %#v, %v", recorded, err)
	}
	if _, _, err := runProjectTestCommand(configuration, "fetch", "excluded.cpp"); err == nil {
		t.Fatal("explicit CLI file did not override exclusion")
	}
	for _, explicit := range []bool{false, true} {
		parsed := arguments{command: "format", format: "cli-style"}
		_, session, err := prepareProject(&parsed, projectOptions{explicitFormat: explicit}, configuration.root, project)
		session.close()
		want := "project-style"
		if explicit {
			want = "cli-style"
		}
		if err != nil || parsed.format != want {
			t.Fatalf("format precedence: %q, %v", parsed.format, err)
		}
	}
}

func TestProjectRejectsSymlinkAndLockedMissingSection(t *testing.T) {
	root := t.TempDir()
	file := writeProjectTestFile(t, root, "actual", "version: 1\n")
	alias := filepath.Join(root, projectFilename)
	if err := os.Symlink(file, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := readProjectFile(alias); err == nil {
		t.Fatal("accepted project symlink")
	}
	other := t.TempDir()
	writeProjectTestFile(t, other, projectFilename, "version: 1\n")
	parsed := arguments{command: "build"}
	if _, session, err := prepareProject(&parsed, projectOptions{locked: true}, t.TempDir(), other); err == nil {
		session.close()
		t.Fatal("locked accepted missing repositories")
	}
	if _, session, err := prepareProject(&parsed, projectOptions{}, t.TempDir(), other); err != nil || session == nil || session.record {
		t.Fatalf("ordinary unpinned: %v", err)
	} else {
		session.close()
	}
	if _, err := os.Stat(filepath.Join(other, "env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unexpected artifact directory")
	}
}

func writeProjectTestFile(t *testing.T, root, path, contents string) string {
	t.Helper()
	writeBuildFile(t, root, path, contents)
	return filepath.Join(root, path)
}
