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
	libraryRecipeMarker    = "hard.recipe.v1"
	libraryManifestVersion = 1
)

type libraryRecipe struct {
	Source                   string   `yaml:"source"`
	BuildSystem              string   `yaml:"build_system"`
	SourceDirectory          string   `yaml:"source_directory"`
	ConfigureArguments       []string `yaml:"configure_arguments"`
	SourceIncludeDirectories []string `yaml:"source_include_directories"`
	IncludeDirectories       []string `yaml:"include_directories"`
	StaticLibraries          []string `yaml:"static_libraries"`
}

type libraryArtifact struct {
	key      string
	header   string
	cflags   []string
	archives []string
}

type libraryManifest struct {
	Version int         `json:"version"`
	Input   string      `json:"input"`
	Files   []cacheFile `json:"files"`
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
	headers := make([]string, 0)
	for _, dependency := range dependencies {
		contents, err := os.ReadFile(dependency)
		if err != nil {
			return nil, nil, fmt.Errorf("read possible library recipe %s: %w", dependency, err)
		}
		_, found, err := parseLibraryRecipe(contents)
		if err != nil {
			return nil, nil, fmt.Errorf("parse library recipe %s: %w", dependency, err)
		}
		if found {
			headers = append(headers, dependency)
		}
	}
	artifacts, err := manager.prepareHeaders(headers)
	return artifacts, headers, err
}

