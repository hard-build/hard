package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

const (
	libraryManifestVersion = 3
)

type libraryRecipe struct {
	Version                  int      `yaml:"version"`
	Dependencies             []string `yaml:"dependencies"`
	Source                   string   `yaml:"source"`
	BuildSystem              string   `yaml:"build_system"`
	SourceDirectory          string   `yaml:"source_directory"`
	ConfigureArguments       []string `yaml:"configure_arguments"`
	SourceIncludeDirectories []string `yaml:"source_include_directories"`
	IncludeDirectories       []string `yaml:"include_directories"`
	StaticLibraries          []string `yaml:"static_libraries"`
}

type libraryArtifact struct {
	key             string
	header          string
	cflags          []string
	archives        []string
	prefix          string
	sourceDirectory string
	dependencies    []string
}

type libraryManifest struct {
	Version   int         `json:"version"`
	Input     string      `json:"input"`
	Directory string      `json:"directory"`
	Files     []cacheFile `json:"files"`
}

type libraryManager struct {
	root             string
	environment      string
	compiler         string
	jobs             int
	build            bool
	noCache          bool
	workingDirectory string
	githubResolver   *githubSnapshotResolver
	cache            *artifactCache
	progress         *progressBar
	stderr           io.Writer

	mutex   sync.Mutex
	results map[string]libraryArtifact
}

func newLibraryManager(
	root string,
	environment string,
	compiler string,
	jobs int,
	build bool,
	noCache bool,
	workingDirectory string,
	githubResolver *githubSnapshotResolver,
	cache *artifactCache,
	progress *progressBar,
	stderr io.Writer,
) *libraryManager {
	if progress != nil {
		progress.displayPath = func(path string) string { return buildParsingDisplayPath(root, path, workingDirectory) }
	}
	return &libraryManager{
		root:             root,
		environment:      environment,
		compiler:         compiler,
		jobs:             jobs,
		build:            build,
		noCache:          noCache,
		workingDirectory: workingDirectory,
		githubResolver:   githubResolver,
		cache:            cache,
		progress:         progress,
		stderr:           stderr,
		results:          make(map[string]libraryArtifact),
	}
}

func (manager *libraryManager) prepareDependencies(dependencies []string) ([]libraryArtifact, []string, error) {
	graph, headers, err := manager.discoverLibraries(clangAnalysis{}, dependencies, nil)
	if err != nil {
		return nil, nil, err
	}
	artifacts, err := manager.prepareGraph(graph)
	return artifacts, headers, err
}

func (manager *libraryManager) prepareHeaders(headers []string) ([]libraryArtifact, error) {
	graph, _, err := manager.discoverLibraries(clangAnalysis{}, headers, nil)
	if err != nil {
		return nil, err
	}
	return manager.prepareGraph(graph)
}

func (manager *libraryManager) prepareRecipe(
	header string,
	headerContents []byte,
	recipe libraryRecipe,
	dependencies ...libraryArtifact,
) (libraryArtifact, error) {
	repository, err := libraryRecipeRepository(recipe.Source)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("library recipe %s: %w", header, err)
	}
	if manager.githubResolver == nil {
		return libraryArtifact{}, fmt.Errorf("library recipe %s requires GitHub dependency %s", header, recipe.Source)
	}
	if err := manager.githubResolver.ensure(repository, header); err != nil {
		return libraryArtifact{}, err
	}
	sourceRoot, err := githubRepositoryDirectory(manager.root, repository.owner, repository.name)
	if session := manager.githubResolver.session; session != nil {
		sourceRoot = session.selected[recipe.Source]
	}
	if err != nil {
		return libraryArtifact{}, err
	}
	sourceRoot, err = filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return libraryArtifact{}, err
	}
	sourceDirectory := filepath.Join(sourceRoot, filepath.FromSlash(recipe.SourceDirectory))
	if manager.build {
		return manager.buildRecipe(header, headerContents, recipe, sourceRoot, sourceDirectory, dependencies...)
	}

	cflags := make([]string, 0, len(recipe.SourceIncludeDirectories))
	for _, directory := range recipe.SourceIncludeDirectories {
		path := filepath.Join(sourceDirectory, filepath.FromSlash(directory))
		if err := requireLibraryDirectory(path, "source include"); err != nil {
			return libraryArtifact{}, fmt.Errorf("library recipe %s: %w", header, err)
		}
		cflags = append(cflags, "-I"+path)
	}
	return libraryArtifact{
		key:             header,
		header:          header,
		cflags:          cflags,
		sourceDirectory: sourceDirectory,
	}, nil
}

