package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Nodes are in build order; dependency indexes always refer to earlier nodes.
// A descriptor-only reference has no Header and no preprocessing context.
type libraryNode struct {
	Descriptor   string `json:"descriptor"`
	Header       string `json:"header,omitempty"`
	Context      string `json:"context,omitempty"`
	Dependencies []int  `json:"dependencies,omitempty"`
}

func isLibraryHeader(path string) bool {
	return isBuildHeader(path) && strings.HasSuffix(strings.TrimSuffix(path, filepath.Ext(path)), ".hard")
}

func libraryDescriptorForHeader(header, workingDirectory string) (string, error) {
	descriptor, err := companionFile(header, workingDirectory, "recipe", func(name string) bool {
		return name == strings.TrimSuffix(filepath.Base(header), filepath.Ext(header))
	}, func(path string) string { return path })
	if err != nil {
		return "", err
	}
	if descriptor == "" {
		return "", fmt.Errorf("recipe header %s requires sibling %s (version: 1 YAML)", header, strings.TrimSuffix(filepath.Base(header), filepath.Ext(header)))
	}
	return realAbsolutePath(descriptor, workingDirectory)
}

func (manager *libraryManager) discoverLibraries(analysis clangAnalysis, dependencies, flags []string) ([]libraryNode, []string, error) {
	var headers []string
	descriptors := make(map[string]string)
	for _, path := range dependencies {
		if !isLibraryHeader(path) {
			continue
		}
		canonical, err := realAbsolutePath(path, manager.workingDirectory)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := descriptors[canonical]; ok {
			continue
		}
		descriptor, err := libraryDescriptorForHeader(canonical, manager.workingDirectory)
		if err != nil {
			return nil, nil, err
		}
		descriptors[canonical] = descriptor
		headers = append(headers, canonical)
	}
	edges := make(map[string][]string)
	for _, include := range analysis.includes {
		if include.source == "" || include.target == "" || include.system {
			continue
		}
		source, err := realAbsolutePath(include.source, manager.workingDirectory)
		if err != nil {
			return nil, nil, err
		}
		target, err := realAbsolutePath(include.target, manager.workingDirectory)
		if err != nil {
			return nil, nil, err
		}
		edges[source] = append(edges[source], target)
	}
	var graph []libraryNode
	done := make(map[string]int)
	active := make(map[string]bool)
	var stack []string
	var visit func(string, string, int) (int, error)
	visit = func(descriptor, header string, occurrence int) (int, error) {
		identity := fmt.Sprintf("%s\x00%s\x00%d", descriptor, header, occurrence)
		if active[descriptor] {
			return 0, fmt.Errorf("recipe dependency cycle: %s", strings.Join(append(stack, descriptor), " -> "))
		}
		if index, ok := done[identity]; ok {
			return index, nil
		}
		active[descriptor] = true
		stack = append(stack, descriptor)
		defer func() { delete(active, descriptor); stack = stack[:len(stack)-1] }()
		contents, err := readRegularProjectFile(descriptor)
		if err != nil {
			return 0, fmt.Errorf("read recipe %s: %w", descriptor, err)
		}
		recipe, _, err := parseLibraryRecipe(contents)
		if err != nil {
			return 0, fmt.Errorf("parse recipe %s: %w", descriptor, err)
		}
		node := libraryNode{Descriptor: descriptor, Header: header}
		added := make(map[int]bool)
		add := func(index int) {
			if !added[index] {
				added[index] = true
				node.Dependencies = append(node.Dependencies, index)
			}
		}
		for _, reference := range recipe.Dependencies {
			path, err := manager.resolveRecipeReference(reference, descriptor, flags)
			if err != nil {
				return 0, err
			}
			index, err := visit(path, "", -1)
			if err != nil {
				return 0, err
			}
			add(index)
		}
		if occurrence >= 0 {
			current := analysis.libraryVisits[occurrence]
			node.Context = current.Context
			for _, child := range current.Dependencies {
				target := analysis.libraryVisits[child].Header
				index, err := visit(descriptors[target], target, child)
				if err != nil {
					return 0, err
				}
				add(index)
			}
		} else if header != "" {
			seen := make(map[string]bool)
			var walk func(string) error
			walk = func(path string) error {
				if seen[path] {
					return nil
				}
				seen[path] = true
				for _, target := range edges[path] {
					if descriptor, ok := descriptors[target]; ok {
						index, err := visit(descriptor, target, -1)
						if err != nil {
							return err
						}
						add(index)
					} else if err := walk(target); err != nil {
						return err
					}
				}
				return nil
			}
			if err := walk(header); err != nil {
				return 0, err
			}
		}
		index := len(graph)
		graph = append(graph, node)
		done[identity] = index
		return index, nil
	}
	if analysis.libraryVisits != nil {
		for index, occurrence := range analysis.libraryVisits {
			if _, err := visit(descriptors[occurrence.Header], occurrence.Header, index); err != nil {
				return nil, nil, err
			}
		}
		return graph, headers, nil
	}
	for _, header := range headers {
		if _, err := visit(descriptors[header], header, -1); err != nil {
			return nil, nil, err
		}
	}
	return graph, headers, nil
}

