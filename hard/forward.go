package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	return forwardDeclarationsFromAnalysis(
		analysis,
		[]string{absoluteHeader},
		workingDirectory,
	), dependencies, nil
}

func forwardDeclarationsFromAnalysis(
	analysis clangAnalysis,
	files []string,
	workingDirectory string,
) []forwardDeclaration {
	allowed := make(map[string]struct{}, len(files))
	for _, file := range files {
		if path, ok := comparableClangPath(file, workingDirectory); ok {
			allowed[path] = struct{}{}
		}
	}
	type ownedDeclaration struct {
		file        string
		declaration clangDeclaration
	}
	declarations := make([]ownedDeclaration, 0, len(analysis.declarations))
	for _, declaration := range analysis.declarations {
		file, ok := comparableClangPath(declaration.file, workingDirectory)
		if !ok {
			continue
		}
		if _, ok := allowed[file]; !ok {
			continue
		}
		declarations = append(declarations, ownedDeclaration{
			file:        file,
			declaration: declaration,
		})
	}
	sort.SliceStable(declarations, func(left, right int) bool {
		if declarations[left].file != declarations[right].file {
			return declarations[left].file < declarations[right].file
		}
		return declarations[left].declaration.offset < declarations[right].declaration.offset
	})
	seen := make(map[string]struct{})
	result := make([]forwardDeclaration, 0, len(declarations))
	for _, owned := range declarations {
		declaration := owned.declaration
		if declaration.specialization {
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
	return result
}

func sourceForwardContents(
	output string,
	analysis clangAnalysis,
	dependencies []string,
	cflags []string,
	workingDirectory string,
) ([]byte, error) {
	declarations := forwardDeclarationsFromAnalysis(analysis, dependencies, workingDirectory)
	declarations, err := safeForwardDeclarations(output, declarations, cflags, workingDirectory)
	if err != nil {
		return nil, err
	}
	return renderForwardDeclarations(declarations), nil
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
	leftPath, leftOK := comparableClangPath(left, "")
	rightPath, rightOK := comparableClangPath(right, "")
	return leftOK && rightOK && leftPath == rightPath
}

func comparableClangPath(path, workingDirectory string) (string, bool) {
	if path == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	return filepath.Clean(absolute), true
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

func sourceForwardHeaderPath(root, environment, source string) (string, error) {
	object, err := objectFilePath(root, environment, source)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(object, ".o") + ".fwd.h", nil
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
