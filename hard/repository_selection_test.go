package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordedCommandsOnlyDownloadReachableRepositories(t *testing.T) {
	for _, command := range []string{"fetch", "build", "run", "test"} {
		for _, locked := range []bool{false, true} {
			name := command
			if locked {
				name += "_locked"
			}
			t.Run(name, func(t *testing.T) {
				proxy := newInheritanceTestProxy(t)
				used, archive := inheritedTestSnapshot(t, "github.com/demo/used", "main", firstCommit, map[string]string{
					"value.h": "#pragma once\ninline int value() { return 7; }\n",
				})
				proxy.add(used, archive)
				unused := used
				unused.Source = "github.com/demo/unused"
				configuration := projectTestConfiguration(t)
				if command == "test" {
					configuration.cc, _ = installTestTools(t)
				}
				project := t.TempDir()
				withWorkingDirectory(t, project)
				filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{used.Source: used, unused.Source: unused}))
				before := readTestFile(t, filename)
				source := "app.cpp"
				if command == "test" {
					source = "app.test.cpp"
				}
				writeProjectTestFile(t, project, source, "#include <github.com/demo/used/value.h>\n#if 0\n#include <github.com/demo/unused/value.h>\n#endif\nint main() { return value() == 7 ? 0 : 1; }\n")
				args := []string{command, source, "-v", "--no-color"}
				if locked {
					args = append(args, "--locked")
				}
				for pass := 0; pass < 2; pass++ {
					out := runDiscoveryCommand(t, configuration, args...)
					if pass == 1 && (strings.Count(out, "Parsing "+source) != 1 || !strings.Contains(out, "Parsing "+source+" (CACHED)")) {
						t.Fatalf("warm selection missed cache:\n%s", out)
					}
				}
				want := "/v1/snapshot " + used.Source + "@" + used.Commit
				if got := proxy.log(); got != want {
					t.Fatalf("downloaded repositories outside source closure: %s", got)
				}
				_, record := readDiscoveryRecord(t, configuration, project, source, command)
				if len(record.Pins) != 1 || record.Pins[used.Source] != used {
					t.Fatalf("cached unused pins: %#v", record.Pins)
				}
				// Removing an include must drop the old selection, even though its
				// pin and snapshot still exist. An unrelated corrupt snapshot is inert.
				writeProjectTestFile(t, configuration.root, "snapshot/"+used.Source+"/@"+used.Commit+"/value.h", "corrupted\n")
				writeProjectTestFile(t, project, source, "int main() { return 0; }\n")
				runDiscoveryCommand(t, configuration, args...)
				_, record = readDiscoveryRecord(t, configuration, project, source, command)
				if len(record.Pins) != 0 || readTestFile(t, filename) != before || proxy.log() != want {
					t.Fatal("unused pins were loaded or the project record changed")
				}
				// Once the include is active again, checksum validation is mandatory.
				writeProjectTestFile(t, project, source, "#include <github.com/demo/used/value.h>\nint main() { return 0; }\n")
				if _, _, err := runProjectTestCommand(configuration, args...); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
					t.Fatalf("active snapshot was not validated: %v", err)
				}
				owner, _ := localProjectRoot(configuration.root, configuration.env, project)
				if _, err := os.Lstat(filepath.Join(owner, "include", unused.Source)); !os.IsNotExist(err) {
					t.Fatalf("unused repository entered the include view: %v", err)
				}
			})
		}
	}
}

func TestRecordedRecipeSelectionIncludesYAMLDependencies(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	pins := make(map[string]repositoryPin)
	for _, name := range []string{"image", "compression", "unused"} {
		pin, archive := inheritedTestSnapshot(t, "github.com/demo/"+name, "main", firstCommit, map[string]string{"library.h": "#pragma once\n"})
		pins[pin.Source] = pin
		if name != "unused" {
			proxy.add(pin, archive)
		}
	}
	configuration := projectTestConfiguration(t)
	configuration.cc = "compiler-must-not-run"
	project := t.TempDir()
	withWorkingDirectory(t, project)
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, pins))
	before := readTestFile(t, filename)
	for _, name := range []string{"image", "compression"} {
		recipe := strings.Replace(validLibraryRecipeYAML(), "github.com/owner/library", "github.com/demo/"+name, 1)
		if name == "image" {
			recipe += "dependencies: [compression.hard]\n"
		}
		writeProjectTestFile(t, project, name+".hard", recipe)
	}
	writeProjectTestFile(t, project, "image.hard.h", "#pragma once\n#include <library.h>\n")
	writeProjectTestFile(t, project, "image.test.cpp", "#include \"image.hard.h\"\n")
	for _, extra := range [][]string{nil, nil, {"--no-cache"}} {
		args := append([]string{"fetch", "image.test.cpp", "--locked", "-v", "--no-color"}, extra...)
		runDiscoveryCommand(t, configuration, args...)
	}
	if got := proxy.log(); strings.Contains(got, "unused") || strings.Contains(got, "resolve") || strings.Count(got, "/v1/snapshot ") != 2 {
		t.Fatalf("wrong recipe closure downloads: %s", got)
	}
	_, record := readDiscoveryRecord(t, configuration, project, "image.test.cpp", "fetch")
	if len(record.Pins) != 2 || readTestFile(t, filename) != before {
		t.Fatalf("wrong recipe closure or changed pins: %#v", record.Pins)
	}
	// A descriptor change invalidates discovery even if the C++ source is unchanged.
	writeProjectTestFile(t, project, "image.hard", strings.Replace(validLibraryRecipeYAML(), "github.com/owner/library", "github.com/demo/image", 1))
	runDiscoveryCommand(t, configuration, "fetch", "image.test.cpp", "--locked", "-v")
	_, record = readDiscoveryRecord(t, configuration, project, "image.test.cpp", "fetch")
	if len(record.Pins) != 1 || record.Pins["github.com/demo/image"] != pins["github.com/demo/image"] {
		t.Fatalf("removed recipe dependency remained selected: %#v", record.Pins)
	}
}

func TestExplicitUpdateDownloadsUnreferencedRepository(t *testing.T) {
	proxy := newInheritanceTestProxy(t)
	old, _ := inheritedTestSnapshot(t, "github.com/demo/value", "release", firstCommit, map[string]string{"value.h": "// old\n"})
	updated, archive := inheritedTestSnapshot(t, old.Source, "next", nextCommit, map[string]string{"value.h": "// updated\n"})
	proxy.add(updated, archive)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{old.Source: old}))
	writeProjectTestFile(t, project, "local.cpp", "int main() { return 0; }\n")
	configuration := projectTestConfiguration(t)
	runDiscoveryCommand(t, configuration, "fetch", "local.cpp", "-v")
	if got := proxy.log(); got != "" {
		t.Fatalf("unused repository was downloaded: %s", got)
	}
	runDiscoveryCommand(t, configuration, "fetch", "local.cpp", "--update="+old.Source+"@next", "-v")
	file, err := readProjectFile(filename)
	if err != nil || file.Repositories[old.Source] != updated {
		t.Fatalf("update did not record verified contents: %#v, %v", file, err)
	}
	if got := proxy.log(); got != "/v1/resolve "+old.Source+"@\n/v1/snapshot "+old.Source+"@"+updated.Commit {
		t.Fatalf("unexpected explicit update requests: %s", got)
	}
}