func libraryOwnsHeader(header string, artifacts []libraryArtifact) bool {
	for _, artifact := range artifacts {
		for _, root := range []string{artifact.sourceDirectory, artifact.prefix} {
			if root != "" && pathWithin(root, header) {
				return true
			}
		}
	}
	return false
}

func (manager *libraryManager) buildRecipe(
	header string,
	headerContents []byte,
	recipe libraryRecipe,
	sourceRoot string,
	sourceDirectory string,
	dependencies ...libraryArtifact,
) (libraryArtifact, error) {
	if manager.cache == nil {
		return libraryArtifact{}, errors.New("library build requires an artifact cache")
	}
	if err := requireLibraryDirectory(sourceDirectory, "source"); err != nil {
		return libraryArtifact{}, fmt.Errorf("library recipe %s: %w", header, err)
	}
	cmake, err := resolveLibraryTool("cmake", manager.workingDirectory)
	if err != nil {
		return libraryArtifact{}, err
	}
	compiler, err := resolveLibraryTool(manager.compiler, manager.workingDirectory)
	if err != nil {
		return libraryArtifact{}, err
	}
	compilerFingerprint, err := manager.cache.toolFingerprint(compiler, manager.workingDirectory)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("fingerprint library compiler %s: %w", compiler, err)
	}
	inputs, sourceEntries, err := librarySourceInputs(sourceRoot)
	if err != nil {
		return libraryArtifact{}, err
	}
	// Recipe bytes already participate below. Their project-local filename
	// does not affect CMake, whose working directory is the vendor snapshot.
	arguments := []string{
		"recipe:" + string(headerContents),
		"compiler-path:" + compilerFingerprint.Path,
		"compiler-digest:" + compilerFingerprint.Digest,
	}
	for _, dependency := range dependencies {
		arguments = append(arguments, "dependency:"+dependency.key)
	}
	arguments = append(arguments, sourceEntries...)
	arguments = append(arguments, "source-directory:"+recipe.SourceDirectory)
	for _, argument := range recipe.ConfigureArguments {
		arguments = append(arguments, "configure:"+argument)
	}
	for _, directory := range recipe.IncludeDirectories {
		arguments = append(arguments, "include:"+directory)
	}
	for _, library := range recipe.StaticLibraries {
		arguments = append(arguments, "archive:"+library)
	}
	input, err := manager.cache.actionFingerprintWithWorkingDirectory(
		"library-cmake-v3",
		cmake,
		arguments,
		inputs,
		manager.workingDirectory,
		sourceDirectory,
	)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("fingerprint library %s: %w", recipe.Source, err)
	}
	cacheRoot := manager.root
	if manager.githubResolver != nil && manager.githubResolver.session != nil {
		cacheRoot = manager.githubResolver.session.root
	}
	packageRoot, err := libraryPackageRoot(cacheRoot, manager.environment, recipe.Source, input)
	if err != nil {
		return libraryArtifact{}, err
	}
	cacheRoot, err = filepath.Abs(cacheRoot)
	if err != nil {
		return libraryArtifact{}, err
	}
	relativePackage, err := filepath.Rel(cacheRoot, packageRoot)
	if err != nil {
		return libraryArtifact{}, err
	}
	packageRoot, err = ensureCacheDirectory(cacheRoot, relativePackage)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("create library cache directory: %w", err)
	}
	info, err := os.Lstat(packageRoot)
	if err != nil {
		return libraryArtifact{}, err
	}
	if !info.IsDir() {
		return libraryArtifact{}, fmt.Errorf("library cache is not a directory: %s", packageRoot)
	}
	manifestPath := filepath.Join(packageRoot, "manifest.json")
	lock, err := lockProjectDirectory(manifestPath)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("lock library cache %s: %w", packageRoot, err)
	}
	defer lock.Close()
	if !manager.noCache {
		installDirectory, cached, err := libraryManifestHit(manifestPath, packageRoot, input)
		if err != nil {
			return libraryArtifact{}, err
		}
		if cached {
			if manager.progress != nil {
				manager.progress.updateStep("Building " + recipe.Source + " (CACHED)")
			}
			return libraryInstalledArtifact(header, recipe, installDirectory, input)
		}
	}
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return libraryArtifact{}, fmt.Errorf("invalidate library manifest %s: %w", manifestPath, err)
	}
	// Never rebuild in place: another process may already be compiling or
	// linking against an older generation after releasing this package lock.
	generation, err := os.MkdirTemp(packageRoot, "generation-")
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("create library generation: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(generation)
		}
	}()
	installDirectory := filepath.Join(generation, "install")
	buildDirectory := filepath.Join(generation, "build")
	if err := os.MkdirAll(buildDirectory, 0o755); err != nil {
		return libraryArtifact{}, fmt.Errorf("create library build directory %s: %w", buildDirectory, err)
	}

	configure := []string{
		"-S", sourceDirectory,
		"-B", buildDirectory,
		"-DCMAKE_CXX_COMPILER=" + compiler,
		"-DCMAKE_INSTALL_PREFIX=" + installDirectory,
	}
	configure = append(configure, recipe.ConfigureArguments...)
	if err := manager.runCMake(
		cmake,
		"Configuring "+recipe.Source,
		configure,
		sourceDirectory,
		dependencies...,
	); err != nil {
		return libraryArtifact{}, err
	}
	if err := manager.runCMake(
		cmake,
		"Building "+recipe.Source,
		[]string{"--build", buildDirectory, "--parallel", fmt.Sprintf("%d", manager.jobs)},
		sourceDirectory,
		dependencies...,
	); err != nil {
		return libraryArtifact{}, err
	}
	if err := manager.runCMake(
		cmake,
		"Installing "+recipe.Source,
		[]string{"--install", buildDirectory},
		sourceDirectory,
		dependencies...,
	); err != nil {
		return libraryArtifact{}, err
	}
	artifact, err := libraryInstalledArtifact(header, recipe, installDirectory, input)
	if err != nil {
		return libraryArtifact{}, err
	}
	files, err := libraryInstallManifestFiles(installDirectory)
	if err != nil {
		return libraryArtifact{}, err
	}
	manifest := libraryManifest{Version: libraryManifestVersion, Input: input, Directory: filepath.Base(generation), Files: files}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("encode library manifest %s: %w", manifestPath, err)
	}
	if err := writeCacheRecord(manifestPath, append(encoded, '\n')); err != nil {
		return libraryArtifact{}, fmt.Errorf("write library manifest %s: %w", manifestPath, err)
	}
	published = true
	return artifact, nil
}

