package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type buildJob struct {
	index  int
	source string
}

type buildResult struct {
	index             int
	dependencies      []string
	cacheDependencies []string
	cflags            []string
	libraries         []libraryArtifact
	libraryHeaders    []string
	entrypoint        string
	forward           string
	diagnostics       []byte
	fatal             bool
	err               error
}

type compileJob struct {
	index             int
	source            string
	commandSource     string
	display           string
	object            string
	forwards          []string
	cacheDependencies []string
	cflags            []string
}

type compileResult struct {
	index       int
	diagnostics []byte
	err         error
}

type linkJob struct {
	index       int
	source      string
	objects     []string
	artifact    string
	destination string
	display     string
}

type linkResult struct {
	index       int
	diagnostics []byte
	err         error
}

func buildSources(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	configuredEntryPoints []string,
	sources []string,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	stdout io.Writer,
	stderr io.Writer,
) error {
	progress := newProgressBar(stdout, -1, verbose, silent, noColor)
	return buildSourcesWithProgress(
		root,
		runtimeRoot,
		environment,
		compiler,
		cflags,
		ldflags,
		configuredEntryPoints,
		sources,
		output,
		jobs,
		verbose,
		silent,
		progress,
		stderr,
		false,
	)
}

func buildSourcesWithProgress(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	configuredEntryPoints []string,
	sources []string,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	progress *progressBar,
	stderr io.Writer,
	noCache bool,
) error {
	return buildSourcesWithProgressExecutable(
		root,
		runtimeRoot,
		environment,
		"",
		compiler,
		cflags,
		ldflags,
		configuredEntryPoints,
		sources,
		output,
		jobs,
		verbose,
		silent,
		progress,
		stderr,
		noCache,
	)
}

func buildSourcesWithProgressExecutable(
	root string,
	runtimeRoot string,
	environment string,
	executableSuffix string,
	compiler string,
	cflags []string,
	ldflags []string,
	configuredEntryPoints []string,
	sources []string,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	progress *progressBar,
	stderr io.Writer,
	noCache bool,
) error {
	if len(sources) == 0 {
		return progress.finish()
	}
	if jobs < 1 {
		return errors.Join(fmt.Errorf("jobs must be positive: %d", jobs), progress.finish())
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return errors.Join(fmt.Errorf("determine working directory: %w", err), progress.finish())
	}
	cache, err := newArtifactCache(!noCache)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	rootSourceCount := len(sources)
	githubResolver := newGitHubSnapshotResolver(root, progress)
	libraryManager := newLibraryManager(
		root,
		environment,
		compiler,
		jobs,
		true,
		noCache,
		workingDirectory,
		githubResolver,
		cache,
		progress,
		stderr,
	)
	supportHeader := runtimeSupportHeader(runtimeRoot, workingDirectory)
	parsingActivity := func(path string, cached bool) {
		step := "Parsing " + buildParsingDisplayPath(root, path, workingDirectory)
		if cached {
			step += " (CACHED)"
		}
		progress.updateStep(step)
	}
	sources, dependenciesBySource, cacheDependenciesBySource, cflagsBySource, librariesBySource, entryPointsBySource, failures, err := discoverBuildSourceClosureWithLibraries(
		root,
		environment,
		supportHeader,
		githubResolver,
		cflags,
		configuredEntryPoints,
		sources,
		jobs,
		workingDirectory,
		stderr,
		parsingActivity,
		cache,
		libraryManager,
	)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	for index := range dependenciesBySource {
		dependenciesBySource[index] = removeDependencyPath(
			dependenciesBySource[index],
			supportHeader,
		)
	}
	linkCount := 0
	for index := 0; index < rootSourceCount; index++ {
		if entryPointsBySource[index] != "" {
			linkCount++
		}
	}
	if err := errors.Join(failures...); err != nil {
		return errors.Join(err, progress.finish())
	}
	progress.setTotal(1 + len(sources) + 2*linkCount)
	if err := compileSourcesWithConfiguration(
		root,
		environment,
		compiler,
		cflags,
		cflagsBySource,
		sources,
		dependenciesBySource,
		cacheDependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
		cache,
	); err != nil {
		return errors.Join(err, progress.finish())
	}
	err = linkSourcesWithLibrariesExecutable(
		root,
		environment,
		executableSuffix,
		compiler,
		ldflags,
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		output,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
		cache,
	)
	return errors.Join(err, progress.finish())
}

func discoverBuildSourceClosure(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	rootSources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
) ([]string, [][]string, []string, []error, error) {
	return discoverBuildSourceClosureWithActivity(
		githubResolver,
		cflags,
		configuredEntryPoints,
		rootSources,
		jobs,
		workingDirectory,
		stderr,
		nil,
	)
}

func discoverBuildSourceClosureWithActivity(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	rootSources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
	activity func(string),
) ([]string, [][]string, []string, []error, error) {
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	sources, dependencies, _, entryPoints, failures, err := discoverBuildSourceClosureWithCache(
		"",
		"",
		"",
		githubResolver,
		cflags,
		configuredEntryPoints,
		rootSources,
		jobs,
		workingDirectory,
		stderr,
		parsingActivity,
		nil,
	)
	return sources, dependencies, entryPoints, failures, err
}

func discoverBuildSourceClosureWithCache(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	rootSources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
	activity func(string, bool),
	cache *artifactCache,
) ([]string, [][]string, [][]string, []string, []error, error) {
	sources, dependencies, cacheDependencies, _, _, entryPoints, failures, err := discoverBuildSourceClosureWithLibraries(
		root,
		environment,
		supportHeader,
		githubResolver,
		cflags,
		configuredEntryPoints,
		rootSources,
		jobs,
		workingDirectory,
		stderr,
		activity,
		cache,
		nil,
	)
	return sources, dependencies, cacheDependencies, entryPoints, failures, err
}

