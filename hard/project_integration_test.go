package main

import (
	"archive/tar"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestProjectRecipeAndVendorAreBothPinned(t *testing.T) {
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake not installed")
	}
	recipe := `/* hard.recipe.v1
source: github.com/demo/vendor
build_system: cmake
source_directory: .
source_include_directories: [include]
include_directories: [include]
static_libraries: [lib/libvendor.a]
*/
#pragma once
#include <vendor.h>
`
	archives := map[string][]byte{
		"github.com/hard-build/recipe": githubTestArchive(t, []githubTestArchiveEntry{{name: "recipe/vendor.hard.h", typeflag: tar.TypeReg, mode: 0o644, contents: recipe}}),
		"github.com/demo/vendor": githubTestArchive(t, []githubTestArchiveEntry{
			{name: "vendor/CMakeLists.txt", typeflag: tar.TypeReg, mode: 0o644, contents: "cmake_minimum_required(VERSION 3.16)\nproject(vendor LANGUAGES CXX)\nadd_library(vendor STATIC vendor.cpp)\ninstall(TARGETS vendor ARCHIVE DESTINATION lib)\ninstall(FILES include/vendor.h DESTINATION include)\n"},
			{name: "vendor/vendor.cpp", typeflag: tar.TypeReg, mode: 0o644, contents: "int vendor_value() { return 7; }\n"},
			{name: "vendor/include/vendor.h", typeflag: tar.TypeReg, mode: 0o644, contents: "#pragma once\nint vendor_value();\n"},
		}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/resolve":
			_ = json.NewEncoder(response).Encode(map[string]string{"commit": firstCommit, "ref": "main"})
		case "/v1/snapshot":
			_, _ = response.Write(archives[request.URL.Query().Get("source")])
		default:
			http.Error(response, "unexpected request", 500)
		}
	}))
	defer server.Close()
	t.Setenv("HARD_PROXY", server.URL)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "app.cpp", "#include <recipe/vendor.hard.h>\nint main() { return vendor_value() == 7 ? 0 : 41; }\n")
	configuration := projectTestConfiguration(t)
	if out, diagnostics, err := runProjectTestCommand(configuration, "fetch", "--lock"); err != nil {
		t.Fatalf("recipe fetch: %v\n%s\n%s", err, out, diagnostics)
	}
	file, err := readProjectFile(filepath.Join(project, projectFilename))
	if err != nil || len(file.Repositories) != 2 {
		t.Fatalf("recipe and vendor records: %v, %#v", err, file)
	}
	if paths, _ := filepath.Glob(filepath.Join(configuration.root, "project", "*", "env")); len(paths) != 0 {
		t.Fatalf("fetch built a package: %v", paths)
	}
	for pass := 0; pass < 2; pass++ {
		out, diagnostics, err := runProjectTestCommand(configuration, "run", "--locked", "-v")
		if err != nil {
			t.Fatalf("recipe run: %v\n%s\n%s", err, out, diagnostics)
		}
		if pass == 1 && !strings.Contains(out, "Building github.com/demo/vendor (CACHED)") {
			t.Fatalf("vendor cache not reused: %s", out)
		}
	}
}

func TestConcurrentProjectAdditionsAreNotLost(t *testing.T) {
	server, _ := newRepositoryTestProxy(t)
	t.Setenv("HARD_PROXY", server.URL)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, "first.cpp", "#include <github.com/demo/first/first.h>\n")
	writeProjectTestFile(t, project, "second.cpp", "#include <github.com/demo/other/first.h>\n")
	configuration := projectTestConfiguration(t)
	var wait sync.WaitGroup
	failures := make([]error, 2)
	for index, path := range []string{"first.cpp", "second.cpp"} {
		wait.Add(1)
		go func(index int, path string) {
			defer wait.Done()
			_, _, failures[index] = runProjectTestCommand(configuration, "fetch", "--lock", path)
		}(index, path)
	}
	wait.Wait()
	for _, err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	file, err := readProjectFile(filepath.Join(project, projectFilename))
	if err != nil || len(file.Repositories) != 3 {
		t.Fatalf("lost concurrent addition: %v, %#v", err, file)
	}
}

