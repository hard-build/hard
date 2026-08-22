package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type forwardNamespace struct {
	name   string
	inline bool
}

type forwardDeclaration struct {
	namespaces []forwardNamespace
	templates  []string
	kind       string
	name       string
}

type forwardGroup struct {
	namespaces   []forwardNamespace
	declarations []forwardDeclaration
}

type forwardJob struct {
	index  int
	header string
	output string
}

type forwardResult struct {
	index int
	err   error
}

func generateForwardDeclarations(root, environment string, headers []string, jobs int) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	return generateForwardDeclarationsWithFlags(
		root,
		environment,
		headers,
		nil,
		jobs,
		workingDirectory,
	)
}

func generateForwardDeclarationsWithFlags(
	root string,
	environment string,
	headers []string,
	cflags []string,
	jobs int,
	workingDirectory string,
) error {
	return generateForwardDeclarationsWithFlagsAndActivity(
		root,
		environment,
		headers,
		cflags,
		jobs,
		workingDirectory,
		nil,
	)
}

func generateForwardDeclarationsWithFlagsAndActivity(
	root string,
	environment string,
	headers []string,
	cflags []string,
	jobs int,
	workingDirectory string,
	activity func(string),
) error {
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	return generateForwardDeclarationsWithFlagsAndCache(
		root,
		environment,
		headers,
		cflags,
		jobs,
		workingDirectory,
		parsingActivity,
		nil,
	)
}

func generateForwardDeclarationsWithFlagsAndCache(
	root string,
	environment string,
	headers []string,
	cflags []string,
	jobs int,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
) error {
	if len(headers) == 0 {
		return nil
	}
	if jobs < 1 {
		return fmt.Errorf("jobs must be positive: %d", jobs)
	}

	tasks := make([]forwardJob, 0, len(headers))
	outputs := make(map[string]string)
	seenHeaders := make(map[string]struct{})
	for _, header := range headers {
		canonicalHeader, err := filepath.EvalSymlinks(header)
		if err != nil {
			return fmt.Errorf("resolve forward header %s: %w", header, err)
		}
		canonicalHeader, err = filepath.Abs(canonicalHeader)
		if err != nil {
			return fmt.Errorf("make forward header absolute %s: %w", header, err)
		}
		if _, ok := seenHeaders[canonicalHeader]; ok {
			continue
		}
		seenHeaders[canonicalHeader] = struct{}{}
		output, err := forwardHeaderPath(root, environment, canonicalHeader)
		if err != nil {
			return err
		}
		if previous, ok := outputs[output]; ok && previous != canonicalHeader {
			return fmt.Errorf("forward header collision: %s and %s map to %s", previous, canonicalHeader, output)
		}
		outputs[output] = canonicalHeader
		tasks = append(tasks, forwardJob{index: len(tasks), header: canonicalHeader, output: output})
	}

	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	queue := make(chan forwardJob)
	results := make(chan forwardResult, len(tasks))

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range queue {
				results <- forwardResult{
					index: task.index,
					err: generateForwardHeaderWithFlagsAndCache(
						cache,
						root,
						environment,
						task.header,
						task.output,
						cflags,
						workingDirectory,
						activity,
					),
				}
			}
		}()
	}

	go func() {
		defer close(queue)
		for _, task := range tasks {
			queue <- task
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]error, len(tasks))
	for result := range results {
		ordered[result.index] = result.err
	}
	return errors.Join(ordered...)
}

func generateForwardHeader(header, output string) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	return generateForwardHeaderWithFlags(header, output, nil, workingDirectory)
}

func generateForwardHeaderWithFlags(
	header string,
	output string,
	cflags []string,
	workingDirectory string,
) error {
	return generateForwardHeaderWithFlagsAndCache(
		nil,
		"",
		"",
		header,
		output,
		cflags,
		workingDirectory,
		nil,
	)
}

func generateForwardHeaderWithFlagsAndCache(
	cache *artifactCache,
	root string,
	environment string,
	header string,
	output string,
	cflags []string,
	workingDirectory string,
	activity func(string, bool),
) error {
	arguments := parseCacheArguments(cflags, nil)
	var recordPath string
	if cache != nil {
		var err error
		recordPath, err = parseCachePath(root, environment, header)
		if err != nil {
			return err
		}
		record, cached, err := cache.parseHit(
			recordPath,
			"header-parse",
			header,
			arguments,
			workingDirectory,
		)
		if err != nil {
			return fmt.Errorf("read parse cache for %s: %w", header, err)
		}
		if cached {
			if activity != nil {
				activity(header, true)
			}
			if err := writeForwardHeader(output, []byte(record.Forward)); err != nil {
				return fmt.Errorf("write cached forward header %s: %w", output, err)
			}
			return nil
		}
		if err := cache.invalidateParse(recordPath); err != nil {
			return fmt.Errorf("invalidate parse cache for %s: %w", header, err)
		}
	}
	if activity != nil {
		activity(header, false)
	}

	declarations, dependencies, err := extractForwardDeclarationsFromHeaderWithDependencies(
		header,
		nil,
		cflags,
		workingDirectory,
	)
	if err != nil {
		return fmt.Errorf("parse forward header %s: %w", header, err)
	}
	declarations, err = safeForwardDeclarations(
		output,
		declarations,
		cflags,
		workingDirectory,
	)
	if err != nil {
		return fmt.Errorf("validate forward header %s: %w", header, err)
	}
	contents := renderForwardDeclarations(declarations)
	if err := writeForwardHeader(output, contents); err != nil {
		return fmt.Errorf("write forward header %s: %w", output, err)
	}
	if cache == nil {
		return nil
	}
	_, err = cache.storeParse(
		recordPath,
		parseCacheRecord{
			Kind:         "header-parse",
			Dependencies: append([]string(nil), dependencies...),
			Forward:      string(contents),
		},
		header,
		arguments,
		workingDirectory,
	)
	if err != nil {
		return fmt.Errorf("store parse cache for %s: %w", header, err)
	}
	return nil
}