func (manager *libraryManager) runCMake(
	cmake string,
	step string,
	arguments []string,
	workingDirectory string,
	dependencies ...libraryArtifact,
) error {
	if manager.progress != nil {
		manager.progress.updateStep(step)
	}
	command := exec.Command(cmake, arguments...)
	command.Dir = workingDirectory
	command.Env = environmentWithoutVariable(os.Environ(), "CXXFLAGS")
	command.Env = append(command.Env, "CXXFLAGS=")
	command.Env = libraryBuildEnvironment(command.Env, dependencies)
	var diagnostics bytes.Buffer
	command.Stdout = &diagnostics
	command.Stderr = &diagnostics
	if err := command.Run(); err != nil {
		if diagnostics.Len() != 0 && manager.stderr != nil {
			if _, writeErr := io.Copy(manager.stderr, &diagnostics); writeErr != nil {
				return errors.Join(fmt.Errorf("run cmake: %w", err), fmt.Errorf("write cmake diagnostics: %w", writeErr))
			}
		}
		return fmt.Errorf("run cmake %s: %w", strings.Join(arguments, " "), err)
	}
	return nil
}

func parseLibraryRecipe(contents []byte) (libraryRecipe, bool, error) {
	document := string(contents)
	if err := validateLibraryYAML([]byte(document)); err != nil {
		return libraryRecipe{}, false, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(document))
	decoder.KnownFields(true)
	var recipe libraryRecipe
	if err := decoder.Decode(&recipe); err != nil {
		return libraryRecipe{}, false, fmt.Errorf("decode YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return libraryRecipe{}, false, errors.New("multiple YAML documents are not allowed")
		}
		return libraryRecipe{}, false, fmt.Errorf("decode YAML: %w", err)
	}
	if err := validateLibraryRecipe(recipe); err != nil {
		return libraryRecipe{}, false, err
	}
	return recipe, true, nil
}