func discoverBuildSourceClosureWithLibraries(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	rootSources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
	activity func(string, bool),
	cache *artifactCache,
	libraryManager *libraryManager,
) ([]string, [][]string, [][]string, [][]string, [][]libraryArtifact, []string, []error, error) {
	sources := append([]string(nil), rootSources...)
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		canonicalSource, err := realAbsolutePath(source, workingDirectory)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("resolve build source %s: %w", source, err)
		}
		seen[canonicalSource] = struct{}{}
	}

	dependenciesBySource := make([][]string, 0, len(sources))
	cacheDependenciesBySource := make([][]string, 0, len(sources))
	cflagsBySource := make([][]string, 0, len(sources))
	librariesBySource := make([][]libraryArtifact, 0, len(sources))
	entryPointsBySource := make([]string, 0, len(sources))
	var failures []error
	for first := 0; first < len(sources); {
		last := len(sources)
		results := inspectBuildSourcesWithLibraries(
			root,
			environment,
			supportHeader,
			githubResolver,
			cflags,
			configuredEntryPoints,
			sources[first:last],
			jobs,
			workingDirectory,
			activity,
			cache,
			libraryManager,
		)
		fatal := false
		for _, result := range results {
			var dependencies []string
			var cacheDependencies []string
			effectiveCFlags := append([]string(nil), cflags...)
			var libraries []libraryArtifact
			var entrypoint string
			if result != nil {
				if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
					return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("write build diagnostics: %w", err)
				}
				dependencies = result.dependencies
				cacheDependencies = result.cacheDependencies
				effectiveCFlags = result.cflags
				libraries = result.libraries
				entrypoint = result.entrypoint
				fatal = fatal || result.fatal
				if result.err != nil {
					failures = append(failures, result.err)
				}
			}
			dependenciesBySource = append(dependenciesBySource, dependencies)
			cacheDependenciesBySource = append(cacheDependenciesBySource, cacheDependencies)
			cflagsBySource = append(cflagsBySource, effectiveCFlags)
			librariesBySource = append(librariesBySource, libraries)
			entryPointsBySource = append(entryPointsBySource, entrypoint)
		}
		if fatal {
			break
		}

		for index := first; index < last; index++ {
			for _, dependency := range dependenciesBySource[index] {
				if !isBuildHeader(dependency) {
					continue
				}
				source, err := implementationSourceForHeader(dependency, workingDirectory)
				if err != nil {
					return nil, nil, nil, nil, nil, nil, nil, err
				}
				if source == "" {
					continue
				}
				canonicalSource, err := realAbsolutePath(source, workingDirectory)
				if err != nil {
					return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("resolve implementation source %s: %w", source, err)
				}
				if _, ok := seen[canonicalSource]; ok {
					continue
				}
				seen[canonicalSource] = struct{}{}
				sources = append(sources, source)
			}
		}
		first = last
	}
	return sources, dependenciesBySource, cacheDependenciesBySource, cflagsBySource, librariesBySource, entryPointsBySource, failures, nil
}

func inspectBuildSources(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	sources []string,
	jobs int,
	workingDirectory string,
	activity func(string),
) []*buildResult {
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	return inspectBuildSourcesWithCache(
		"",
		"",
		"",
		githubResolver,
		cflags,
		configuredEntryPoints,
		sources,
		jobs,
		workingDirectory,
		parsingActivity,
		nil,
	)
}

func inspectBuildSourcesWithCache(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	sources []string,
	jobs int,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
) []*buildResult {
	return inspectBuildSourcesWithLibraries(
		root,
		environment,
		supportHeader,
		githubResolver,
		cflags,
		configuredEntryPoints,
		sources,
		jobs,
		workingDirectory,
		activity,
		cache,
		nil,
	)
}

func inspectBuildSourcesWithLibraries(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	sources []string,
	jobs int,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
	libraryManager *libraryManager,
) []*buildResult {
	workerCount := jobs
	if workerCount > len(sources) {
		workerCount = len(sources)
	}

	queue := make(chan buildJob)
	results := make(chan buildResult, len(sources))
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopWork := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}

				select {
				case <-stop:
					return
				case job, ok := <-queue:
					if !ok {
						return
					}
					result := inspectBuildSourceWithCache(
						root,
						environment,
						supportHeader,
						githubResolver,
						cflags,
						configuredEntryPoints,
						job,
						workingDirectory,
						activity,
						cache,
						libraryManager,
					)
					if result.fatal {
						stopWork()
					}
					results <- result
					if result.fatal {
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(queue)
		for index, source := range sources {
			select {
			case <-stop:
				return
			case queue <- buildJob{index: index, source: source}:
			}
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]*buildResult, len(sources))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}
	return ordered
}

