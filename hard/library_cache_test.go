package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func sharedLibraryFixture(t *testing.T) (configuration, repositoryPin) {
	t.Helper()
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake not installed")
	}
	proxy := newInheritanceTestProxy(t)
	vendor, archive := inheritedTestSnapshot(t, "github.com/demo/vendor", "release", secondCommit, map[string]string{
		"CMakeLists.txt": "cmake_minimum_required(VERSION 3.16)\nproject(vendor LANGUAGES CXX)\nif(\"$ENV{HARD_LIBRARY_TEST_FAIL}\" STREQUAL \"1\")\nmessage(FATAL_ERROR \"injected build failure\")\nendif()\nadd_library(vendor STATIC vendor.cpp)\ninstall(TARGETS vendor ARCHIVE DESTINATION lib)\ninstall(FILES vendor.h DESTINATION include)\n",
		"vendor.cpp":     "int vendor_value() { return 7; }\n",
		"vendor.h":       "#pragma once\nint vendor_value();\n",
	})
	proxy.add(vendor, archive)
	t.Setenv("HARD_LIBRARY_TEST_FAIL", "0")
	return projectTestConfiguration(t), vendor
}

func sharedLibraryManager(t *testing.T, configuration configuration, vendor repositoryPin, noCache bool, stdout io.Writer) (*libraryManager, string) {
	t.Helper()
	project := t.TempDir()
	filename := writeProjectTestFile(t, project, projectFilename, inheritedTestYAML(t, map[string]repositoryPin{vendor.Source: vendor}))
	header := writeProjectTestFile(t, project, "vendor.hard.h", "#pragma once\n")
	writeProjectTestFile(t, project, "vendor.hard", "version: 1\nsource: github.com/demo/vendor\nbuild_system: cmake\nsource_directory: .\nsource_include_directories: [.]\ninclude_directories: [include]\nstatic_libraries: [lib/libvendor.a]\n")
	file, err := readProjectFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	providerConfiguration, err := loadRepositoryConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	session, err := newDependencySession(file, configuration.root, projectOptions{locked: true}, newRepositoryProvider(providerConfiguration))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.close)
	view, err := session.view(configuration, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := newGitHubSnapshotResolver(view, nil)
	resolver.session = session
	cache, err := newArtifactCache(!noCache)
	if err != nil {
		t.Fatal(err)
	}
	progress := newProgressBar(stdout, -1, true, false, true)
	return newLibraryManager(view, configuration.env, configuration.cc, 1, true, noCache, project, resolver, cache, progress, io.Discard), header
}

func prepareSharedLibrary(t *testing.T, manager *libraryManager, header string) libraryArtifact {
	t.Helper()
	prepared, err := manager.prepareHeaders([]string{header})
	if err != nil || len(prepared) != 1 {
		t.Fatalf("prepare library: %v, %#v", err, prepared)
	}
	return prepared[0]
}

func TestLibraryPackageSharedAcrossPinnedProjects(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	var artifacts []libraryArtifact
	for index := 0; index < 2; index++ {
		var progress bytes.Buffer
		manager, header := sharedLibraryManager(t, configuration, vendor, false, &progress)
		artifacts = append(artifacts, prepareSharedLibrary(t, manager, header))
		if strings.Contains(progress.String(), "Building "+vendor.Source+" (CACHED)") != (index == 1) {
			t.Fatalf("wrong package cache state for project %d: %s", index, progress.String())
		}
		if _, err := os.Stat(filepath.Join(manager.root, "env", configuration.env, "library")); !os.IsNotExist(err) {
			t.Fatalf("project-local library cache exists: %v", err)
		}
	}
	if artifacts[0].archives[0] != artifacts[1].archives[0] {
		t.Fatalf("identical libraries were rebuilt per project:\n%s\n%s", artifacts[0].archives[0], artifacts[1].archives[0])
	}
	if !pathWithin(filepath.Join(configuration.root, "project", "host", filepath.FromSlash(vendor.Source), "package"), artifacts[0].archives[0]) {
		t.Fatalf("package not in shared cache: %s", artifacts[0].archives[0])
	}
}

func TestLibraryPackageParallelProjects(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	managers := make([]*libraryManager, 2)
	headers := make([]string, 2)
	for index := range managers {
		managers[index], headers[index] = sharedLibraryManager(t, configuration, vendor, false, io.Discard)
	}
	var wait sync.WaitGroup
	start := make(chan struct{})
	artifacts := make([][]libraryArtifact, 2)
	failures := make([]error, 2)
	for index := range managers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			artifacts[index], failures[index] = managers[index].prepareHeaders([]string{headers[index]})
		}(index)
	}
	close(start)
	wait.Wait()
	for index, err := range failures {
		if err != nil || len(artifacts[index]) != 1 {
			t.Fatalf("parallel project %d: %v", index, err)
		}
	}
	if artifacts[0][0].archives[0] != artifacts[1][0].archives[0] {
		t.Fatalf("parallel builds produced different generations: %#v", artifacts)
	}
	packageRoot, err := libraryPackageRoot(configuration.root, configuration.env, vendor.Source, artifacts[0][0].key)
	if err != nil {
		t.Fatal(err)
	}
	generations, err := filepath.Glob(filepath.Join(packageRoot, "generation-*"))
	if err != nil || len(generations) != 1 {
		t.Fatalf("parallel generation count: %v, %v", generations, err)
	}
}