func (manager *libraryManager) prepareHeaders(headers []string) ([]libraryArtifact, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	artifacts := make([]libraryArtifact, 0, len(headers))
	seen := make(map[string]struct{})
	for _, header := range headers {
		canonical, err := realAbsolutePath(header, manager.workingDirectory)
		if err != nil {
			return nil, fmt.Errorf("resolve library recipe header %s: %w", header, err)
		}
		if artifact, ok := manager.results[canonical]; ok {
			if _, duplicate := seen[artifact.key]; !duplicate {
				seen[artifact.key] = struct{}{}
				artifacts = append(artifacts, artifact)
			}
			continue
		}
		contents, err := os.ReadFile(canonical)
		if err != nil {
			return nil, fmt.Errorf("read library recipe %s: %w", canonical, err)
		}
		recipe, found, err := parseLibraryRecipe(contents)
		if err != nil {
			return nil, fmt.Errorf("parse library recipe %s: %w", canonical, err)
		}
		if !found {
			return nil, fmt.Errorf("library recipe marker is no longer present in %s", canonical)
		}
		artifact, err := manager.prepareRecipe(canonical, contents, recipe)
		if err != nil {
			return nil, err
		}
		manager.results[canonical] = artifact
		if _, duplicate := seen[artifact.key]; !duplicate {
			seen[artifact.key] = struct{}{}
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func (manager *libraryManager) prepareRecipe(
	header string,
	headerContents []byte,
	recipe libraryRecipe,
) (libraryArtifact, error) {
	repository, err := libraryRecipeRepository(recipe.Source)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("library recipe %s: %w", header, err)
	}
	if manager.githubResolver == nil {
		return libraryArtifact{}, fmt.Errorf("library recipe %s requires GitHub dependency %s", header, recipe.Source)
	}
	if err := manager.githubResolver.ensure(repository); err != nil {
		return libraryArtifact{}, err
	}
	sourceRoot, err := githubRepositoryDirectory(manager.root, repository.owner, repository.name)
	if err != nil {
		return libraryArtifact{}, err
	}
	sourceDirectory := filepath.Join(sourceRoot, filepath.FromSlash(recipe.SourceDirectory))
	if manager.build {
		return manager.buildRecipe(header, headerContents, recipe, sourceRoot, sourceDirectory)
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
		key:    header,
		header: header,
		cflags: cflags,
	}, nil
}

func (manager *libraryManager) buildRecipe(
	header string,
	headerContents []byte,
	recipe libraryRecipe,
	sourceRoot string,
	sourceDirectory string,
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
	inputs = append(inputs, header)
	arguments := []string{
		"recipe:" + string(headerContents),
		"compiler-path:" + compilerFingerprint.Path,
		"compiler-digest:" + compilerFingerprint.Digest,
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
	input, err := manager.cache.actionFingerprint(
		"library-cmake-v1",
		cmake,
		arguments,
		inputs,
		manager.workingDirectory,
	)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("fingerprint library %s: %w", recipe.Source, err)
	}
	packageRoot, err := libraryPackageRoot(manager.root, manager.environment, recipe.Source, input)
	if err != nil {
		return libraryArtifact{}, err
	}
	installDirectory := filepath.Join(packageRoot, "install")
	manifestPath := filepath.Join(packageRoot, "manifest.json")
	if !manager.noCache {
		cached, err := libraryManifestHit(manifestPath, installDirectory, input)
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
	if err := os.RemoveAll(packageRoot); err != nil {
		return libraryArtifact{}, fmt.Errorf("remove stale library package %s: %w", packageRoot, err)
	}
	buildDirectory := filepath.Join(packageRoot, "build")
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
	if err := manager.runCMake(cmake, "Configuring "+recipe.Source, configure); err != nil {
		return libraryArtifact{}, err
	}
	if err := manager.runCMake(
		cmake,
		"Building "+recipe.Source,
		[]string{"--build", buildDirectory, "--parallel", fmt.Sprintf("%d", manager.jobs)},
	); err != nil {
		return libraryArtifact{}, err
	}
	if err := manager.runCMake(
		cmake,
		"Installing "+recipe.Source,
		[]string{"--install", buildDirectory},
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
	manifest := libraryManifest{Version: libraryManifestVersion, Input: input, Files: files}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return libraryArtifact{}, fmt.Errorf("encode library manifest %s: %w", manifestPath, err)
	}
	if err := writeCacheRecord(manifestPath, append(encoded, '\n')); err != nil {
		return libraryArtifact{}, fmt.Errorf("write library manifest %s: %w", manifestPath, err)
	}
	return artifact, nil
}

func (manager *libraryManager) runCMake(cmake, step string, arguments []string) error {
	if manager.progress != nil {
		manager.progress.updateStep(step)
	}
	command := exec.Command(cmake, arguments...)
	command.Dir = manager.workingDirectory
	command.Env = environmentWithoutVariable(os.Environ(), "CXXFLAGS")
	command.Env = append(command.Env, "CXXFLAGS=")
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
	text := string(contents)
	position := 0
	if strings.HasPrefix(text, "\ufeff") {
		position += len("\ufeff")
	}
	found := false
	var document string
	for {
		for position < len(text) && strings.ContainsRune(" \t\r\n", rune(text[position])) {
			position++
		}
		if strings.HasPrefix(text[position:], "//") {
			if newline := strings.IndexByte(text[position:], '\n'); newline >= 0 {
				position += newline + 1
				continue
			}
			break
		}
		if !strings.HasPrefix(text[position:], "/*") {
			break
		}
		end := strings.Index(text[position+2:], "*/")
		if end < 0 {
			return libraryRecipe{}, false, errors.New("unterminated leading block comment")
		}
		body := text[position+2 : position+2+end]
		trimmed := strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
		if trimmed == libraryRecipeMarker || strings.HasPrefix(trimmed, libraryRecipeMarker+"\n") {
			if found {
				return libraryRecipe{}, false, errors.New("multiple hard.recipe.v1 blocks")
			}
			found = true
			document = strings.TrimPrefix(trimmed, libraryRecipeMarker)
			document = strings.TrimPrefix(document, "\n")
		}
		position += 2 + end + 2
	}
	if !found {
		return libraryRecipe{}, false, nil
	}
	if strings.TrimSpace(document) == "" {
		return libraryRecipe{}, false, errors.New("empty hard.recipe.v1 document")
	}
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
	environmentRoot := filepath.Join(absoluteRoot, "env")
	libraryRoot := filepath.Join(environmentRoot, environment, "library")
	if !pathWithin(environmentRoot, libraryRoot) {
		return "", fmt.Errorf("HARD_ENV escapes environment directory: %s", environment)
	}
	packageRoot := filepath.Join(
		libraryRoot,
		"github.com",
		repository.owner,
		repository.name,
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
	artifact := libraryArtifact{key: key, header: header}
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

func libraryManifestHit(manifestPath, installDirectory, input string) (bool, error) {
	contents, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read library manifest %s: %w", manifestPath, err)
	}
	var manifest libraryManifest
	if err := json.Unmarshal(contents, &manifest); err != nil ||
		manifest.Version != libraryManifestVersion || manifest.Input != input {
		return false, nil
	}
	files, err := libraryInstallManifestFiles(installDirectory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if len(files) != len(manifest.Files) {
		return false, nil
	}
	for index := range files {
		if files[index] != manifest.Files[index] {
			return false, nil
		}
	}
	return true, nil
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
		if !info.Mode().IsRegular() {
			return fmt.Errorf("library install contains a non-regular file: %s", path)
		}
		digest, err := digestInputFile(path)
		if err != nil {
			return err
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
	archives := make([]string, 0)
	seen := make(map[string]struct{})
	for _, index := range indexes {
		if index < 0 || index >= len(artifactsBySource) {
			continue
		}
		for _, artifact := range artifactsBySource[index] {
			if _, ok := seen[artifact.key]; ok {
				continue
			}
			seen[artifact.key] = struct{}{}
			archives = append(archives, artifact.archives...)
		}
	}
	return archives
}