func (manager *libraryManager) resolveRecipeReference(reference, referringFile string, flags []string) (string, error) {
	// Quoted-include lookup: the including file's directory precedes -iquote/-I.
	candidates := []string{filepath.Join(filepath.Dir(referringFile), filepath.FromSlash(reference))}
	if filepath.IsAbs(reference) {
		candidates = []string{reference}
	}
	directories := make(map[string][]string)
	for i := 0; i < len(flags); i++ {
		for _, option := range []string{"-iquote", "-I", "-isystem", "-idirafter"} {
			var directory string
			if flags[i] == option && i+1 < len(flags) {
				i++
				directory = flags[i]
			} else if strings.HasPrefix(flags[i], option) && len(flags[i]) > len(option) {
				directory = strings.TrimPrefix(flags[i], option)
			}
			if directory != "" {
				if !filepath.IsAbs(directory) {
					directory = filepath.Join(manager.workingDirectory, directory)
				}
				directories[option] = append(directories[option], directory)
				break
			}
		}
	}
	for _, option := range []string{"-iquote", "-I", "-isystem", "-idirafter"} {
		for _, directory := range directories[option] {
			candidates = append(candidates, filepath.Join(directory, filepath.FromSlash(reference)))
		}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("recipe dependency is not a regular file: %s", path)
			}
			return realAbsolutePath(path, manager.workingDirectory)
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if repository, ok := githubRepositoryFromDependency(reference); ok {
		if manager.githubResolver == nil {
			return "", fmt.Errorf("recipe %s requires unavailable dependency %s", referringFile, reference)
		}
		if err := manager.githubResolver.ensure(repository, referringFile); err != nil {
			return "", err
		}
		root, err := githubRepositoryDirectory(manager.root, repository.owner, repository.name)
		if err != nil {
			return "", err
		}
		if session := manager.githubResolver.session; session != nil {
			root = session.selected["github.com/"+repository.key()]
		}
		suffix := strings.TrimPrefix(reference, "github.com/"+repository.key()+"/")
		for _, known := range wellKnownGitHubRepositories {
			if strings.HasPrefix(reference, known.include+"/") {
				suffix = strings.TrimPrefix(reference, known.include+"/")
				break
			}
		}
		path := filepath.Join(root, filepath.FromSlash(suffix))
		if !pathWithin(root, path) {
			return "", fmt.Errorf("recipe dependency escapes repository: %s", reference)
		}
		canonical, err := realAbsolutePath(path, manager.workingDirectory)
		if err != nil {
			return "", fmt.Errorf("recipe %s dependency %s: %w", referringFile, reference, err)
		}
		return canonical, nil
	}
	return "", fmt.Errorf("recipe %s: dependency %s not found in include search paths", referringFile, reference)
}

func (manager *libraryManager) prepareGraph(graph []libraryNode) ([]libraryArtifact, error) {
	if manager == nil {
		return nil, nil
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	var edges clangAnalysis
	for index, node := range graph {
		for _, dependency := range node.Dependencies {
			if dependency < 0 || dependency >= index {
				return nil, fmt.Errorf("invalid cached recipe graph dependency %d for %s", dependency, node.Descriptor)
			}
			edges.includes = append(edges.includes, clangInclude{source: node.Descriptor, target: graph[dependency].Descriptor})
		}
	}
	if err := manager.githubResolver.prepareInheritedIncludes(edges, manager.workingDirectory); err != nil {
		return nil, err
	}
	artifacts := make([]libraryArtifact, 0, len(graph))
	for index, node := range graph {
		contents, err := readRegularProjectFile(node.Descriptor)
		if err != nil {
			return nil, fmt.Errorf("read recipe %s: %w", node.Descriptor, err)
		}
		recipe, _, err := parseLibraryRecipe(contents)
		if err != nil {
			return nil, fmt.Errorf("parse recipe %s: %w", node.Descriptor, err)
		}
		identity := []string{string(contents), node.Context}

		// Every transitive prefix is available to the vendor build. Direct edges
		// remain explicit for archive ordering and package identity.
		var dependencies []libraryArtifact
		seen := make(map[int]bool)
		packages := make(map[string]bool)
		var add func(int) error
		add = func(i int) error {
			if i < 0 || i >= index {
				return fmt.Errorf("invalid cached recipe graph dependency %d for %s", i, node.Descriptor)
			}
			if seen[i] {
				return nil
			}
			seen[i] = true
			if !packages[artifacts[i].key] {
				packages[artifacts[i].key] = true
				dependencies = append(dependencies, artifacts[i])
			}
			for _, child := range graph[i].Dependencies {
				if err := add(child); err != nil {
					return err
				}
			}
			return nil
		}
		for _, i := range node.Dependencies {
			if err := add(i); err != nil {
				return nil, err
			}
		}
		for _, artifact := range dependencies {
			identity = append(identity, artifact.key)
		}
		encoded, err := json.Marshal(identity)
		if err != nil {
			return nil, err
		}
		// Memoization also separates descriptor origins so inherited pins are
		// validated for every referring repository. Persistent keys remain portable.
		digest := sha256.Sum256(append([]byte(node.Descriptor+"\x00"), encoded...))
		key := hex.EncodeToString(digest[:])
		artifact, ok := manager.results[key]
		if !ok {
			artifact, err = manager.prepareRecipe(node.Descriptor, encoded, recipe, dependencies...)
			if err != nil {
				return nil, err
			}
			artifact.header = node.Header
			if !manager.build {
				artifact.key = key
			}
			for _, i := range node.Dependencies {
				artifact.dependencies = append(artifact.dependencies, artifacts[i].key)
			}
			manager.results[key] = artifact
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func libraryGraphFiles(graph []libraryNode) []string {
	var files []string
	seen := make(map[string]bool)
	for _, node := range graph {
		for _, path := range []string{node.Descriptor, node.Header} {
			if path != "" && !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
		}
	}
	return files
}
