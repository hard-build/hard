package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type buildJob struct {
	index  int
	source string
}

type buildResult struct {
	index        int
	dependencies []string
	entrypoint   string
	diagnostics  []byte
	fatal        bool
	err          error
}

type compileJob struct {
	index    int
	source   string
	display  string
	object   string
	forwards []string
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
	)
}

func buildSourcesWithProgress(
	root string,
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
	rootSourceCount := len(sources)
	githubResolver := newGitHubSnapshotResolver(root, progress)
	parsingActivity := func(path string) {
		progress.updateStep("Parsing " + buildParsingDisplayPath(root, path, workingDirectory))
	}
	sources, dependenciesBySource, entryPointsBySource, failures, err := discoverBuildSourceClosureWithActivity(
		githubResolver,
		cflags,
		configuredEntryPoints,
		sources,
		jobs,
		workingDirectory,
		stderr,
		parsingActivity,
	)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	supportHeader := environmentSupportHeader(root, environment, workingDirectory)
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
	dependencies := make(map[string]struct{})
	for _, sourceDependencies := range dependenciesBySource {
		for _, dependency := range sourceDependencies {
			dependencies[dependency] = struct{}{}
		}
	}
	paths := make([]string, 0, len(dependencies))
	for dependency := range dependencies {
		paths = append(paths, dependency)
	}
	sort.Strings(paths)
	if err := generateForwardDeclarationsWithFlagsAndActivity(
		root,
		environment,
		paths,
		cflags,
		jobs,
		workingDirectory,
		parsingActivity,
	); err != nil {
		failures = append(failures, err)
	}
	if err := errors.Join(failures...); err != nil {
		return errors.Join(err, progress.finish())
	}
	progress.setTotal(1 + len(sources) + 2*linkCount)
	if err := compileSources(
		root,
		environment,
		compiler,
		cflags,
		sources,
		dependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
	); err != nil {
		return errors.Join(err, progress.finish())
	}
	err = linkSources(
		root,
		environment,
		compiler,
		ldflags,
		sources,
		dependenciesBySource,
		entryPointsBySource,
		rootSourceCount,
		output,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		stderr,
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
	sources := append([]string(nil), rootSources...)
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		canonicalSource, err := realAbsolutePath(source, workingDirectory)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("resolve build source %s: %w", source, err)
		}
		seen[canonicalSource] = struct{}{}
	}

	dependenciesBySource := make([][]string, 0, len(sources))
	entryPointsBySource := make([]string, 0, len(sources))
	var failures []error
	for first := 0; first < len(sources); {
		last := len(sources)
		results := inspectBuildSources(
			githubResolver,
			cflags,
			configuredEntryPoints,
			sources[first:last],
			jobs,
			workingDirectory,
			activity,
		)
		fatal := false
		for _, result := range results {
			var dependencies []string
			var entrypoint string
			if result != nil {
				if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
					return nil, nil, nil, nil, fmt.Errorf("write build diagnostics: %w", err)
				}
				dependencies = result.dependencies
				entrypoint = result.entrypoint
				fatal = fatal || result.fatal
				if result.err != nil {
					failures = append(failures, result.err)
				}
			}
			dependenciesBySource = append(dependenciesBySource, dependencies)
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
					return nil, nil, nil, nil, err
				}
				if source == "" {
					continue
				}
				canonicalSource, err := realAbsolutePath(source, workingDirectory)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("resolve implementation source %s: %w", source, err)
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
	return sources, dependenciesBySource, entryPointsBySource, failures, nil
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

					if activity != nil {
						activity(job.source)
					}
					var diagnostics bytes.Buffer
					fatal, dependencies, err := sourceDependencies(
						githubResolver,
						cflags,
						job.source,
						workingDirectory,
						&diagnostics,
					)
					var entrypoint string
					var entryError error
					if err == nil {
						entrypoint, entryError = sourceEntryPointWithFlags(
							job.source,
							workingDirectory,
							cflags,
							configuredEntryPoints,
						)
					}
					if fatal {
						stopWork()
					}
					results <- buildResult{
						index:        job.index,
						dependencies: dependencies,
						entrypoint:   entrypoint,
						diagnostics:  append([]byte(nil), diagnostics.Bytes()...),
						fatal:        fatal,
						err:          errors.Join(entryError, err),
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
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	stderr io.Writer,
) error {
	results, err := compileSourceBatch(
		root,
		environment,
		compiler,
		cflags,
		sources,
		dependenciesBySource,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
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
) ([]*compileResult, error) {
	if jobs < 1 {
		return nil, fmt.Errorf("jobs must be positive: %d", jobs)
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
		object, err := objectFilePath(root, environment, source)
		if err != nil {
			return nil, err
		}
		forwards := make([]string, 0, len(dependenciesBySource[index]))
		for _, dependency := range dependenciesBySource[index] {
			forward, err := forwardHeaderPath(root, environment, dependency)
			if err != nil {
				return nil, err
			}
			forwards = append(forwards, forward)
		}
		tasks = append(tasks, compileJob{
			index:    index,
			source:   source,
			display:  compileSourceDisplayPath(root, source, workingDirectory),
			object:   object,
			forwards: forwards,
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
					fatal, err := compileSource(
						compiler,
						cflags,
						job.forwards,
						job.source,
						job.object,
						workingDirectory,
						&diagnostics,
					)
					if fatal {
						stopWork()
					}
					var command []byte
					if verbose && !silent {
						command = renderCompileCommand(
							compiler,
							cflags,
							job.forwards,
							job.source,
							job.object,
						)
					}
					progress.complete("Compiling "+job.display, command)
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
	arguments := compileArguments(cflags, forwards, source, object)
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

func linkSources(
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
) error {
	if jobs < 1 {
		return fmt.Errorf("jobs must be positive: %d", jobs)
	}
	tasks, err := planLinkJobs(
		root,
		environment,
		sources,
		dependenciesBySource,
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
					fatal, err := linkBinary(
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
					if verbose && !silent {
						command = renderLinkCommand(compiler, ldflags, job.objects, job.artifact)
					}
					progress.complete("Linking "+job.display, command)
					if err == nil {
						copyError := copyBinary(job.artifact, job.destination)
						progress.complete("Copying "+job.display, nil)
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
		artifact, err := binaryArtifactPath(root, environment, sources[entryIndex])
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
			destination = strings.TrimSuffix(filepath.Clean(sourcePath), filepath.Ext(sourcePath))
		} else if directoryOutput {
			relativeSource, err := binaryOutputRelativeSourcePath(sources[entryIndex], workingDirectory)
			if err != nil {
				return nil, err
			}
			destination = filepath.Join(
				outputPath,
				strings.TrimSuffix(relativeSource, filepath.Ext(relativeSource)),
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

func environmentSupportHeader(root, environment, workingDirectory string) string {
	header, err := realAbsolutePath(
		filepath.Join(root, "env", environment, "hard.h"),
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
	mirrored = strings.TrimSuffix(mirrored, filepath.Ext(mirrored))
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