func inspectBuildSourceWithCache(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	configuredEntryPoints []string,
	job buildJob,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
	libraryManager *libraryManager,
) buildResult {
	result := buildResult{index: job.index, cflags: append([]string(nil), cflags...)}
	var recordPath string
	if cache != nil {
		cacheCandidateReady := true
		var err error
		recordPath, err = parseCachePath(root, environment, job.source)
		if err != nil {
			result.err = err
			return result
		}
		if cache.read && libraryManager != nil {
			candidate, ok, err := readParseCacheRecord(recordPath)
			if err != nil {
				result.err = fmt.Errorf("read parse cache for %s: %w", job.source, err)
				return result
			}
			candidateResult, resultError := parseResultFingerprint(candidate)
			if ok && candidate.Version == artifactCacheVersion && candidate.Kind == "source-parse" &&
				resultError == nil && candidate.Result == candidateResult {
				result.libraries, err = libraryManager.prepareHeaders(candidate.LibraryHeaders)
				if err != nil {
					cacheCandidateReady = false
					result.libraries = nil
					result.libraryHeaders = nil
					result.cflags = append([]string(nil), cflags...)
				} else {
					result.cflags = libraryCFlags(cflags, result.libraries)
					result.libraryHeaders = append([]string(nil), candidate.LibraryHeaders...)
				}
			}
		}
		if cacheCandidateReady {
			arguments := parseCacheArguments(result.cflags, configuredEntryPoints)
			fingerprintWorkingDirectory := compilerCacheWorkingDirectory(
				result.cflags,
				workingDirectory,
			)
			record, cached, err := cache.parseHit(
				recordPath,
				"source-parse",
				job.source,
				arguments,
				workingDirectory,
				fingerprintWorkingDirectory,
			)
			if err != nil {
				result.err = fmt.Errorf("read parse cache for %s: %w", job.source, err)
				return result
			}
			if cached {
				if activity != nil {
					activity(job.source, true)
				}
				forward, err := sourceForwardHeaderPath(root, environment, job.source)
				if err != nil {
					result.err = err
					return result
				}
				if err := writeForwardHeader(forward, []byte(record.Forward)); err != nil {
					result.err = fmt.Errorf("write cached forward header %s: %w", forward, err)
					return result
				}
				result.dependencies = append([]string(nil), record.ManagedDependencies...)
				result.cacheDependencies = append([]string(nil), record.Dependencies...)
				result.entrypoint = record.EntryPoint
				result.forward = record.Forward
				return result
			}
		}
		if err := cache.invalidateParse(recordPath); err != nil {
			result.err = fmt.Errorf("invalidate parse cache for %s: %w", job.source, err)
			return result
		}
	}
	if activity != nil {
		activity(job.source, false)
	}

	fatal, dependencies, analysis, effectiveCFlags, libraries, libraryHeaders, diagnostics, err := sourceAnalysisWithLibraries(
		githubResolver,
		libraryManager,
		cflags,
		job.source,
		workingDirectory,
	)
	result.dependencies = dependencies.managed
	result.cacheDependencies = dependencies.managed
	result.cflags = effectiveCFlags
	result.libraries = libraries
	result.libraryHeaders = libraryHeaders
	result.diagnostics = append([]byte(nil), diagnostics...)
	result.fatal = fatal
	var entryError error
	if err == nil {
		result.entrypoint, entryError = sourceEntryPointWithFlags(
			job.source,
			workingDirectory,
			result.cflags,
			configuredEntryPoints,
		)
	}
	var forwardError error
	if err == nil && entryError == nil && cache != nil {
		forward, pathError := sourceForwardHeaderPath(root, environment, job.source)
		if pathError != nil {
			forwardError = pathError
		} else {
			forwardDependencies := removeDependencyPath(
				result.dependencies,
				supportHeader,
			)
			contents, contentError := sourceForwardContents(
				forward,
				analysis,
				forwardDependencies,
				result.cflags,
				workingDirectory,
			)
			if contentError != nil {
				forwardError = fmt.Errorf("generate forward header for %s: %w", job.source, contentError)
			} else if writeError := writeForwardHeader(forward, contents); writeError != nil {
				forwardError = fmt.Errorf("write forward header %s: %w", forward, writeError)
			} else {
				result.forward = string(contents)
			}
		}
	}
	result.err = errors.Join(entryError, forwardError, err)
	if cache == nil || result.fatal || result.err != nil {
		return result
	}
	arguments := parseCacheArguments(result.cflags, configuredEntryPoints)
	fingerprintWorkingDirectory := compilerCacheWorkingDirectory(
		result.cflags,
		workingDirectory,
	)
	_, cacheError := cache.storeParse(
		recordPath,
		parseCacheRecord{
			Kind:                "source-parse",
			Dependencies:        append([]string(nil), result.cacheDependencies...),
			ManagedDependencies: append([]string(nil), result.dependencies...),
			LibraryHeaders:      append([]string(nil), result.libraryHeaders...),
			EntryPoint:          result.entrypoint,
			Forward:             result.forward,
		},
		job.source,
		arguments,
		workingDirectory,
		fingerprintWorkingDirectory,
	)
	if cacheError != nil {
		result.err = fmt.Errorf("store parse cache for %s: %w", job.source, cacheError)
	}
	return result
}

func implementationSourceForHeader(header, workingDirectory string) (string, error) {
	directory := filepath.Dir(header)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("search implementation for header %s: %w", header, err)
	}
	stem := strings.TrimSuffix(filepath.Base(header), filepath.Ext(header))
	headerKey := strings.TrimSuffix(header, filepath.Ext(header))
	var candidates []string
	for _, entry := range entries {
		matches, err := matchesSource("build", entry.Name())
		if err != nil {
			return "", err
		}
		if !matches || strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) != stem {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("inspect implementation source %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		canonicalPath, err := realAbsolutePath(path, workingDirectory)
		if err != nil {
			return "", fmt.Errorf("resolve implementation source %s: %w", path, err)
		}
		if strings.TrimSuffix(canonicalPath, filepath.Ext(canonicalPath)) != headerKey {
			continue
		}
		relativePath, err := filepath.Rel(workingDirectory, path)
		if err != nil {
			return "", fmt.Errorf("make implementation source relative %s: %w", path, err)
		}
		candidates = append(candidates, filepath.Clean(relativePath))
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf(
			"multiple sources implement header %s: %s",
			header,
			strings.Join(candidates, ", "),
		)
	}
	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0], nil
}

func compileSources(
	root string,
	environment string,
	compiler string,
	cflags []string,
	sources []string,
	dependenciesBySource [][]string,
	cacheDependenciesBySource [][]string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
	cache *artifactCache,
) error {
	return compileSourcesWithConfiguration(
		root,
		environment,
		compiler,
		cflags,
		nil,
		sources,
		dependenciesBySource,
		cacheDependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
		cache,
	)
}