func TestLibraryPackageNoCachePreservesExistingConsumers(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	prepare := func(noCache bool) libraryArtifact {
		t.Helper()
		manager, header := sharedLibraryManager(t, configuration, vendor, noCache, io.Discard)
		return prepareSharedLibrary(t, manager, header)
	}
	first := prepare(false)
	oldArchive := first.archives[0]
	oldContents, err := os.ReadFile(oldArchive)
	if err != nil {
		t.Fatal(err)
	}
	second := prepare(true)
	if first.key != second.key || oldArchive == second.archives[0] {
		t.Fatalf("forced build did not publish a new generation: %#v, %#v", first, second)
	}
	if third := prepare(false); third.archives[0] != second.archives[0] {
		t.Fatal("repeat did not use the new generation")
	}
	// An existing consumer can still link and run using the old absolute paths.
	consumer := t.TempDir()
	main := writeProjectTestFile(t, consumer, "main.cpp", "#include <vendor.h>\nint main() { return vendor_value() == 7 ? 0 : 1; }\n")
	binary := filepath.Join(consumer, "main")
	if output, err := exec.Command("c++", first.cflags[0], main, oldArchive, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("old consumer link: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("old consumer execution: %v\n%s", err, output)
	}
	t.Setenv("HARD_LIBRARY_TEST_FAIL", "1")
	manager, header := sharedLibraryManager(t, configuration, vendor, true, io.Discard)
	if _, err := manager.prepareHeaders([]string{header}); err == nil {
		t.Fatal("forced build unexpectedly succeeded")
	}
	packageRoot, err := libraryPackageRoot(configuration.root, configuration.env, vendor.Source, first.key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packageRoot, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("failed build retained an eligible manifest: %v", err)
	}
	current, err := os.ReadFile(oldArchive)
	if err != nil || !bytes.Equal(oldContents, current) {
		t.Fatalf("forced/failed build changed an existing consumer's archive: %v", err)
	}
	generations, err := filepath.Glob(filepath.Join(packageRoot, "generation-*"))
	if err != nil || len(generations) != 2 {
		t.Fatalf("failed build left a partial generation or removed a published one: %v, %v", generations, err)
	}
	t.Setenv("HARD_LIBRARY_TEST_FAIL", "0")
	fourth := prepare(false)
	if fourth.archives[0] == second.archives[0] {
		t.Fatal("reused an invalidated manifest after failed forced build")
	}
	writeProjectTestFile(t, filepath.Dir(fourth.archives[0]), filepath.Base(fourth.archives[0]), "corrupted archive")
	if repaired := prepare(false); repaired.archives[0] == fourth.archives[0] {
		t.Fatal("accepted a corrupted shared package")
	}
}

func TestLibraryPackageEnvironmentAndRecipeIsolation(t *testing.T) {
	configuration, vendor := sharedLibraryFixture(t)
	manager, header := sharedLibraryManager(t, configuration, vendor, false, io.Discard)
	first := prepareSharedLibrary(t, manager, header)
	configuration.env = "other-toolchain"
	manager, header = sharedLibraryManager(t, configuration, vendor, false, io.Discard)
	second := prepareSharedLibrary(t, manager, header)
	if first.archives[0] == second.archives[0] {
		t.Fatal("reused a package from another HARD_ENV")
	}
	configuration.env = "host"
	manager, header = sharedLibraryManager(t, configuration, vendor, false, io.Discard)
	descriptor := strings.TrimSuffix(header, ".h")
	contents := readTestFile(t, descriptor)
	contents = strings.Replace(contents, "source_directory: .", "source_directory: .\nconfigure_arguments: [-DCMAKE_BUILD_TYPE=Debug]", 1)
	writeProjectTestFile(t, filepath.Dir(descriptor), filepath.Base(descriptor), contents)
	third := prepareSharedLibrary(t, manager, header)
	if first.key == third.key || first.archives[0] == third.archives[0] {
		t.Fatal("reused a package with different recipe arguments")
	}
}

func TestLibraryPackageManifestGenerationValidation(t *testing.T) {
	root := t.TempDir()
	generation := "generation-test"
	install := filepath.Join(root, generation, "install")
	writeProjectTestFile(t, install, "lib/library.a", "archive")
	files, err := libraryInstallManifestFiles(install)
	if err != nil {
		t.Fatal(err)
	}
	manifest := libraryManifest{Version: libraryManifestVersion, Input: "input", Directory: generation, Files: files}
	for _, directory := range []string{generation, "", "../outside", "generation-../outside", "generation-\\outside", "/generation-absolute", "generation-missing"} {
		manifest.Directory = directory
		contents, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		filename := writeProjectTestFile(t, root, "manifest.json", string(contents))
		got, hit, err := libraryManifestHit(filename, root, "input")
		if err != nil || hit != (directory == generation) || hit && got != install {
			t.Fatalf("directory %q: %q, %t, %v", directory, got, hit, err)
		}
	}
	if err := os.Symlink(filepath.Join(root, generation), filepath.Join(root, "generation-link")); err != nil {
		t.Fatal(err)
	}
	manifest.Directory = "generation-link"
	contents, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	filename := writeProjectTestFile(t, root, "manifest.json", string(contents))
	if _, _, err := libraryManifestHit(filename, root, "input"); err == nil {
		t.Fatal("accepted a symlink as a generation directory")
	}
}
