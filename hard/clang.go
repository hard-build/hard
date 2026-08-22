package main

/*
#cgo linux CXXFLAGS: -std=c++20 -I/usr/lib/llvm-18/include
#cgo linux LDFLAGS: -lclang-18 -lstdc++
#include <stdlib.h>
#include "clang_bridge.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"
)

func clangVersion() string {
	return C.GoString(C.hard_clang_version())
}

const (
	clangDiagnosticIgnored = iota
	clangDiagnosticNote
	clangDiagnosticWarning
	clangDiagnosticError
	clangDiagnosticFatal
)

type clangInclude struct {
	source   string
	target   string
	spelling string
	system   bool
}

type clangDeclaration struct {
	file           string
	name           string
	kind           string
	definition     bool
	specialization bool
	offset         uint
	namespaces     []forwardNamespace
	templates      []string
}

type clangFunction struct {
	file       string
	name       string
	definition bool
	global     bool
}

type clangDiagnostic struct {
	severity uint
	text     string
	category string
	file     string
	line     uint
	column   uint
}

type clangAnalysis struct {
	includes     []clangInclude
	declarations []clangDeclaration
	functions    []clangFunction
	diagnostics  []clangDiagnostic
}

type clangAnalysisOptions struct {
	skipFunctionBodies bool
}

type clangDependencySet struct {
	managed []string
}

func analyzeClangFile(
	source string,
	contents []byte,
	arguments []string,
	options clangAnalysisOptions,
) (clangAnalysis, error) {
	cSource := C.CString(source)
	defer C.free(unsafe.Pointer(cSource))

	var cContents *C.char
	if contents != nil {
		cContents = C.CString(string(contents))
		defer C.free(unsafe.Pointer(cContents))
	}

	cArguments := make([]*C.char, len(arguments))
	for index, argument := range arguments {
		cArguments[index] = C.CString(argument)
		defer C.free(unsafe.Pointer(cArguments[index]))
	}
	var argumentsPointer **C.char
	if len(cArguments) != 0 {
		argumentsPointer = (**C.char)(unsafe.Pointer(&cArguments[0]))
	}

	var errorCode C.int
	analysis := C.hard_clang_analyze(
		cSource,
		cContents,
		argumentsPointer,
		C.int(len(cArguments)),
		C.int(boolToInt(options.skipFunctionBodies)),
		&errorCode,
	)
	if analysis == nil {
		return clangAnalysis{}, errors.New("libclang returned no analysis")
	}
	defer C.hard_clang_analysis_dispose(analysis)
	if errorCode != 0 {
		message := C.GoString(C.hard_clang_analysis_error(analysis))
		if message == "" {
			message = fmt.Sprintf("libclang parse error %d", int(errorCode))
		}
		return clangAnalysis{}, errors.New(message)
	}

	return copyClangAnalysis(analysis), nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func clangSourceArguments(cflags []string, workingDirectory string) []string {
	arguments := []string{"-working-directory", workingDirectory}
	arguments = append(arguments, cflags...)
	for index, argument := range arguments {
		if argument == "-x" && index+1 < len(arguments) ||
			strings.HasPrefix(argument, "-x") && len(argument) > len("-x") {
			return arguments
		}
	}
	return append(arguments, "-x", "c++")
}

func analyzeClangDependencies(
	source string,
	workingDirectory string,
	cflags []string,
) (clangAnalysis, error) {
	absoluteSource := source
	if !filepath.IsAbs(absoluteSource) {
		absoluteSource = filepath.Join(workingDirectory, absoluteSource)
	}
	absoluteSource, err := filepath.Abs(absoluteSource)
	if err != nil {
		return clangAnalysis{}, fmt.Errorf("make dependency source absolute %s: %w", source, err)
	}
	analysis, err := analyzeClangFile(
		absoluteSource,
		nil,
		clangSourceArguments(cflags, workingDirectory),
		clangAnalysisOptions{skipFunctionBodies: true},
	)
	if err != nil {
		return clangAnalysis{}, fmt.Errorf("analyze dependencies %s: %w", source, err)
	}
	return analysis, nil
}

func clangDependencyPaths(
	analysis clangAnalysis,
	source string,
	workingDirectory string,
) ([]string, error) {
	dependencies, err := clangDependencyPathSet(analysis, source, workingDirectory)
	if err != nil {
		return nil, err
	}
	return dependencies.managed, nil
}

func clangDependencyPathSet(
	analysis clangAnalysis,
	source string,
	workingDirectory string,
) (clangDependencySet, error) {
	sourcePath, err := realAbsolutePath(source, workingDirectory)
	if err != nil {
		return clangDependencySet{}, fmt.Errorf("resolve source %s: %w", source, err)
	}

	managedSeen := make(map[string]struct{})
	managed := make([]string, 0, len(analysis.includes))
	for _, include := range analysis.includes {
		if include.target == "" || include.system {
			continue
		}
		realPath, err := realAbsolutePath(include.target, workingDirectory)
		if err != nil {
			return clangDependencySet{}, fmt.Errorf("resolve dependency %s: %w", include.target, err)
		}
		if realPath == sourcePath {
			continue
		}
		realPath = filepath.Clean(realPath)
		if _, ok := managedSeen[realPath]; ok {
			continue
		}
		managedSeen[realPath] = struct{}{}
		managed = append(managed, realPath)
	}
	sort.Strings(managed)
	return clangDependencySet{managed: managed}, nil
}

func clangUnresolvedIncludes(analysis clangAnalysis) []string {
	seen := make(map[string]struct{})
	includes := make([]string, 0)
	for _, include := range analysis.includes {
		if include.target != "" || include.spelling == "" {
			continue
		}
		spelling := filepath.ToSlash(include.spelling)
		if _, ok := seen[spelling]; ok {
			continue
		}
		seen[spelling] = struct{}{}
		includes = append(includes, spelling)
	}
	sort.Strings(includes)
	return includes
}

func clangErrorDiagnostics(analysis clangAnalysis) []byte {
	var output bytes.Buffer
	for _, diagnostic := range analysis.diagnostics {
		if diagnostic.severity < clangDiagnosticError || diagnostic.text == "" {
			continue
		}
		output.WriteString(diagnostic.text)
		if !strings.HasSuffix(diagnostic.text, "\n") {
			output.WriteByte('\n')
		}
	}
	return output.Bytes()
}

func sourceDependenciesWithClang(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	source string,
	workingDirectory string,
) (bool, []string, []byte, error) {
	fatal, dependencies, diagnostics, err := sourceDependencySetWithClang(
		githubResolver,
		cflags,
		source,
		workingDirectory,
	)
	return fatal, dependencies.managed, diagnostics, err
}

func sourceDependencySetWithClang(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	source string,
	workingDirectory string,
) (bool, clangDependencySet, []byte, error) {
	fatal, dependencies, _, diagnostics, err := sourceAnalysisWithClang(
		githubResolver,
		cflags,
		source,
		workingDirectory,
	)
	return fatal, dependencies, diagnostics, err
}

func sourceAnalysisWithClang(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	source string,
	workingDirectory string,
) (bool, clangDependencySet, clangAnalysis, []byte, error) {
	attemptedRepositories := make(map[string]struct{})
	for {
		analysis, err := analyzeClangDependencies(source, workingDirectory, cflags)
		if err != nil {
			return true, clangDependencySet{}, clangAnalysis{}, nil, err
		}
		unresolved := clangUnresolvedIncludes(analysis)
		if len(unresolved) == 0 {
			dependencies, err := clangDependencyPathSet(analysis, source, workingDirectory)
			if err != nil {
				return true, clangDependencySet{}, analysis, clangErrorDiagnostics(analysis), err
			}
			return false, dependencies, analysis, nil, nil
		}

		newRepository := false
		managedInclude := false
		var downloadErrors []error
		for _, include := range unresolved {
			repository, ok := githubRepositoryFromDependency(include)
			if !ok {
				continue
			}
			managedInclude = true
			if _, ok := attemptedRepositories[repository.key()]; ok {
				continue
			}
			attemptedRepositories[repository.key()] = struct{}{}
			newRepository = true
			if githubResolver == nil {
				downloadErrors = append(
					downloadErrors,
					fmt.Errorf("GitHub dependency is unavailable: %s", include),
				)
				continue
			}
			if err := githubResolver.ensure(repository); err != nil {
				downloadErrors = append(downloadErrors, err)
			}
		}
		if err := errors.Join(downloadErrors...); err != nil {
			return false, clangDependencySet{}, analysis, clangErrorDiagnostics(analysis), err
		}
		if newRepository {
			continue
		}

		diagnostics := clangErrorDiagnostics(analysis)
		if managedInclude {
			return false, clangDependencySet{}, analysis, diagnostics, fmt.Errorf(
				"GitHub dependency remains unavailable for %s: %s",
				source,
				strings.Join(unresolved, ", "),
			)
		}
		if len(diagnostics) == 0 {
			diagnostics = []byte(strings.Join(unresolved, "\n") + "\n")
		}
		return false, clangDependencySet{}, analysis, diagnostics, fmt.Errorf(
			"dependencies %s: unresolved include: %s",
			source,
			strings.Join(unresolved, ", "),
		)
	}
}

func copyClangAnalysis(source *C.hard_clang_analysis) clangAnalysis {
	result := clangAnalysis{}
	includeCount := int(C.hard_clang_include_count(source))
	result.includes = make([]clangInclude, 0, includeCount)
	for index := 0; index < includeCount; index++ {
		cIndex := C.size_t(index)
		result.includes = append(result.includes, clangInclude{
			source:   C.GoString(C.hard_clang_include_source(source, cIndex)),
			target:   C.GoString(C.hard_clang_include_target(source, cIndex)),
			spelling: C.GoString(C.hard_clang_include_spelling(source, cIndex)),
			system:   C.hard_clang_include_is_system(source, cIndex) != 0,
		})
	}

	declarationCount := int(C.hard_clang_declaration_count(source))
	result.declarations = make([]clangDeclaration, 0, declarationCount)
	for index := 0; index < declarationCount; index++ {
		cIndex := C.size_t(index)
		declaration := clangDeclaration{
			file:           C.GoString(C.hard_clang_declaration_file(source, cIndex)),
			name:           C.GoString(C.hard_clang_declaration_name(source, cIndex)),
			kind:           C.GoString(C.hard_clang_declaration_kind(source, cIndex)),
			definition:     C.hard_clang_declaration_is_definition(source, cIndex) != 0,
			specialization: C.hard_clang_declaration_is_specialization(source, cIndex) != 0,
			offset:         uint(C.hard_clang_declaration_offset(source, cIndex)),
		}
		namespaceCount := int(C.hard_clang_declaration_namespace_count(source, cIndex))
		declaration.namespaces = make([]forwardNamespace, 0, namespaceCount)
		for namespaceIndex := 0; namespaceIndex < namespaceCount; namespaceIndex++ {
			cNamespaceIndex := C.size_t(namespaceIndex)
			declaration.namespaces = append(declaration.namespaces, forwardNamespace{
				name: C.GoString(C.hard_clang_declaration_namespace_name(
					source,
					cIndex,
					cNamespaceIndex,
				)),
				inline: C.hard_clang_declaration_namespace_is_inline(
					source,
					cIndex,
					cNamespaceIndex,
				) != 0,
			})
		}
		templateCount := int(C.hard_clang_declaration_template_parameter_count(source, cIndex))
		declaration.templates = make([]string, 0, templateCount)
		for templateIndex := 0; templateIndex < templateCount; templateIndex++ {
			declaration.templates = append(
				declaration.templates,
				C.GoString(C.hard_clang_declaration_template_parameter(
					source,
					cIndex,
					C.size_t(templateIndex),
				)),
			)
		}
		result.declarations = append(result.declarations, declaration)
	}

	functionCount := int(C.hard_clang_function_count(source))
	result.functions = make([]clangFunction, 0, functionCount)
	for index := 0; index < functionCount; index++ {
		cIndex := C.size_t(index)
		result.functions = append(result.functions, clangFunction{
			file:       C.GoString(C.hard_clang_function_file(source, cIndex)),
			name:       C.GoString(C.hard_clang_function_name(source, cIndex)),
			definition: C.hard_clang_function_is_definition(source, cIndex) != 0,
			global:     C.hard_clang_function_is_global(source, cIndex) != 0,
		})
	}

	diagnosticCount := int(C.hard_clang_diagnostic_count(source))
	result.diagnostics = make([]clangDiagnostic, 0, diagnosticCount)
	for index := 0; index < diagnosticCount; index++ {
		cIndex := C.size_t(index)
		result.diagnostics = append(result.diagnostics, clangDiagnostic{
			severity: uint(C.hard_clang_diagnostic_severity(source, cIndex)),
			text:     C.GoString(C.hard_clang_diagnostic_text(source, cIndex)),
			category: C.GoString(C.hard_clang_diagnostic_category(source, cIndex)),
			file:     C.GoString(C.hard_clang_diagnostic_file(source, cIndex)),
			line:     uint(C.hard_clang_diagnostic_line(source, cIndex)),
			column:   uint(C.hard_clang_diagnostic_column(source, cIndex)),
		})
	}
	return result
}