func compileSourcesWithConfiguration(
	root string,
	environment string,
	compiler string,
	cflags []string,
	cflagsBySource [][]string,
	sources []string,
	dependenciesBySource [][]string,
	cacheDependenciesBySource [][]string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
	cache *artifactCache,
) error {
	results, err := compileSourceBatchWithConfiguration(
		root,
		environment,
		compiler,
		cflags,
		cflagsBySource,
		sources,
		dependenciesBySource,
		cacheDependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
	)
	if err != nil {
		return err
	}
	if err := writeCompileDiagnostics(results, silent, stderr); err != nil {
		return err
	}
	return compileResultsError(results)
}

func compileSourceBatch(
	root string,
	environment string,
	compiler string,
	cflags []string,
	sources []string,
	dependenciesBySource [][]string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
) ([]*compileResult, error) {
	return compileSourceBatchWithCacheDependencies(
		root,
		environment,
		compiler,
		cflags,
		sources,
		dependenciesBySource,
		dependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
	)
}

func compileSourceBatchWithCacheDependencies(
	root string,
	environment string,
	compiler string,
	cflags []string,
	sources []string,
	dependenciesBySource [][]string,
	cacheDependenciesBySource [][]string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
) ([]*compileResult, error) {
	return compileSourceBatchWithConfiguration(
		root,
		environment,
		compiler,
		cflags,
		nil,
		sources,
		dependenciesBySource,
		cacheDependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
	)
}

func compileSourceBatchWithConfiguration(
	root string,
	environment string,
	compiler string,
	cflags []string,
	cflagsBySource [][]string,
	sources []string,
	dependenciesBySource [][]string,
	cacheDependenciesBySource [][]string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
) ([]*compileResult, error) {
	if jobs < 1 {
		return nil, fmt.Errorf("jobs must be positive: %d", jobs)
	}
	if cflagsBySource == nil {
		cflagsBySource = make([][]string, len(sources))
		for index := range cflagsBySource {
			cflagsBySource[index] = cflags
		}
	}
	if len(cflagsBySource) != len(sources) {
		return nil, fmt.Errorf(
			"compiler flag source count does not match sources: %d != %d",
			len(cflagsBySource),
			len(sources),
		)
	}
	if len(cacheDependenciesBySource) != len(sources) {
		return nil, fmt.Errorf(
			"cache dependency source count does not match sources: %d != %d",
			len(cacheDependenciesBySource),
			len(sources),
		)
	}
	if len(dependenciesBySource) != len(sources) {
		return nil, fmt.Errorf(
			"dependency source count does not match sources: %d != %d",
			len(dependenciesBySource),
			len(sources),
		)
	}
	tasks := make([]compileJob, 0, len(sources))
	for index, source := range sources {
		commandSource, err := lexicalAbsolutePath(source, workingDirectory)
		if err != nil {
			return nil, fmt.Errorf("make compile source absolute %s: %w", source, err)
		}
		object, err := objectFilePath(root, environment, source)
		if err != nil {
			return nil, err
		}
		forward, err := sourceForwardHeaderPath(root, environment, source)
		if err != nil {
			return nil, err
		}
		cacheDependencies := cacheDependenciesBySource[index]
		tasks = append(tasks, compileJob{
			index:             index,
			source:            source,
			commandSource:     commandSource,
			display:           compileSourceDisplayPath(root, source, workingDirectory),
			object:            object,
			forwards:          []string{forward},
			cacheDependencies: cacheDependencies,
			cflags:            cflagsBySource[index],
		})
	}

	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	queue := make(chan compileJob)
	results := make(chan compileResult, len(tasks))
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopWork := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}

				select {
				case <-stop:
					return
				case job, ok := <-queue:
					if !ok {
						return
					}

					var diagnostics bytes.Buffer
					fatal, cached, err := compileSourceWithCache(
						cache,
						compiler,
						job.cflags,
						job.forwards,
						job.cacheDependencies,
						job.source,
						job.object,
						workingDirectory,
						&diagnostics,
					)
					if fatal {
						stopWork()
					}
					var command []byte
					if verbose && !silent && !cached {
						command = renderCompileCommand(
							compiler,
							job.cflags,
							job.forwards,
							job.commandSource,
							job.object,
						)
					}
					step := "Compiling " + job.display
					if cached {
						step += " (CACHED)"
					}
					progress.complete(step, command)
					results <- compileResult{
						index:       job.index,
						diagnostics: append([]byte(nil), diagnostics.Bytes()...),
						err:         err,
					}
					if fatal {
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(queue)
		for _, task := range tasks {
			select {
			case <-stop:
				return
			case queue <- task:
			}
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]*compileResult, len(tasks))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}
	return ordered, nil
}

func writeCompileDiagnostics(results []*compileResult, silent bool, stderr io.Writer) error {
	for _, result := range results {
		if result == nil {
			continue
		}
		if len(result.diagnostics) != 0 && (!silent || result.err != nil) {
			if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
				return fmt.Errorf("write compile diagnostics: %w", err)
			}
		}
	}
	return nil
}

func compileResultsError(results []*compileResult) error {
	var failures []error
	for _, result := range results {
		if result == nil {
			continue
		}
		if result.err != nil {
			failures = append(failures, result.err)
		}
	}
	return errors.Join(failures...)
}