func TestProjectWrapperKeepsWorkingDirectoryAndIgnoresHostConfiguration(t *testing.T) {
	project := t.TempDir()
	writeProjectTestFile(t, project, projectFilename, "version: 1\n")
	writeProjectTestFile(t, project, "src/app.cpp", "")
	workingDirectory := filepath.Join(project, "src")
	hardRoot := t.TempDir()
	// Even an unavailable host configuration must not be inspected or mounted.
	configuration := filepath.Join(t.TempDir(), "missing-corporate.yaml")
	wrapper := installWrapperAtPrefix(t, filepath.Join(t.TempDir(), "portable"))
	for _, target := range []string{"linux64", "windows64", "docker://example/toolchain:fixed"} {
		for _, proxy := range []string{"https://dependencies.example", ""} {
			t.Run(target+"/proxy="+proxy, func(t *testing.T) {
				log := filepath.Join(t.TempDir(), "docker.log")
				bin := installFakeWrapperDocker(t, log)
				command := exec.Command(wrapper, "--target="+target, "build", "--locked")
				command.Dir = workingDirectory
				command.Env = wrapperTestEnvironment(map[string]string{
					"PATH":              bin + string(os.PathListSeparator) + os.Getenv("PATH"),
					"HARD_ROOT":         hardRoot,
					"HARD_CONFIG":       configuration,
					"HARD_PROXY":        proxy,
					"HARD_AUTH_FIXTURE": "fixture-secret-not-in-argv",
					"HARD_CC":           "must-not-be-forwarded",
				})
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("wrapper: %v, %s", err, output)
				}
				arguments := readWrapperArguments(t, log)
				for _, wanted := range []string{
					"type=bind,source=" + hardRoot + ",target=/hard",
					"type=bind,source=" + workingDirectory + ",target=" + workingDirectory,
					"--locked",
				} {
					if !containsWrapperArgument(arguments, wanted) {
						t.Fatalf("missing %s in %v", wanted, arguments)
					}
				}
				mounts, environment := 0, 0
				for _, argument := range arguments {
					if argument == "--mount" {
						mounts++
					}
					if argument == "--env" {
						environment++
					}
				}
				if mounts != 2 || environment != 0 {
					t.Fatalf("unexpected extra mounts or environment: %v", arguments)
				}
				joined := strings.Join(arguments, " ")
				for _, forbidden := range []string{configuration, "HARD_CONFIG", "HARD_PROXY", "https://dependencies.example", "HARD_AUTH_", "fixture-secret-not-in-argv", "HARD_CC"} {
					if strings.Contains(joined, forbidden) {
						t.Fatalf("wrapper forwarded host configuration or credentials: %s", forbidden)
					}
				}
			})
		}
	}
}

func TestPinnedGoogleTestExecutionAndCache(t *testing.T) {
	if err := exec.Command("pkg-config", "--exists", googleTestPackage).Run(); err != nil {
		t.Skip("GoogleTest not installed")
	}
	server, _ := newRepositoryTestProxy(t)
	t.Setenv("HARD_PROXY", server.URL)
	project := t.TempDir()
	withWorkingDirectory(t, project)
	writeProjectTestFile(t, project, projectFilename, "version: 1\nrepositories: {}\n")
	writeProjectTestFile(t, project, "value.test.cpp", "#include <gtest/gtest.h>\n#include <github.com/demo/first/first.h>\nTEST(Dependency, Pinned) { EXPECT_EQ(dependency_value(), 1); }\n")
	configuration := projectTestConfiguration(t)
	if out, diagnostics, err := runProjectTestCommand(configuration, "test"); err != nil {
		t.Fatalf("pinned test: %v\n%s\n%s", err, out, diagnostics)
	}
	out, diagnostics, err := runProjectTestCommand(configuration, "test", "--locked", "-v")
	if err != nil || !strings.Contains(out, "Testing value.test (CACHED)") {
		t.Fatalf("cached pinned test: %v\n%s\n%s", err, out, diagnostics)
	}
}