func validateLibraryYAML(contents []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return errors.New("YAML document has no root value")
	}
	if err := validateLibraryYAMLNode(document.Content[0]); err != nil {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple YAML documents are not allowed")
		}
		return fmt.Errorf("decode YAML: %w", err)
	}
	return nil
}

func validateLibraryYAMLNode(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Alias != nil {
		return errors.New("YAML aliases are not allowed")
	}
	if node.Anchor != "" {
		return errors.New("YAML anchors are not allowed")
	}
	allowedTags := map[string]bool{
		"!!map": true, "!!seq": true, "!!str": true, "!!null": true,
		"!!bool": true, "!!int": true, "!!float": true, "!!timestamp": true,
	}
	if node.Tag != "" && !allowedTags[node.Tag] {
		return fmt.Errorf("YAML tag is not allowed: %s", node.Tag)
	}
	if node.Value == "<<" || node.Tag == "!!merge" {
		return errors.New("YAML merge keys are not allowed")
	}
	for _, child := range node.Content {
		if err := validateLibraryYAMLNode(child); err != nil {
			return err
		}
	}
	return nil
}

func validateLibraryRecipe(recipe libraryRecipe) error {
	if recipe.Version != 1 {
		return fmt.Errorf("unsupported recipe version %d; expected 1", recipe.Version)
	}
	for _, dependency := range recipe.Dependencies {
		if strings.TrimSpace(dependency) == "" || !strings.HasSuffix(dependency, ".hard") || strings.ContainsAny(dependency, "\x00\r\n") {
			return fmt.Errorf("invalid dependency %q; expected a .hard path", dependency)
		}
	}
	if _, err := libraryRecipeRepository(recipe.Source); err != nil {
		return err
	}
	if recipe.BuildSystem != "cmake" {
		return fmt.Errorf("unsupported build_system %q; expected cmake", recipe.BuildSystem)
	}
	if err := validateLibraryRelativePath(recipe.SourceDirectory, true); err != nil {
		return fmt.Errorf("invalid source_directory: %w", err)
	}
	for _, argument := range recipe.ConfigureArguments {
		if argument == "" {
			return errors.New("configure_arguments contains an empty argument")
		}
		upper := strings.ToUpper(argument)
		if strings.HasPrefix(upper, "-DCMAKE_CXX_COMPILER") ||
			strings.HasPrefix(upper, "-DCMAKE_INSTALL_PREFIX") {
			return fmt.Errorf("configure argument is managed by hard: %s", argument)
		}
	}
	for _, path := range recipe.SourceIncludeDirectories {
		if err := validateLibraryRelativePath(path, true); err != nil {
			return fmt.Errorf("invalid source_include_directories path %q: %w", path, err)
		}
	}
	for _, path := range recipe.IncludeDirectories {
		if err := validateLibraryRelativePath(path, true); err != nil {
			return fmt.Errorf("invalid include_directories path %q: %w", path, err)
		}
	}
	if len(recipe.StaticLibraries) == 0 {
		return errors.New("static_libraries must contain at least one path")
	}
	for _, path := range recipe.StaticLibraries {
		if err := validateLibraryRelativePath(path, false); err != nil {
			return fmt.Errorf("invalid static_libraries path %q: %w", path, err)
		}
	}
	return nil
}

func validateLibraryRelativePath(path string, allowCurrent bool) error {
	if path == "" {
		return errors.New("path is empty")
	}
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return errors.New("path is absolute")
	}
	clean := filepath.Clean(path)
	if clean != path && filepath.ToSlash(clean) != filepath.ToSlash(path) {
		return errors.New("path is not clean")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("path escapes its root")
	}
	if clean == "." && !allowCurrent {
		return errors.New("path names a directory")
	}
	return nil
}