func compileSourceWithCache(
	cache *artifactCache,
	compiler string,
	cflags []string,
	forwards []string,
	cacheDependencies []string,
	source string,
	object string,
	workingDirectory string,
	stderr io.Writer,
) (bool, bool, error) {
	commandSource, err := lexicalAbsolutePath(source, workingDirectory)
	if err != nil {
		return true, false, fmt.Errorf("make compile source absolute %s: %w", source, err)
	}
	if cache == nil {
		fatal, err := compileSource(
			compiler,
			cflags,
			forwards,
			source,
			object,
			workingDirectory,
			stderr,
		)
		return fatal, false, err
	}

	arguments := compileArguments(cflags, forwards, commandSource, object)
	inputs := append([]string{commandSource}, cacheDependencies...)
	inputs = append(inputs, forwards...)
	input, err := cache.actionFingerprintWithWorkingDirectory(
		"compile",
		compiler,
		arguments,
		inputs,
		workingDirectory,
		compilerCacheWorkingDirectory(cflags, workingDirectory),
	)
	if err != nil {
		return true, false, fmt.Errorf("fingerprint compile %s: %w", source, err)
	}
	cached, err := cache.hit(object, buildCacheSuffix, input)
	if err != nil {
		return true, false, fmt.Errorf("read compile cache %s: %w", source, err)
	}
	if cached {
		return false, true, nil
	}
	if err := cache.invalidate(object, buildCacheSuffix); err != nil {
		return true, false, fmt.Errorf("invalidate compile cache %s: %w", source, err)
	}
	fatal, err := compileSource(
		compiler,
		cflags,
		forwards,
		source,
		object,
		workingDirectory,
		stderr,
	)
	if err != nil {
		return fatal, false, err
	}
	if err := cache.store(object, buildCacheSuffix, input); err != nil {
		return true, false, fmt.Errorf("store compile cache %s: %w", source, err)
	}
	return false, false, nil
}

func compileSource(
	compiler string,
	cflags []string,
	forwards []string,
	source string,
	object string,
	workingDirectory string,
	stderr io.Writer,
) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(object), 0o755); err != nil {
		return true, fmt.Errorf("create object directory for %s: %w", source, err)
	}
	commandSource, err := lexicalAbsolutePath(source, workingDirectory)
	if err != nil {
		return true, fmt.Errorf("make compile source absolute %s: %w", source, err)
	}
	arguments := compileArguments(cflags, forwards, commandSource, object)
	command := exec.Command(compiler, arguments...)
	command.Dir = workingDirectory
	command.Stdout = stderr
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return true, fmt.Errorf("run %s: %w", compiler, err)
		}
		return false, fmt.Errorf("compile %s: %w", source, err)
	}
	return false, nil
}

func compileArguments(cflags, forwards []string, source, object string) []string {
	arguments := append([]string(nil), cflags...)
	for _, forward := range forwards {
		arguments = append(arguments, "-include", forward)
	}
	return append(arguments, "-c", source, "-o", object)
}

func renderCompileCommand(compiler string, cflags, forwards []string, source, object string) []byte {
	arguments := append([]string{compiler}, compileArguments(cflags, forwards, source, object)...)
	for index, argument := range arguments {
		arguments[index] = quoteShellArgument(argument)
	}
	return []byte(strings.Join(arguments, " ") + "\n")
}

func linkSourcesWithCache(
	root string,
	environment string,
	compiler string,
	ldflags []string,
	sources []string,
	dependenciesBySource [][]string,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
	cache *artifactCache,
) error {
	return linkSourcesWithLibraries(
		root,
		environment,
		compiler,
		ldflags,
		sources,
		dependenciesBySource,
		nil,
		entryPointsBySource,
		rootSourceCount,
		output,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
		cache,
	)
}

func linkSourcesWithLibraries(
	root string,
	environment string,
	compiler string,
	ldflags []string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
	cache *artifactCache,
) error {
	return linkSourcesWithLibrariesExecutable(
		root,
		environment,
		"",
		compiler,
		ldflags,
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		output,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
		cache,
	)
}

func linkSourcesWithLibrariesExecutable(
	root string,
	environment string,
	executableSuffix string,
	compiler string,
	ldflags []string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
	cache *artifactCache,
) error {
	if jobs < 1 {
		return fmt.Errorf("jobs must be positive: %d", jobs)
	}
	tasks, err := planLinkJobsWithLibrariesExecutable(
		root,
		environment,
		executableSuffix,
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		output,
		workingDirectory,
	)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	queue := make(chan linkJob)
	results := make(chan linkResult, len(tasks))
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopWork := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}

				select {
				case <-stop:
					return
				case job, ok := <-queue:
					if !ok {
						return
					}

					var diagnostics bytes.Buffer
					fatal, cached, err := linkBinaryWithCache(
						cache,
						compiler,
						ldflags,
						job.objects,
						job.source,
						job.artifact,
						workingDirectory,
						&diagnostics,
					)
					if fatal {
						stopWork()
					}
					var command []byte
					if verbose && !silent && !cached {
						command = renderLinkCommand(compiler, ldflags, job.objects, job.artifact)
					}
					linkStep := "Linking " + job.display
					if cached {
						linkStep += " (CACHED)"
					}
					progress.complete(linkStep, command)
					if err == nil {
						copyCached := false
						var copyError error
						if cache != nil && cache.read {
							copyCached, copyError = sameRegularFiles(job.artifact, job.destination)
						}
						if copyError == nil && !copyCached {
							copyError = copyBinary(job.artifact, job.destination)
						}
						copyStep := "Copying " + job.display
						if copyCached {
							copyStep += " (CACHED)"
						}
						progress.complete(copyStep, nil)
						if copyError != nil {
							err = fmt.Errorf(
								"copy binary %s to %s: %w",
								job.source,
								job.destination,
								copyError,
							)
						}
					}
					results <- linkResult{
						index:       job.index,
						diagnostics: append([]byte(nil), diagnostics.Bytes()...),
						err:         err,
					}
					if fatal {
						return
					}
				}
			}
		}()
	}

	go func() {
		defer close(queue)
		for _, task := range tasks {
			select {
			case <-stop:
				return
			case queue <- task:
			}
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]*linkResult, len(tasks))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}

	var failures []error
	for _, result := range ordered {
		if result == nil {
			continue
		}
		if len(result.diagnostics) != 0 && (!silent || result.err != nil) {
			if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
				return fmt.Errorf("write link diagnostics: %w", err)
			}
		}
		if result.err != nil {
			failures = append(failures, result.err)
		}
	}
	return errors.Join(failures...)
}

