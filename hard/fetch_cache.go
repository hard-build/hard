package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func fetchParseCachePath(root, environment, source string, layouts ...*cacheLayout) (string, error) {
	if len(layouts) != 0 && layouts[0] != nil {
		path, err := layouts[0].sourcePath(source, true)
		if err != nil {
			return "", err
		}
		return path + parseCacheSuffix, nil
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("make HARD_ROOT absolute: %w", err)
	}
	absoluteSource, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("make source absolute %s: %w", source, err)
	}
	volume := filepath.VolumeName(absoluteSource)
	mirrored := strings.TrimLeft(absoluteSource[len(volume):], string(filepath.Separator))
	if mirrored == "" || mirrored == "." {
		return "", fmt.Errorf("cannot mirror source path: %s", source)
	}
	cacheRoot := filepath.Join(absoluteRoot, "fetch")
	environmentRoot := filepath.Join(cacheRoot, environment)
	if !pathWithin(cacheRoot, environmentRoot) {
		return "", fmt.Errorf("HARD_ENV escapes fetch cache directory: %s", environment)
	}
	return filepath.Join(environmentRoot, mirrored+parseCacheSuffix), nil
}

func inspectFetchSourceWithCache(
	root string,
	environment string,
	resolver *githubSnapshotResolver,
	cflags []string,
	job buildJob,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
	libraryManager *libraryManager,
) buildResult {
	result := buildResult{index: job.index, cflags: append([]string(nil), cflags...)}
	arguments := parseCacheArguments(cflags, nil)
	fingerprintWorkingDirectory := compilerCacheWorkingDirectory(cflags, workingDirectory)
	var recordPath string
	if cache != nil {
		var err error
		recordPath, err = fetchParseCachePath(root, environment, job.source, cache.paths())
		if err != nil {
			result.err = err
			return result
		}
		record, cached, err := cache.parseHit(recordPath, "fetch-parse", job.source, arguments, workingDirectory, fingerprintWorkingDirectory)
		if err != nil {
			result.err = fmt.Errorf("read fetch cache for %s: %w", job.source, err)
			return result
		}
		if cached {
			// Validate inherited requirements on every invocation, including edges
			// whose headers were already available in the immutable source view.
			var analysis clangAnalysis
			for _, include := range record.Includes {
				analysis.includes = append(analysis.includes, clangInclude{
					source: include.Source, target: include.Target, spelling: include.Spelling,
				})
			}
			if err := resolver.prepareInheritedIncludes(analysis, workingDirectory); err != nil {
				result.err = err
				return result
			}
			result.libraries, err = libraryManager.prepareHeaders(record.LibraryHeaders)
			if err != nil {
				result.err = err
				return result
			}
			result.cflags = libraryCFlags(cflags, result.libraries)
			result.dependencies = append([]string(nil), record.ManagedDependencies...)
			result.cacheDependencies = append([]string(nil), record.Dependencies...)
			result.libraryHeaders = append([]string(nil), record.LibraryHeaders...)
			if activity != nil {
				activity(job.source, true)
			}
			return result
		}
		if err := cache.invalidateParse(recordPath); err != nil {
			result.err = fmt.Errorf("invalidate fetch cache for %s: %w", job.source, err)
			return result
		}
	}
	if activity != nil {
		activity(job.source, false)
	}
	fatal, dependencies, analysis, effectiveFlags, libraries, libraryHeaders, diagnostics, err := sourceAnalysisWithLibraries(
		resolver, libraryManager, cflags, job.source, workingDirectory,
	)
	result.dependencies = dependencies.managed
	result.cacheDependencies = dependencies.managed
	result.cflags = effectiveFlags
	result.libraries = libraries
	result.libraryHeaders = libraryHeaders
	result.diagnostics = append([]byte(nil), diagnostics...)
	result.fatal = fatal
	result.err = err
	if cache == nil || result.fatal || result.err != nil {
		return result
	}
	record := parseCacheRecord{
		Kind:                "fetch-parse",
		Dependencies:        result.cacheDependencies,
		ManagedDependencies: result.dependencies,
		LibraryHeaders:      result.libraryHeaders,
	}
	for _, include := range analysis.includes {
		if !include.system {
			record.Includes = append(record.Includes, parseCacheInclude{
				Source: include.source, Target: include.target, Spelling: include.spelling,
			})
		}
	}
	// Base flags identify the analysis. Recipe contents are hashed as inputs;
	// their derived include flags are restored only after the input check, so a
	// stale recipe cannot resolve dependencies before reparsing a changed source.
	_, err = cache.storeParse(recordPath, record, job.source, arguments, workingDirectory, fingerprintWorkingDirectory)
	if err != nil {
		result.err = fmt.Errorf("store fetch cache for %s: %w", job.source, err)
	}
	return result
}