func libraryRecipeRepository(source string) (githubRepository, error) {
	parts := strings.Split(filepath.ToSlash(source), "/")
	if len(parts) != 3 || parts[0] != "github.com" ||
		!validGitHubPathSegment(parts[1]) || !validGitHubPathSegment(parts[2]) {
		return githubRepository{}, fmt.Errorf("invalid source %q; expected github.com/<owner>/<repository>", source)
	}
	return githubRepository{owner: parts[1], name: parts[2]}, nil
}

func libraryPackageRoot(root, environment, source, fingerprint string) (string, error) {
	repository, err := libraryRecipeRepository(source)
	if err != nil {
		return "", err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("make HARD_ROOT absolute: %w", err)
	}
	libraryRoot, err := projectEnvironmentRoot(absoluteRoot, environment)
	if err != nil {
		return "", err
	}
	packageRoot := filepath.Join(
		libraryRoot,
		"github.com",
		repository.owner,
		repository.name,
		"package",
		fingerprint,
	)
	if !pathWithin(libraryRoot, packageRoot) {
		return "", fmt.Errorf("library package path escapes environment: %s", packageRoot)
	}
	return packageRoot, nil
}

func librarySourceInputs(root string) ([]string, []string, error) {
	inputs := make([]string, 0)
	entries := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		entries = append(entries, "source-entry:"+relative)
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries = append(entries, "source-symlink:"+relative+"="+filepath.ToSlash(target))
			inputs = append(inputs, path)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("library source contains unsupported file: %s", path)
		}
		inputs = append(inputs, path)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("inspect library source tree %s: %w", root, err)
	}
	sort.Strings(inputs)
	sort.Strings(entries)
	return inputs, entries, nil
}

func libraryInstalledArtifact(
	header string,
	recipe libraryRecipe,
	installDirectory string,
	key string,
) (libraryArtifact, error) {
	artifact := libraryArtifact{key: key, header: header, prefix: installDirectory}
	for _, directory := range recipe.IncludeDirectories {
		path := filepath.Join(installDirectory, filepath.FromSlash(directory))
		if err := requireLibraryDirectory(path, "installed include"); err != nil {
			return libraryArtifact{}, err
		}
		artifact.cflags = append(artifact.cflags, "-I"+path)
	}
	for _, library := range recipe.StaticLibraries {
		path := filepath.Join(installDirectory, filepath.FromSlash(library))
		info, err := os.Lstat(path)
		if err != nil {
			return libraryArtifact{}, fmt.Errorf("inspect installed static library %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return libraryArtifact{}, fmt.Errorf("installed static library is not a regular file: %s", path)
		}
		artifact.archives = append(artifact.archives, path)
	}
	return artifact, nil
}

func libraryManifestHit(manifestPath, packageRoot, input string) (string, bool, error) {
	contents, err := readRegularProjectFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read library manifest %s: %w", manifestPath, err)
	}
	var manifest libraryManifest
	if err := json.Unmarshal(contents, &manifest); err != nil ||
		manifest.Version != libraryManifestVersion || manifest.Input != input ||
		!strings.HasPrefix(manifest.Directory, "generation-") ||
		strings.ContainsAny(manifest.Directory, "/\\") {
		return "", false, nil
	}
	generation := filepath.Join(packageRoot, manifest.Directory)
	info, err := os.Lstat(generation)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("inspect library generation %s: %w", generation, err)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("library generation is not a directory: %s", generation)
	}
	installDirectory := filepath.Join(generation, "install")
	files, err := libraryInstallManifestFiles(installDirectory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if len(files) != len(manifest.Files) {
		return "", false, nil
	}
	for index := range files {
		if files[index] != manifest.Files[index] {
			return "", false, nil
		}
	}
	return installDirectory, true, nil
}