func planLinkJobs(
	root string,
	environment string,
	sources []string,
	dependenciesBySource [][]string,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	workingDirectory string,
) ([]linkJob, error) {
	return planLinkJobsWithLibraries(
		root,
		environment,
		sources,
		dependenciesBySource,
		nil,
		entryPointsBySource,
		rootSourceCount,
		output,
		workingDirectory,
	)
}

func planLinkJobsWithLibraries(
	root string,
	environment string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	workingDirectory string,
) ([]linkJob, error) {
	return planLinkJobsWithLibrariesExecutable(
		root,
		environment,
		"",
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		output,
		workingDirectory,
	)
}

func planLinkJobsWithLibrariesExecutable(
	root string,
	environment string,
	executableSuffix string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	output string,
	workingDirectory string,
) ([]linkJob, error) {
	if len(dependenciesBySource) != len(sources) {
		return nil, fmt.Errorf(
			"dependency source count does not match sources: %d != %d",
			len(dependenciesBySource),
			len(sources),
		)
	}
	if librariesBySource != nil && len(librariesBySource) != len(sources) {
		return nil, fmt.Errorf(
			"library source count does not match sources: %d != %d",
			len(librariesBySource),
			len(sources),
		)
	}
	if len(entryPointsBySource) != len(sources) {
		return nil, fmt.Errorf(
			"entry source count does not match sources: %d != %d",
			len(entryPointsBySource),
			len(sources),
		)
	}
	if rootSourceCount < 0 || rootSourceCount > len(sources) {
		return nil, fmt.Errorf(
			"root source count is outside source range: %d not in [0, %d]",
			rootSourceCount,
			len(sources),
		)
	}

	entryIndexes := make([]int, 0)
	for index := 0; index < rootSourceCount; index++ {
		if entryPointsBySource[index] != "" {
			entryIndexes = append(entryIndexes, index)
		}
	}
	if len(entryIndexes) == 0 {
		return nil, nil
	}

	directoryOutput, outputPath, err := resolveLinkOutput(output, workingDirectory, len(entryIndexes))
	if err != nil {
		return nil, err
	}
	artifacts := make(map[string]string)
	destinations := make(map[string]string)
	tasks := make([]linkJob, 0, len(entryIndexes))
	for _, entryIndex := range entryIndexes {
		objectIndexes, err := resolveLinkObjectIndexes(
			sources,
			dependenciesBySource,
			entryPointsBySource,
			entryIndex,
			workingDirectory,
		)
		if err != nil {
			return nil, fmt.Errorf("resolve link objects for %s: %w", sources[entryIndex], err)
		}

		objects := make([]string, 0, len(objectIndexes))
		for _, objectIndex := range objectIndexes {
			object, err := objectFilePath(root, environment, sources[objectIndex])
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		}
		objects = append(objects, libraryArchivesByIndexes(librariesBySource, objectIndexes)...)
		artifact, err := binaryArtifactPathWithSuffix(
			root,
			environment,
			sources[entryIndex],
			executableSuffix,
		)
		if err != nil {
			return nil, err
		}
		if previous, ok := artifacts[artifact]; ok {
			return nil, fmt.Errorf("binary artifact collision: %s and %s map to %s", previous, sources[entryIndex], artifact)
		}
		artifacts[artifact] = sources[entryIndex]

		destination := outputPath
		if output == "" {
			sourcePath := sources[entryIndex]
			if !filepath.IsAbs(sourcePath) {
				sourcePath = filepath.Join(workingDirectory, sourcePath)
			}
			destination = appendExecutableSuffix(
				strings.TrimSuffix(filepath.Clean(sourcePath), filepath.Ext(sourcePath)),
				executableSuffix,
			)
		} else if directoryOutput {
			relativeSource, err := binaryOutputRelativeSourcePath(sources[entryIndex], workingDirectory)
			if err != nil {
				return nil, err
			}
			destination = appendExecutableSuffix(
				filepath.Join(
					outputPath,
					strings.TrimSuffix(relativeSource, filepath.Ext(relativeSource)),
				),
				executableSuffix,
			)
		}
		if previous, ok := destinations[destination]; ok {
			return nil, fmt.Errorf("binary output collision: %s and %s map to %s", previous, sources[entryIndex], destination)
		}
		if err := validateLinkDestination(destination); err != nil {
			return nil, err
		}
		destinations[destination] = sources[entryIndex]

		display := destination
		if output == "" || !filepath.IsAbs(output) {
			if relative, err := filepath.Rel(workingDirectory, destination); err == nil {
				display = relative
			}
		}
		tasks = append(tasks, linkJob{
			index:       len(tasks),
			source:      sources[entryIndex],
			objects:     objects,
			artifact:    artifact,
			destination: destination,
			display:     display,
		})
	}
	return tasks, nil
}

