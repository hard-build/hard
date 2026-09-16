package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedDiscoveryDefaults(t *testing.T, configuration configuration, defaults []repositoryPin, additional ...repositoryPin) {
	t.Helper()
	provider, err := loadRepositoryConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	session := &dependencySession{root: configuration.root, provider: newRepositoryProvider(provider)}
	for _, pin := range defaults {
		if _, err := session.defaultPin(pin.Source, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, pin := range additional {
		if _, _, err := session.obtain(pin, nil); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HARD_PROXY", "")
	t.Setenv("HARD_CONFIG", "")
}

func readDiscoveryRecord(t *testing.T, configuration configuration, project string) (string, dependencyDiscoveryRecord) {
	t.Helper()
	owner, err := localProjectRoot(configuration.root, configuration.env, project)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(owner, "dependencies.json")
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var record dependencyDiscoveryRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		t.Fatal(err)
	}
	if record.Result != discoveryRecordDigest(record) {
		t.Fatal("invalid discovery record digest")
	}
	return filename, record
}

func runDiscoveryCommand(t *testing.T, configuration configuration, args ...string) string {
	t.Helper()
	out, diagnostics, err := runProjectTestCommand(configuration, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s\n%s", args, err, out, diagnostics)
	}
	if strings.Count(out, "Searching source files") != 1 {
		t.Fatalf("source search repeated:\n%s", out)
	}
	return out
}

func TestUnrecordedDiscoveryWarmCommands(t *testing.T) {
	for _, command := range []string{"fetch", "build", "run", "test"} {
		t.Run(command, func(t *testing.T) {
			proxy := newInheritanceTestProxy(t)
			pin, archive := inheritedTestSnapshot(t, "github.com/demo/value", "main", firstCommit, map[string]string{
				"value.h": "#pragma once\nint value();\n", "value.cpp": "#include \"value.h\"\nint value() { return 7; }\n",
			})
			proxy.add(pin, archive)
			configuration := projectTestConfiguration(t)
			if command == "test" {
				configuration.cc, _ = installTestTools(t)
			}
			seedDiscoveryDefaults(t, configuration, []repositoryPin{pin})
			before := proxy.log()
			project := t.TempDir()
			withWorkingDirectory(t, project)
			source := "app.cpp"
			if command == "test" {
				source = "app.test.cpp"
			}
			writeProjectTestFile(t, project, source, "#include <github.com/demo/value/value.h>\nint main() { return value() == 7 ? 0 : 1; }\n")
			for iteration := 0; iteration < 3; iteration++ {
				out := runDiscoveryCommand(t, configuration, command, "-v", "--no-color", "-j4")
				if iteration == 0 {
					continue
				}
				for _, name := range []string{source, pin.Source + "/value.cpp"} {
					if strings.Count(out, "Parsing "+name) != 1 || !strings.Contains(out, "Parsing "+name+" (CACHED)") {
						t.Fatalf("warm %s repeated analysis or missed cache:\n%s", command, out)
					}
				}
			}
			if _, err := os.Stat(filepath.Join(project, projectFilename)); !os.IsNotExist(err) || before != proxy.log() {
				t.Fatalf("warm command wrote hard.yaml or contacted upstream: %v", err)
			}
			_, record := readDiscoveryRecord(t, configuration, project)
			if !sameRepositoryContents(record.Pins[pin.Source], pin) {
				t.Fatalf("wrong cached selection: %#v", record.Pins)
			}
		})
	}
}

func TestDiscoveryCacheDoesNotPinInheritedOrDefaultRevisions(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	inherited, inheritedArchive := inheritedTestSnapshot(t, "github.com/demo/leaf", "release", firstCommit, map[string]string{"leaf.h": "#pragma once\n"})
	proxy.add(inherited, inheritedArchive)
	defaultPin, defaultArchive := inheritedTestSnapshot(t, inherited.Source, "main", secondCommit, map[string]string{"leaf.h": "#pragma once\n// default\n"})
	proxy.add(defaultPin, defaultArchive)
	parent, parentArchive := inheritedTestSnapshot(t, "github.com/demo/parent", "main", firstCommit, map[string]string{
		"parent.h": "#pragma once\n#include <github.com/demo/leaf/leaf.h>\n", "hard.yaml": inheritedTestYAML(t, map[string]repositoryPin{inherited.Source: inherited}),
	})
	proxy.add(parent, parentArchive)
	configuration := projectTestConfiguration(t)
	seedDiscoveryDefaults(t, configuration, []repositoryPin{parent, defaultPin}, inherited)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include \"selection.h\"\n")
	writeProjectTestFile(t, project, "selection.h", "#include <github.com/demo/parent/parent.h>\n")
	for iteration := 0; iteration < 2; iteration++ {
		out := runDiscoveryCommand(t, configuration, "fetch", "-v")
		if iteration == 1 && (strings.Count(out, "Parsing app.cpp") != 1 || !strings.Contains(out, "Parsing app.cpp (CACHED)")) {
			t.Fatalf("inherited warm selection missed:\n%s", out)
		}
		_, record := readDiscoveryRecord(t, configuration, project)
		if record.Pins[inherited.Source] != inherited {
			t.Fatal("default replaced an inherited revision")
		}
	}
	// Removing the parent's include must remove its inherited requirement too.
	writeProjectTestFile(t, project, "selection.h", "#include <github.com/demo/leaf/leaf.h>\n")
	runDiscoveryCommand(t, configuration, "fetch", "-v")
	_, record := readDiscoveryRecord(t, configuration, project)
	if len(record.Pins) != 1 || !sameRepositoryContents(record.Pins[inherited.Source], defaultPin) {
		t.Fatalf("old inherited revision became a hidden pin: %#v", record.Pins)
	}
	// @default is an input, not merely a fallback for missing cache entries.
	writeProjectTestFile(t, configuration.root, "snapshot/"+defaultPin.Source+"/@default", inherited.Commit+"\n")
	runDiscoveryCommand(t, configuration, "fetch", "-v")
	_, record = readDiscoveryRecord(t, configuration, project)
	if !sameRepositoryContents(record.Pins[inherited.Source], inherited) {
		t.Fatal("cached selection ignored changed @default")
	}
	// The explicit project choice remains authoritative, including under --locked.
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{defaultPin.Source: defaultPin}))
	before := readTestFile(t, filename)
	runDiscoveryCommand(t, configuration, "fetch", "--locked", "-v")
	owner, _ := localProjectRoot(configuration.root, configuration.env, project)
	selected, err := filepath.EvalSymlinks(filepath.Join(owner, "include", defaultPin.Source))
	if err != nil || filepath.Base(selected) != "@"+defaultPin.Commit || readTestFile(t, filename) != before {
		t.Fatalf("cache overrode the project pin: %s, %v", selected, err)
	}
}