func libraryInstallManifestFiles(root string) ([]cacheFile, error) {
	files := make([]cacheFile, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var digest string
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) {
				return fmt.Errorf("library install contains an absolute symlink: %s", path)
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			if !pathWithin(root, resolved) {
				return fmt.Errorf("library install symlink escapes prefix: %s", path)
			}
			targetDigest, err := digestInputFile(resolved)
			if err != nil {
				return err
			}
			digest = "symlink:" + filepath.ToSlash(target) + ":" + targetDigest
		} else {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("library install contains a non-regular file: %s", path)
			}
			digest, err = digestInputFile(path)
			if err != nil {
				return err
			}
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, cacheFile{Path: filepath.ToSlash(relative), Digest: digest})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect library install tree %s: %w", root, err)
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].Path < files[right].Path
	})
	return files, nil
}

func requireLibraryDirectory(path, kind string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect %s directory %s: %w", kind, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s path is not a directory: %s", kind, path)
	}
	return nil
}

func resolveLibraryTool(tool, workingDirectory string) (string, error) {
	path, err := exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("locate library build tool %s: %w", tool, err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve library build tool %s: %w", tool, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make library build tool absolute %s: %w", tool, err)
	}
	return filepath.Clean(path), nil
}

func environmentWithoutVariable(environment []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		if !strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func libraryCFlags(base []string, artifacts []libraryArtifact) []string {
	flags := append([]string(nil), base...)
	seen := make(map[string]struct{})
	for _, artifact := range artifacts {
		for _, flag := range artifact.cflags {
			if _, ok := seen[flag]; ok {
				continue
			}
			seen[flag] = struct{}{}
			flags = append(flags, flag)
		}
	}
	return flags
}

func libraryArchivesByIndexes(artifactsBySource [][]libraryArtifact, indexes []int) []string {
	// A stable topological ordering across the entire binary closure puts each
	// archive before all archives that satisfy its references, including diamonds.
	var ordered []libraryArtifact
	byKey := make(map[string]libraryArtifact)
	incoming := make(map[string]int)
	for _, index := range indexes {
		if index < 0 || index >= len(artifactsBySource) {
			continue
		}
		for _, artifact := range artifactsBySource[index] {
			if _, ok := byKey[artifact.key]; !ok {
				byKey[artifact.key] = artifact
				ordered = append(ordered, artifact)
			}
		}
	}
	for _, artifact := range ordered {
		for _, key := range artifact.dependencies {
			if _, ok := byKey[key]; ok {
				incoming[key]++
			}
		}
	}
	var archives []string
	emitted := make(map[string]bool)
	for len(emitted) < len(ordered) {
		progressed := false
		for _, artifact := range ordered {
			if emitted[artifact.key] || incoming[artifact.key] != 0 {
				continue
			}
			emitted[artifact.key] = true
			archives = append(archives, artifact.archives...)
			for _, key := range artifact.dependencies {
				incoming[key]--
			}
			progressed = true
		}
		// Graphs are cycle-checked before building. Keep this helper total for
		// synthetic metadata supplied by internal callers.
		if !progressed {
			break
		}
	}
	return archives
}

func libraryBuildEnvironment(environment []string, dependencies []libraryArtifact) []string {
	var prefixes, pkgconfig []string
	seen := make(map[string]bool)
	for _, artifact := range dependencies {
		if artifact.prefix == "" || seen[artifact.prefix] {
			continue
		}
		seen[artifact.prefix] = true
		prefixes = append(prefixes, artifact.prefix)
		pkgconfig = append(pkgconfig, filepath.Join(artifact.prefix, "lib", "pkgconfig"), filepath.Join(artifact.prefix, "share", "pkgconfig"))
	}
	for _, item := range []struct {
		name   string
		values []string
	}{{"CMAKE_PREFIX_PATH", prefixes}, {"PKG_CONFIG_PATH", pkgconfig}} {
		if len(item.values) == 0 {
			continue
		}
		for _, value := range environment {
			if strings.HasPrefix(value, item.name+"=") && len(value) > len(item.name)+1 {
				item.values = append(item.values, strings.TrimPrefix(value, item.name+"="))
			}
		}
		environment = environmentWithoutVariable(environment, item.name)
		environment = append(environment, item.name+"="+strings.Join(item.values, string(os.PathListSeparator)))
	}
	return environment
}