func resolveLinkObjectIndexes(
	sources []string,
	dependenciesBySource [][]string,
	entryPointsBySource []string,
	rootIndex int,
	workingDirectory string,
) ([]int, error) {
	if len(dependenciesBySource) != len(sources) {
		return nil, fmt.Errorf(
			"dependency source count does not match sources: %d != %d",
			len(dependenciesBySource),
			len(sources),
		)
	}
	if len(entryPointsBySource) != len(sources) {
		return nil, fmt.Errorf(
			"entry source count does not match sources: %d != %d",
			len(entryPointsBySource),
			len(sources),
		)
	}
	if rootIndex < 0 || rootIndex >= len(sources) {
		return nil, fmt.Errorf(
			"root source index is outside source range: %d not in [0, %d)",
			rootIndex,
			len(sources),
		)
	}

	owners := make(map[string][]int)
	for index, source := range sources {
		canonicalSource, err := realAbsolutePath(source, workingDirectory)
		if err != nil {
			return nil, fmt.Errorf("resolve implementation source %s: %w", source, err)
		}
		key := strings.TrimSuffix(canonicalSource, filepath.Ext(canonicalSource))
		owners[key] = append(owners[key], index)
	}

	seen := make(map[int]struct{})
	objectIndexes := make([]int, 0)
	var visit func(int) error
	visit = func(sourceIndex int) error {
		if _, ok := seen[sourceIndex]; ok {
			return nil
		}
		seen[sourceIndex] = struct{}{}
		objectIndexes = append(objectIndexes, sourceIndex)
		for _, dependency := range dependenciesBySource[sourceIndex] {
			if !isBuildHeader(dependency) {
				continue
			}
			key := strings.TrimSuffix(dependency, filepath.Ext(dependency))
			candidates := owners[key]
			if len(candidates) > 1 {
				implementations := make([]string, 0, len(candidates))
				for _, candidate := range candidates {
					implementations = append(implementations, sources[candidate])
				}
				return fmt.Errorf(
					"multiple sources implement header %s: %s",
					dependency,
					strings.Join(implementations, ", "),
				)
			}
			if len(candidates) == 0 {
				continue
			}
			candidate := candidates[0]
			if candidate != rootIndex && entryPointsBySource[candidate] != "" {
				continue
			}
			if err := visit(candidate); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(rootIndex); err != nil {
		return nil, err
	}
	return objectIndexes, nil
}

func resolveLinkOutput(output, workingDirectory string, entryCount int) (bool, string, error) {
	if output == "" {
		return true, workingDirectory, nil
	}
	directory := strings.HasSuffix(output, string(filepath.Separator))
	path := output
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			directory = true
		} else if directory {
			return false, "", fmt.Errorf("binary output directory is not a directory: %s", path)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "", fmt.Errorf("inspect binary output %s: %w", path, err)
	}
	if !directory && entryCount > 1 {
		return false, "", fmt.Errorf("binary output %s requires exactly one entry source, found %d", output, entryCount)
	}
	return directory, path, nil
}

func validateLinkDestination(destination string) error {
	info, err := os.Lstat(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect binary destination %s: %w", destination, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("binary destination is not a regular file: %s", destination)
	}
	return nil
}

func isBuildHeader(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".h", ".hh", ".hpp", ".h++":
		return true
	default:
		return false
	}
}

func runtimeSupportHeader(runtimeRoot, workingDirectory string) string {
	header, err := realAbsolutePath(
		filepath.Join(runtimeRoot, "hard.h"),
		workingDirectory,
	)
	if err != nil {
		return ""
	}
	return header
}

func removeDependencyPath(dependencies []string, excluded string) []string {
	if excluded == "" {
		return dependencies
	}
	filtered := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency != excluded {
			filtered = append(filtered, dependency)
		}
	}
	return filtered
}

func sourceBinaryName(source string) string {
	base := filepath.Base(source)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func binaryOutputRelativeSourcePath(source, workingDirectory string) (string, error) {
	absoluteSource := source
	if !filepath.IsAbs(absoluteSource) {
		absoluteSource = filepath.Join(workingDirectory, absoluteSource)
	}
	absoluteSource = filepath.Clean(absoluteSource)
	relativeSource, err := filepath.Rel(workingDirectory, absoluteSource)
	if err != nil {
		return "", fmt.Errorf("make binary source relative %s: %w", source, err)
	}
	if relativeSource != ".." && !strings.HasPrefix(relativeSource, ".."+string(filepath.Separator)) {
		return relativeSource, nil
	}
	volume := filepath.VolumeName(absoluteSource)
	mirrored := strings.TrimLeft(absoluteSource[len(volume):], string(filepath.Separator))
	if mirrored == "" || mirrored == "." {
		return "", fmt.Errorf("cannot mirror binary source path: %s", source)
	}
	return mirrored, nil
}

func compileSourceDisplayPath(root, source, workingDirectory string) string {
	canonicalSource, err := realAbsolutePath(source, workingDirectory)
	if err != nil {
		return source
	}
	rootPath := root
	if !filepath.IsAbs(rootPath) {
		rootPath = filepath.Join(workingDirectory, rootPath)
	}
	absoluteRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return source
	}
	sourceRoot, err := filepath.EvalSymlinks(filepath.Join(absoluteRoot, "source"))
	if err != nil {
		return source
	}
	if !pathWithin(filepath.Join(sourceRoot, "github.com"), canonicalSource) {
		return source
	}
	relative, err := filepath.Rel(sourceRoot, canonicalSource)
	if err != nil {
		return source
	}
	return filepath.ToSlash(relative)
}

func buildParsingDisplayPath(root, path, workingDirectory string) string {
	display := compileSourceDisplayPath(root, path, workingDirectory)
	if display != path || !filepath.IsAbs(path) {
		return display
	}
	relative, err := filepath.Rel(workingDirectory, path)
	if err != nil {
		return path
	}
	return filepath.Clean(relative)
}