func extractForwardDeclarations(source []byte) ([]forwardDeclaration, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine working directory: %w", err)
	}
	header := filepath.Join(workingDirectory, "hard-forward-input.hpp")
	return extractForwardDeclarationsFromHeader(header, source, nil, workingDirectory)
}

func extractForwardDeclarationsFromHeader(
	header string,
	contents []byte,
	cflags []string,
	workingDirectory string,
) ([]forwardDeclaration, error) {
	declarations, _, err := extractForwardDeclarationsFromHeaderWithDependencies(
		header,
		contents,
		cflags,
		workingDirectory,
	)
	return declarations, err
}

func extractForwardDeclarationsFromHeaderWithDependencies(
	header string,
	contents []byte,
	cflags []string,
	workingDirectory string,
) ([]forwardDeclaration, []string, error) {
	absoluteHeader := header
	if !filepath.IsAbs(absoluteHeader) {
		absoluteHeader = filepath.Join(workingDirectory, absoluteHeader)
	}
	absoluteHeader, err := filepath.Abs(absoluteHeader)
	if err != nil {
		return nil, nil, fmt.Errorf("make forward header absolute %s: %w", header, err)
	}
	analysis, err := analyzeClangFile(
		absoluteHeader,
		contents,
		clangHeaderArguments(cflags, workingDirectory),
		clangAnalysisOptions{skipFunctionBodies: true},
	)
	if err != nil {
		return nil, nil, err
	}
	if err := clangForwardSyntaxError(analysis, absoluteHeader); err != nil {
		return nil, nil, err
	}

	var dependencies []string
	if contents == nil {
		dependencySet, err := clangDependencyPathSet(analysis, absoluteHeader, workingDirectory)
		if err != nil {
			return nil, nil, err
		}
		dependencies = dependencySet.managed
	}

	declarations := append([]clangDeclaration(nil), analysis.declarations...)
	sort.SliceStable(declarations, func(left, right int) bool {
		return declarations[left].offset < declarations[right].offset
	})
	seen := make(map[string]struct{})
	result := make([]forwardDeclaration, 0, len(declarations))
	for _, declaration := range declarations {
		if declaration.specialization || !sameClangFile(declaration.file, absoluteHeader) {
			continue
		}
		keyParts := make([]string, 0, len(declaration.namespaces)+1)
		for _, namespace := range declaration.namespaces {
			keyParts = append(keyParts, fmt.Sprintf("%t:%s", namespace.inline, namespace.name))
		}
		keyParts = append(keyParts, declaration.name)
		key := strings.Join(keyParts, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		var templates []string
		if len(declaration.templates) != 0 {
			parameters := make([]string, 0, len(declaration.templates))
			for _, parameter := range declaration.templates {
				parameter = stripTemplateDefault(strings.TrimSpace(parameter))
				if parameter != "" {
					parameters = append(parameters, parameter)
				}
			}
			if len(parameters) != 0 {
				templates = []string{"template <" + strings.Join(parameters, ", ") + ">"}
			}
		}
		result = append(result, forwardDeclaration{
			namespaces: append([]forwardNamespace(nil), declaration.namespaces...),
			templates:  templates,
			kind:       declaration.kind,
			name:       declaration.name,
		})
	}
	return result, dependencies, nil
}

func clangHeaderArguments(cflags []string, workingDirectory string) []string {
	arguments := []string{"-working-directory", workingDirectory}
	arguments = append(arguments, cflags...)
	return append(arguments, "-x", "c++-header")
}

func clangForwardSyntaxError(analysis clangAnalysis, header string) error {
	for _, diagnostic := range analysis.diagnostics {
		if diagnostic.severity < clangDiagnosticError ||
			!sameClangFile(diagnostic.file, header) ||
			diagnostic.category != "Parse Issue" {
			continue
		}
		return fmt.Errorf(
			"invalid C++ syntax at %d:%d: %s",
			diagnostic.line,
			diagnostic.column,
			diagnostic.text,
		)
	}
	return nil
}

func sameClangFile(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftPath, leftErr := filepath.EvalSymlinks(left)
	if leftErr != nil {
		leftPath, leftErr = filepath.Abs(left)
	}
	rightPath, rightErr := filepath.EvalSymlinks(right)
	if rightErr != nil {
		rightPath, rightErr = filepath.Abs(right)
	}
	return leftErr == nil && rightErr == nil && filepath.Clean(leftPath) == filepath.Clean(rightPath)
}

func safeForwardDeclarations(
	output string,
	declarations []forwardDeclaration,
	cflags []string,
	workingDirectory string,
) ([]forwardDeclaration, error) {
	accepted := make([]forwardDeclaration, 0, len(declarations))
	for _, declaration := range declarations {
		candidate := append(append([]forwardDeclaration(nil), accepted...), declaration)
		analysis, err := analyzeClangFile(
			output,
			renderForwardDeclarations(candidate),
			clangHeaderArguments(cflags, workingDirectory),
			clangAnalysisOptions{skipFunctionBodies: true},
		)
		if err != nil {
			return nil, err
		}
		if clangAnalysisHasErrors(analysis) {
			continue
		}
		accepted = candidate
	}
	return accepted, nil
}

func clangAnalysisHasErrors(analysis clangAnalysis) bool {
	for _, diagnostic := range analysis.diagnostics {
		if diagnostic.severity >= clangDiagnosticError {
			return true
		}
	}
	return false
}

func stripTemplateDefault(parameter string) string {
	angle := 0
	parentheses := 0
	brackets := 0
	braces := 0
	for index, character := range parameter {
		switch character {
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '(':
			parentheses++
		case ')':
			if parentheses > 0 {
				parentheses--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '=':
			if angle == 0 && parentheses == 0 && brackets == 0 && braces == 0 {
				return strings.TrimSpace(parameter[:index])
			}
		}
	}
	return parameter
}

func renderForwardDeclarations(declarations []forwardDeclaration) []byte {
	groups := make([]forwardGroup, 0)
	groupIndexes := make(map[string]int)
	for _, declaration := range declarations {
		keyParts := make([]string, 0, len(declaration.namespaces))
		for _, namespace := range declaration.namespaces {
			keyParts = append(keyParts, fmt.Sprintf("%t:%s", namespace.inline, namespace.name))
		}
		key := strings.Join(keyParts, "\x00")
		index, ok := groupIndexes[key]
		if !ok {
			index = len(groups)
			groupIndexes[key] = index
			groups = append(groups, forwardGroup{namespaces: declaration.namespaces})
		}
		groups[index].declarations = append(groups[index].declarations, declaration)
	}

	var output strings.Builder
	output.WriteString("#pragma once\n")
	for _, group := range groups {
		output.WriteByte('\n')
		for _, namespace := range group.namespaces {
			if namespace.inline {
				output.WriteString("inline ")
			}
			fmt.Fprintf(&output, "namespace %s\n{\n", namespace.name)
		}
		for _, declaration := range group.declarations {
			for _, template := range declaration.templates {
				output.WriteString(template)
				output.WriteByte('\n')
			}
			fmt.Fprintf(&output, "%s %s;\n", declaration.kind, declaration.name)
		}
		for range group.namespaces {
			output.WriteString("}\n")
		}
	}
	return []byte(output.String())
}

func forwardHeaderPath(root, environment, header string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("make HARD_ROOT absolute: %w", err)
	}
	absoluteHeader, err := filepath.Abs(header)
	if err != nil {
		return "", fmt.Errorf("make forward header absolute %s: %w", header, err)
	}
	volume := filepath.VolumeName(absoluteHeader)
	mirrored := strings.TrimLeft(absoluteHeader[len(volume):], string(filepath.Separator))
	if mirrored == "" || mirrored == "." {
		return "", fmt.Errorf("cannot mirror forward header path: %s", header)
	}
	extension := filepath.Ext(mirrored)
	stem := strings.TrimSuffix(mirrored, extension)
	environmentRoot := filepath.Join(absoluteRoot, "env")
	outputRoot := filepath.Join(environmentRoot, environment, "build")
	relativeEnvironment, err := filepath.Rel(environmentRoot, outputRoot)
	if err != nil {
		return "", fmt.Errorf("validate HARD_ENV path %s: %w", outputRoot, err)
	}
	if relativeEnvironment == ".." || strings.HasPrefix(relativeEnvironment, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("HARD_ENV escapes environment directory: %s", environment)
	}
	output := filepath.Join(outputRoot, stem+"_fwd"+extension)
	relative, err := filepath.Rel(outputRoot, output)
	if err != nil {
		return "", fmt.Errorf("validate forward header path %s: %w", output, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("forward header path escapes build directory: %s", output)
	}
	return output, nil
}

func writeForwardHeader(path string, contents []byte) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode().IsRegular() {
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Equal(existing, contents) {
			return nil
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func appendForwardNamespace(namespaces []forwardNamespace, namespace forwardNamespace) []forwardNamespace {
	result := make([]forwardNamespace, len(namespaces), len(namespaces)+1)
	copy(result, namespaces)
	return append(result, namespace)
}
