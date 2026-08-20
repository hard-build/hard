package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func sourceEntryPoint(source, workingDirectory string, entrypoints []string) (string, error) {
	return sourceEntryPointWithFlags(source, workingDirectory, nil, entrypoints)
}

func sourceEntryPointWithFlags(
	source string,
	workingDirectory string,
	cflags []string,
	entrypoints []string,
) (string, error) {
	if len(entrypoints) == 0 {
		return "", nil
	}
	path := source
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	entrypoint, err := extractEntryPointFromFile(
		path,
		nil,
		workingDirectory,
		cflags,
		entrypoints,
	)
	if err != nil {
		return "", fmt.Errorf("entry source %s: %w", source, err)
	}
	return entrypoint, nil
}

func extractEntryPoint(source []byte, entrypoints []string) (string, error) {
	workingDirectory, err := filepath.Abs(".")
	if err != nil {
		return "", fmt.Errorf("determine working directory: %w", err)
	}
	path := filepath.Join(workingDirectory, "hard-entry-input.cpp")
	return extractEntryPointFromFile(path, source, workingDirectory, nil, entrypoints)
}

func extractEntryPointFromFile(
	source string,
	contents []byte,
	workingDirectory string,
	cflags []string,
	entrypoints []string,
) (string, error) {
	allowed := make(map[string]struct{}, len(entrypoints))
	for _, entrypoint := range entrypoints {
		allowed[entrypoint] = struct{}{}
	}
	if len(allowed) == 0 {
		return "", nil
	}

	analysis, err := analyzeClangFile(
		source,
		contents,
		clangSourceArguments(cflags, workingDirectory),
		clangAnalysisOptions{},
	)
	if err != nil {
		return "", err
	}
	found := make(map[string]struct{})
	for _, function := range analysis.functions {
		if !function.definition || !function.global || !sameClangFile(function.file, source) {
			continue
		}
		if _, ok := allowed[function.name]; ok {
			found[function.name] = struct{}{}
		}
	}
	if len(found) == 0 {
		return "", nil
	}
	if len(found) == 1 {
		for entrypoint := range found {
			return entrypoint, nil
		}
	}

	names := make([]string, 0, len(found))
	for entrypoint := range found {
		names = append(names, entrypoint)
	}
	sort.Strings(names)
	return "", fmt.Errorf("multiple configured entry points: %s", strings.Join(names, ", "))
}
