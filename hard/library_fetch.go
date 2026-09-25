package main

import (
	"path/filepath"
	"strings"
)

// Fetch can inspect vendor public headers before their build has generated
// configuration headers. Defer those ordinary includes to the package build,
// while keeping missing project headers and recipe/repository references strict.
// Filter individual edges before deduplication: the same spelling can be missing
// both in a vendor header and in the consuming project.
func fetchUnresolvedIncludes(analysis clangAnalysis, artifacts []libraryArtifact, workingDirectory string) ([]string, error) {
	var required clangAnalysis
	for _, include := range analysis.includes {
		if include.target != "" || include.spelling == "" {
			continue
		}
		spelling := filepath.ToSlash(include.spelling)
		_, repository := githubRepositoryFromDependency(spelling)
		if include.source != "" && !repository && !isLibraryHeader(spelling) && !strings.HasSuffix(spelling, ".hard") {
			source, err := realAbsolutePath(include.source, workingDirectory)
			if err != nil {
				return nil, err
			}
			if !isLibraryHeader(source) && libraryOwnsHeader(source, artifacts) {
				continue
			}
		}
		required.includes = append(required.includes, include)
	}
	return clangUnresolvedIncludes(required), nil
}