func TestDiscoveryCacheInvalidationAndValidation(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pin, archive := inheritedTestSnapshot(t, "github.com/demo/value", "main", firstCommit, map[string]string{
		"value.h": "#pragma once\n", "unused.txt": "checksum input\n",
	})
	proxy.add(pin, archive)
	configuration := projectTestConfiguration(t)
	seedDiscoveryDefaults(t, configuration, []repositoryPin{pin})
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <github.com/demo/value/value.h>\n")
	writeProjectTestFile(t, project, "local.cpp", "// no external dependencies\n")
	run := func(args ...string) string {
		return runDiscoveryCommand(t, configuration, append([]string{"fetch", "-v"}, args...)...)
	}
	run("app.cpp")
	filename, _ := readDiscoveryRecord(t, configuration, project)
	if out := run("app.cpp", "--no-cache"); strings.Contains(out, "(CACHED)") {
		t.Fatalf("--no-cache restored analysis:\n%s", out)
	}
	for _, corrupt := range []string{"{invalid", `{"version":999}`} {
		if err := os.WriteFile(filename, []byte(corrupt), 0o644); err != nil {
			t.Fatal(err)
		}
		run("app.cpp")
		readDiscoveryRecord(t, configuration, project)
	}
	// A semantically changed record must not retain its previous digest.
	_, record := readDiscoveryRecord(t, configuration, project)
	changed := record.Pins[pin.Source]
	changed.Commit = secondCommit
	record.Pins[pin.Source] = changed
	contents, _ := json.Marshal(record)
	if err := os.WriteFile(filename, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	run("app.cpp")
	// Changing roots or flags must not preload irrelevant repositories.
	run("local.cpp")
	_, record = readDiscoveryRecord(t, configuration, project)
	if len(record.Pins) != 0 {
		t.Fatal("selection from another set of root sources was restored")
	}
	writeProjectTestFile(t, project, "app.cpp", "#ifndef LOCAL_ONLY\n#include <github.com/demo/value/value.h>\n#endif\n")
	run("app.cpp")
	configuration.cflags = append(configuration.cflags, "-DLOCAL_ONLY")
	run("app.cpp")
	_, record = readDiscoveryRecord(t, configuration, project)
	if len(record.Pins) != 0 {
		t.Fatal("selection from another flag configuration was restored")
	}
	configuration.cflags = configuration.cflags[:len(configuration.cflags)-1]
	run("app.cpp")
	// The entire snapshot is still validated, not just parsed headers.
	writeProjectTestFile(t, configuration.root, "snapshot/"+pin.Source+"/@"+pin.Commit+"/unused.txt", "corrupted\n")
	if _, _, err := runProjectTestCommand(configuration, "fetch", "app.cpp"); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("warm discovery bypassed snapshot validation: %v", err)
	}
}