func linkBinaryWithCache(
	cache *artifactCache,
	compiler string,
	ldflags []string,
	objects []string,
	source string,
	artifact string,
	workingDirectory string,
	stderr io.Writer,
) (bool, bool, error) {
	if cache == nil {
		fatal, err := linkBinary(
			compiler,
			ldflags,
			objects,
			source,
			artifact,
			workingDirectory,
			stderr,
		)
		return fatal, false, err
	}

	arguments := linkArguments(ldflags, objects, artifact)
	input, err := cache.actionFingerprintWithWorkingDirectory(
		"link",
		compiler,
		arguments,
		objects,
		workingDirectory,
		compilerCacheWorkingDirectory(ldflags, workingDirectory),
	)
	if err != nil {
		return true, false, fmt.Errorf("fingerprint link %s: %w", source, err)
	}
	cached, err := cache.hit(artifact, buildCacheSuffix, input)
	if err != nil {
		return true, false, fmt.Errorf("read link cache %s: %w", source, err)
	}
	if cached {
		return false, true, nil
	}
	if err := cache.invalidate(artifact, buildCacheSuffix); err != nil {
		return true, false, fmt.Errorf("invalidate link cache %s: %w", source, err)
	}
	fatal, err := linkBinary(
		compiler,
		ldflags,
		objects,
		source,
		artifact,
		workingDirectory,
		stderr,
	)
	if err != nil {
		return fatal, false, err
	}
	if err := cache.store(artifact, buildCacheSuffix, input); err != nil {
		return true, false, fmt.Errorf("store link cache %s: %w", source, err)
	}
	return false, false, nil
}

func linkBinary(
	compiler string,
	ldflags []string,
	objects []string,
	source string,
	artifact string,
	workingDirectory string,
	stderr io.Writer,
) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		return true, fmt.Errorf("create binary directory for %s: %w", source, err)
	}
	command := exec.Command(compiler, linkArguments(ldflags, objects, artifact)...)
	command.Dir = workingDirectory
	command.Stdout = stderr
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return true, fmt.Errorf("run %s: %w", compiler, err)
		}
		return false, fmt.Errorf("link %s: %w", source, err)
	}
	return false, nil
}

func linkArguments(ldflags, objects []string, output string) []string {
	arguments := append([]string(nil), objects...)
	arguments = append(arguments, ldflags...)
	return append(arguments, "-o", output)
}

func renderLinkCommand(compiler string, ldflags, objects []string, output string) []byte {
	arguments := append([]string{compiler}, linkArguments(ldflags, objects, output)...)
	for index, argument := range arguments {
		arguments[index] = quoteShellArgument(argument)
	}
	return []byte(strings.Join(arguments, " ") + "\n")
}

func copyBinary(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("linked artifact is not a regular file: %s", source)
	}

	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(destination)+".*")
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
	if _, err := io.Copy(temporary, input); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func quoteShellArgument(argument string) string {
	if argument != "" {
		safe := true
		for _, character := range argument {
			if character >= 'a' && character <= 'z' ||
				character >= 'A' && character <= 'Z' ||
				character >= '0' && character <= '9' ||
				strings.ContainsRune("_@%+=:,./-", character) {
				continue
			}
			safe = false
			break
		}
		if safe {
			return argument
		}
	}
	return "'" + strings.ReplaceAll(argument, "'", "'\"'\"'") + "'"
}

func objectFilePath(root, environment, source string) (string, error) {
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
	environmentRoot := filepath.Join(absoluteRoot, "env")
	outputRoot := filepath.Join(environmentRoot, environment, "build")
	relativeEnvironment, err := filepath.Rel(environmentRoot, outputRoot)
	if err != nil {
		return "", fmt.Errorf("validate HARD_ENV path %s: %w", outputRoot, err)
	}
	if relativeEnvironment == ".." || strings.HasPrefix(relativeEnvironment, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("HARD_ENV escapes environment directory: %s", environment)
	}
	object := filepath.Join(outputRoot, mirrored+".o")
	relative, err := filepath.Rel(outputRoot, object)
	if err != nil {
		return "", fmt.Errorf("validate object path %s: %w", object, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("object path escapes build directory: %s", object)
	}
	return object, nil
}

func binaryArtifactPath(root, environment, source string) (string, error) {
	return binaryArtifactPathWithSuffix(root, environment, source, "")
}

func binaryArtifactPathWithSuffix(root, environment, source, executableSuffix string) (string, error) {
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
	if sourceBinaryName(mirrored) == "" {
		return "", fmt.Errorf("cannot derive binary name from source: %s", source)
	}
	mirrored = appendExecutableSuffix(
		strings.TrimSuffix(mirrored, filepath.Ext(mirrored)),
		executableSuffix,
	)
	environmentRoot := filepath.Join(absoluteRoot, "env")
	outputRoot := filepath.Join(environmentRoot, environment, "build")
	relativeEnvironment, err := filepath.Rel(environmentRoot, outputRoot)
	if err != nil {
		return "", fmt.Errorf("validate HARD_ENV path %s: %w", outputRoot, err)
	}
	if relativeEnvironment == ".." || strings.HasPrefix(relativeEnvironment, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("HARD_ENV escapes environment directory: %s", environment)
	}
	binary := filepath.Join(outputRoot, mirrored)
	relative, err := filepath.Rel(outputRoot, binary)
	if err != nil {
		return "", fmt.Errorf("validate binary path %s: %w", binary, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("binary path escapes build directory: %s", binary)
	}
	return binary, nil
}

func sourceDependencies(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	source string,
	workingDirectory string,
	stderr io.Writer,
) (bool, []string, error) {
	fatal, dependencies, diagnostics, err := sourceDependenciesWithClang(
		githubResolver,
		cflags,
		source,
		workingDirectory,
	)
	if len(diagnostics) != 0 {
		if _, writeError := stderr.Write(diagnostics); writeError != nil {
			return true, nil, fmt.Errorf("write dependency diagnostics: %w", writeError)
		}
	}
	return fatal, dependencies, err
}

func realAbsolutePath(path, workingDirectory string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(realPath)
}

func lexicalAbsolutePath(path, workingDirectory string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}
