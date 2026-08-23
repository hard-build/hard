package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mattn/go-shellwords"
)

const googleTestPackage = "gtest_main"

type testPlan struct {
	source                    string
	sources                   []string
	dependenciesBySource      [][]string
	cacheDependenciesBySource [][]string
	compileIndexes            []int
}

type testPreparationJob struct {
	index  int
	source string
}

type testPreparationResult struct {
	index       int
	plan        testPlan
	diagnostics []byte
	err         error
}

type testLinkJob struct {
	index    int
	source   string
	objects  []string
	artifact string
}

type testLinkResult struct {
	index       int
	diagnostics []byte
	err         error
}

type testRunJob struct {
	index  int
	source string
	binary string
}

type testRunResult struct {
	index  int
	output []byte
	err    error
}

func testSources(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	sources []string,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if len(sources) == 0 {
		return nil
	}
	progress := newProgressBar(stdout, -1, verbose, silent, noColor)
	return testSourcesWithProgress(
		root,
		runtimeRoot,
		environment,
		compiler,
		cflags,
		ldflags,
		sources,
		jobs,
		verbose,
		silent,
		noColor,
		progress,
		stdout,
		stderr,
		false,
	)
}

func testSourcesWithProgress(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	sources []string,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	progress *progressBar,
	stdout io.Writer,
	stderr io.Writer,
	noCache bool,
) error {
	return testSourcesWithProgressSelection(
		root,
		runtimeRoot,
		environment,
		compiler,
		cflags,
		ldflags,
		sources,
		jobs,
		verbose,
		silent,
		noColor,
		progress,
		stdout,
		stderr,
		noCache,
		false,
		nil,
	)
}

func testSourcesWithProgressSelection(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	sources []string,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	progress *progressBar,
	stdout io.Writer,
	stderr io.Writer,
	noCache bool,
	listTests bool,
	testSelectors []string,
) error {
	if len(sources) == 0 {
		var failures []error
		for _, selector := range testSelectors {
			failures = append(
				failures,
				fmt.Errorf("test selector %q matched no tests", selector),
			)
		}
		if err := progress.finish(); err != nil {
			failures = append(failures, err)
		}
		return errors.Join(failures...)
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
	pkgConfigDiagnostics := stderr
	if silent {
		pkgConfigDiagnostics = io.Discard
	}
	googleCFlags, err := pkgConfigFlags("--cflags", workingDirectory, pkgConfigDiagnostics)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	googleLDFlags, err := pkgConfigFlags("--libs", workingDirectory, pkgConfigDiagnostics)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	testCFlags := append(append([]string(nil), cflags...), googleCFlags...)
	testLDFlags := append(append([]string(nil), ldflags...), googleLDFlags...)

	githubResolver := newGitHubSnapshotResolver(root, progress)
	supportHeader := runtimeSupportHeader(runtimeRoot, workingDirectory)
	activity := func(path string, cached bool) {
		step := "Parsing " + buildParsingDisplayPath(root, path, workingDirectory)
		if cached {
			step += " (CACHED)"
		}
		progress.updateStep(step)
	}
	preparationResults := prepareTestsWithCache(
		root,
		environment,
		supportHeader,
		githubResolver,
		testCFlags,
		sources,
		jobs,
		workingDirectory,
		activity,
		cache,
	)
	plans := make([]testPlan, 0, len(sources))
	var failures []error
	for _, result := range preparationResults {
		if len(result.diagnostics) != 0 && (!silent || result.err != nil) {
			if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
				result.err = errors.Join(
					result.err,
					fmt.Errorf("write test dependency diagnostics: %w", err),
				)
			}
		}
		if result.err != nil {
			failures = append(failures, result.err)
			continue
		}
		for index := range result.plan.dependenciesBySource {
			result.plan.dependenciesBySource[index] = removeDependencyPath(
				result.plan.dependenciesBySource[index],
				supportHeader,
			)
		}
		plans = append(plans, result.plan)
	}

	plans, compileSources, compileDependencies, compileCacheDependencies, err := mergeTestCompileSources(
		root,
		environment,
		plans,
	)
	if err != nil {
		progress.setTotal(1)
		return errors.Join(errors.Join(failures...), err, progress.finish())
	}

	stepsPerPlan := 2
	if len(testSelectors) != 0 {
		stepsPerPlan = 3
	}
	progress.setTotal(1 + len(compileSources) + stepsPerPlan*len(plans))
	compileResults, err := compileSourceBatchWithCacheDependencies(
		root,
		environment,
		compiler,
		testCFlags,
		compileSources,
		compileDependencies,
		compileCacheDependencies,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
	)
	if err != nil {
		return errors.Join(errors.Join(failures...), err, progress.finish())
	}
	if err := writeCompileDiagnostics(compileResults, silent, stderr); err != nil {
		return errors.Join(errors.Join(failures...), err, progress.finish())
	}

	linkJobs, linkPlanFailures := planTestLinkJobs(
		root,
		environment,
		plans,
		compileResults,
		workingDirectory,
	)
	failures = append(failures, linkPlanFailures...)
	linkResults := linkTests(
		compiler,
		testLDFlags,
		linkJobs,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
	)
	runJobs := make([]testRunJob, 0, len(linkJobs))
	for index, result := range linkResults {
		if result == nil {
			failures = append(failures, fmt.Errorf("link test %s: not run", linkJobs[index].source))
			continue
		}
		if len(result.diagnostics) != 0 && (!silent || result.err != nil) {
			if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
				result.err = errors.Join(result.err, fmt.Errorf("write test link diagnostics: %w", err))
			}
		}
		if result.err != nil {
			failures = append(failures, result.err)
			continue
		}
		runJobs = append(runJobs, testRunJob{
			index:  len(runJobs),
			source: linkJobs[index].source,
			binary: linkJobs[index].artifact,
		})
	}

	if listTests || len(testSelectors) != 0 {
		listResults := runTestsWithArguments(
			runJobs,
			jobs,
			verbose,
			silent,
			workingDirectory,
			progress,
			nil,
			"Listing",
			testListBinaryArguments(),
		)
		catalogs := make([][]string, len(listResults))
		listingFailures, failedListOutputs := collectTestRunFailures(
			listResults,
			verbose,
			silent,
		)
		for index, result := range listResults {
			if result != nil && result.err == nil {
				catalogs[index] = parseGoogleTestList(result.output)
			}
		}
		failures = append(failures, listingFailures...)

		if listTests {
			if err := progress.finish(); err != nil {
				failures = append(failures, err)
			}
			failures = append(
				failures,
				writeFailedTestOutputs(failedListOutputs, verbose, silent, stdout, stderr)...,
			)
			if err := writeTestCatalogs(stdout, runJobs, catalogs); err != nil {
				failures = append(failures, err)
			}
			return errors.Join(failures...)
		}

		selectionFailed := len(listingFailures) != 0
		for _, selector := range unmatchedTestSelectors(testSelectors, catalogs) {
			selectionFailed = true
			failures = append(
				failures,
				fmt.Errorf("test selector %q matched no tests", selector),
			)
		}
		if selectionFailed {
			if err := progress.finish(); err != nil {
				failures = append(failures, err)
			}
			failures = append(
				failures,
				writeFailedTestOutputs(failedListOutputs, verbose, silent, stdout, stderr)...,
			)
			return errors.Join(failures...)
		}
	}

	runResults := runTestsWithArguments(
		runJobs,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
		"Testing",
		testBinaryArguments(noColor, testSelectors),
	)
	if err := progress.finish(); err != nil {
		failures = append(failures, err)
	}
	runFailures, failedTestOutputs := collectTestRunFailures(runResults, verbose, silent)
	failures = append(failures, runFailures...)
	failures = append(
		failures,
		writeFailedTestOutputs(failedTestOutputs, verbose, silent, stdout, stderr)...,
	)
	return errors.Join(failures...)
}

func collectTestRunFailures(
	results []*testRunResult,
	verbose bool,
	silent bool,
) ([]error, [][]byte) {
	var failures []error
	var outputs [][]byte
	for index, result := range results {
		if result == nil {
			failures = append(failures, fmt.Errorf("test run %d: not run", index))
			continue
		}
		if result.err == nil {
			continue
		}
		failures = append(failures, result.err)
		if (!verbose || silent) && len(result.output) != 0 {
			outputs = append(outputs, result.output)
		}
	}
	return failures, outputs
}

func writeFailedTestOutputs(
	outputs [][]byte,
	verbose bool,
	silent bool,
	stdout io.Writer,
	stderr io.Writer,
) []error {
	var failures []error
	failedTestOutputWriter := stderr
	if !verbose && !silent {
		failedTestOutputWriter = stdout
	}
	for _, output := range outputs {
		if _, err := io.Copy(failedTestOutputWriter, bytes.NewReader(output)); err != nil {
			failures = append(failures, fmt.Errorf("write failed test output: %w", err))
		}
	}
	return failures
}

func prepareTest(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	testSource string,
	workingDirectory string,
) (testPlan, []byte, error) {
	return prepareTestWithActivity(
		githubResolver,
		cflags,
		testSource,
		workingDirectory,
		nil,
	)
}

func prepareTestWithActivity(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	testSource string,
	workingDirectory string,
	activity func(string),
) (testPlan, []byte, error) {
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	return prepareTestWithCache(
		"",
		"",
		"",
		githubResolver,
		cflags,
		testSource,
		workingDirectory,
		parsingActivity,
		nil,
	)
}

func prepareTestWithCache(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	testSource string,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
) (testPlan, []byte, error) {
	var diagnostics bytes.Buffer
	sources, dependenciesBySource, cacheDependenciesBySource, _, failures, err := discoverBuildSourceClosureWithCache(
		root,
		environment,
		supportHeader,
		githubResolver,
		cflags,
		nil,
		[]string{testSource},
		1,
		workingDirectory,
		&diagnostics,
		activity,
		cache,
	)
	if err != nil {
		return testPlan{}, append([]byte(nil), diagnostics.Bytes()...),
			fmt.Errorf("prepare test %s: %w", testSource, err)
	}

	preparationError := errors.Join(failures...)
	if preparationError != nil {
		return testPlan{}, append([]byte(nil), diagnostics.Bytes()...),
			fmt.Errorf("prepare test %s: %w", testSource, preparationError)
	}
	return testPlan{
		source:                    testSource,
		sources:                   sources,
		dependenciesBySource:      dependenciesBySource,
		cacheDependenciesBySource: cacheDependenciesBySource,
	}, append([]byte(nil), diagnostics.Bytes()...), nil
}

func prepareTests(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
) []*testPreparationResult {
	return prepareTestsWithActivity(
		githubResolver,
		cflags,
		sources,
		jobs,
		workingDirectory,
		nil,
	)
}

func prepareTestsWithActivity(
	githubResolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
	activity func(string),
) []*testPreparationResult {
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	return prepareTestsWithCache(
		"",
		"",
		"",
		githubResolver,
		cflags,
		sources,
		jobs,
		workingDirectory,
		parsingActivity,
		nil,
	)
}

func prepareTestsWithCache(
	root string,
	environment string,
	supportHeader string,
	githubResolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
	activity func(string, bool),
	cache *artifactCache,
) []*testPreparationResult {
	if len(sources) == 0 {
		return nil
	}
	workerCount := jobs
	if workerCount > len(sources) {
		workerCount = len(sources)
	}
	queue := make(chan testPreparationJob)
	results := make(chan testPreparationResult, len(sources))

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range queue {
				plan, diagnostics, err := prepareTestWithCache(
					root,
					environment,
					supportHeader,
					githubResolver,
					cflags,
					job.source,
					workingDirectory,
					activity,
					cache,
				)
				results <- testPreparationResult{
					index:       job.index,
					plan:        plan,
					diagnostics: diagnostics,
					err:         err,
				}
			}
		}()
	}

	go func() {
		defer close(queue)
		for index, source := range sources {
			queue <- testPreparationJob{index: index, source: source}
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]*testPreparationResult, len(sources))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}
	return ordered
}

func mergeTestCompileSources(
	root string,
	environment string,
	plans []testPlan,
) ([]testPlan, []string, [][]string, [][]string, error) {
	compileIndexes := make(map[string]int)
	sources := make([]string, 0)
	dependenciesBySource := make([][]string, 0)
	cacheDependenciesBySource := make([][]string, 0)
	for planIndex := range plans {
		plans[planIndex].compileIndexes = make([]int, len(plans[planIndex].sources))
		for sourceIndex, source := range plans[planIndex].sources {
			object, err := objectFilePath(root, environment, source)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if compileIndex, ok := compileIndexes[object]; ok {
				if !equalStringSlices(
					dependenciesBySource[compileIndex],
					plans[planIndex].dependenciesBySource[sourceIndex],
				) || !equalStringSlices(
					cacheDependenciesBySource[compileIndex],
					plans[planIndex].cacheDependenciesBySource[sourceIndex],
				) {
					return nil, nil, nil, nil, fmt.Errorf(
						"conflicting dependency lists for shared test object %s",
						object,
					)
				}
				plans[planIndex].compileIndexes[sourceIndex] = compileIndex
				continue
			}
			compileIndex := len(sources)
			compileIndexes[object] = compileIndex
			plans[planIndex].compileIndexes[sourceIndex] = compileIndex
			sources = append(sources, source)
			dependenciesBySource = append(
				dependenciesBySource,
				plans[planIndex].dependenciesBySource[sourceIndex],
			)
			cacheDependenciesBySource = append(
				cacheDependenciesBySource,
				plans[planIndex].cacheDependenciesBySource[sourceIndex],
			)
		}
	}
	return plans, sources, dependenciesBySource, cacheDependenciesBySource, nil
}

func equalStringSlices(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func planTestLinkJobs(
	root string,
	environment string,
	plans []testPlan,
	compileResults []*compileResult,
	workingDirectory string,
) ([]testLinkJob, []error) {
	tasks := make([]testLinkJob, 0, len(plans))
	artifacts := make(map[string]string)
	var failures []error
	for _, plan := range plans {
		var compileFailures []error
		for sourceIndex, compileIndex := range plan.compileIndexes {
			if compileIndex < 0 || compileIndex >= len(compileResults) {
				compileFailures = append(compileFailures, fmt.Errorf(
					"compile result index for %s is outside result range: %d",
					plan.sources[sourceIndex],
					compileIndex,
				))
				continue
			}
			result := compileResults[compileIndex]
			if result == nil {
				compileFailures = append(
					compileFailures,
					fmt.Errorf("compile %s: not run", plan.sources[sourceIndex]),
				)
				continue
			}
			if result.err != nil {
				compileFailures = append(compileFailures, result.err)
			}
		}
		if err := errors.Join(compileFailures...); err != nil {
			failures = append(failures, fmt.Errorf("compile test %s: %w", plan.source, err))
			continue
		}

		objectIndexes, err := resolveLinkObjectIndexes(
			plan.sources,
			plan.dependenciesBySource,
			make([]string, len(plan.sources)),
			0,
			workingDirectory,
		)
		if err != nil {
			failures = append(
				failures,
				fmt.Errorf("resolve test objects for %s: %w", plan.source, err),
			)
			continue
		}
		objects := make([]string, 0, len(objectIndexes))
		for _, index := range objectIndexes {
			object, err := objectFilePath(root, environment, plan.sources[index])
			if err != nil {
				failures = append(failures, err)
				objects = nil
				break
			}
			objects = append(objects, object)
		}
		if objects == nil {
			continue
		}
		artifact, err := binaryArtifactPath(root, environment, plan.source)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if previous, ok := artifacts[artifact]; ok {
			failures = append(failures, fmt.Errorf(
				"test binary artifact collision: %s and %s map to %s",
				previous,
				plan.source,
				artifact,
			))
			continue
		}
		artifacts[artifact] = plan.source
		tasks = append(tasks, testLinkJob{
			index:    len(tasks),
			source:   plan.source,
			objects:  objects,
			artifact: artifact,
		})
	}
	return tasks, failures
}

func linkTests(
	compiler string,
	ldflags []string,
	tasks []testLinkJob,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
) []*testLinkResult {
	if len(tasks) == 0 {
		return nil
	}
	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	queue := make(chan testLinkJob)
	results := make(chan testLinkResult, len(tasks))
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
						command = renderLinkCommand(
							compiler,
							ldflags,
							job.objects,
							job.artifact,
						)
					}
					step := "Linking " + sourceBinaryName(job.source)
					if cached {
						step += " (CACHED)"
					}
					progress.complete(step, command)
					results <- testLinkResult{
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

	ordered := make([]*testLinkResult, len(tasks))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}
	return ordered
}

func runTests(
	tasks []testRunJob,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
) []*testRunResult {
	return runTestsWithArguments(
		tasks,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
		cache,
		"Testing",
		testBinaryArguments(noColor, nil),
	)
}

func runTestsWithArguments(
	tasks []testRunJob,
	jobs int,
	verbose bool,
	silent bool,
	workingDirectory string,
	progress *progressBar,
	cache *artifactCache,
	action string,
	arguments []string,
) []*testRunResult {
	if len(tasks) == 0 {
		return nil
	}
	workerCount := jobs
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}
	queue := make(chan testRunJob)
	results := make(chan testRunResult, len(tasks))

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range queue {
				cached, output, err := runTestWithCache(
					cache,
					job,
					arguments,
					workingDirectory,
				)
				var detail []byte
				if verbose && !silent && !cached {
					detail = append(detail, renderTestCommand(job.binary, arguments)...)
					detail = append(detail, output...)
				}
				step := action + " " + sourceBinaryName(job.source)
				if cached {
					step += " (CACHED)"
				}
				progress.complete(step, detail)
				results <- testRunResult{
					index:  job.index,
					output: append([]byte(nil), output...),
					err:    err,
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

	ordered := make([]*testRunResult, len(tasks))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}
	return ordered
}

func runTestWithCache(
	cache *artifactCache,
	job testRunJob,
	arguments []string,
	workingDirectory string,
) (bool, []byte, error) {
	var input string
	if cache != nil {
		var err error
		input, err = cache.actionFingerprint(
			"test",
			"",
			arguments,
			[]string{job.binary},
			workingDirectory,
		)
		if err != nil {
			return false, nil, fmt.Errorf("fingerprint test %s: %w", job.source, err)
		}
		cached, err := cache.hit(job.binary, testResultCacheSuffix, input)
		if err != nil {
			return false, nil, fmt.Errorf("read test cache %s: %w", job.source, err)
		}
		if cached {
			return true, nil, nil
		}
		if err := cache.invalidate(job.binary, testResultCacheSuffix); err != nil {
			return false, nil, fmt.Errorf("invalidate test cache %s: %w", job.source, err)
		}
	}

	command := exec.Command(job.binary, arguments...)
	command.Dir = workingDirectory
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			err = fmt.Errorf("test %s: %w", job.source, err)
		} else {
			err = fmt.Errorf("run test %s: %w", job.source, err)
		}
		return false, append([]byte(nil), output.Bytes()...), err
	}
	if cache != nil {
		if err := cache.store(job.binary, testResultCacheSuffix, input); err != nil {
			return false, append([]byte(nil), output.Bytes()...),
				fmt.Errorf("store test cache %s: %w", job.source, err)
		}
	}
	return false, append([]byte(nil), output.Bytes()...), nil
}

func pkgConfigFlags(option, workingDirectory string, stderr io.Writer) ([]string, error) {
	command := exec.Command("pkg-config", option, googleTestPackage)
	command.Dir = workingDirectory
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	command.Stdout = &output
	command.Stderr = &diagnostics
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(diagnostics.String())
		if detail != "" {
			return nil, fmt.Errorf("pkg-config %s %s: %w: %s", option, googleTestPackage, err, detail)
		}
		return nil, fmt.Errorf("pkg-config %s %s: %w", option, googleTestPackage, err)
	}
	if diagnostics.Len() != 0 {
		if _, err := io.Copy(stderr, &diagnostics); err != nil {
			return nil, fmt.Errorf("write pkg-config diagnostics: %w", err)
		}
	}

	parser := shellwords.NewParser()
	parser.ParseEnv = false
	parser.ParseBacktick = false
	flags, err := parser.Parse(output.String())
	if err != nil {
		return nil, fmt.Errorf("parse pkg-config %s %s output: %w", option, googleTestPackage, err)
	}
	return flags, nil
}

func testBinaryArguments(noColor bool, selectors []string) []string {
	arguments := make([]string, 0, 2)
	if len(selectors) != 0 {
		arguments = append(arguments, "--gtest_filter="+strings.Join(selectors, ":"))
	}
	if noColor {
		return append(arguments, "--gtest_color=no")
	}
	return append(arguments, "--gtest_color=yes")
}

func testListBinaryArguments() []string {
	return []string{"--gtest_list_tests", "--gtest_color=no"}
}

func parseGoogleTestList(output []byte) []string {
	var tests []string
	suite := ""
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			name := googleTestListName(line)
			if strings.HasSuffix(name, ".") {
				suite = strings.TrimSuffix(name, ".")
			} else {
				suite = ""
			}
			continue
		}
		if suite == "" {
			continue
		}
		name := googleTestListName(strings.TrimSpace(line))
		if name != "" {
			tests = append(tests, suite+"."+name)
		}
	}
	return tests
}

func googleTestListName(line string) string {
	if index := strings.Index(line, "  #"); index >= 0 {
		line = line[:index]
	}
	return strings.TrimSpace(line)
}

func unmatchedTestSelectors(selectors []string, catalogs [][]string) []string {
	var unmatched []string
	for _, selector := range selectors {
		matched := false
		for _, catalog := range catalogs {
			for _, test := range catalog {
				if matchTestSelector(selector, test) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			unmatched = append(unmatched, selector)
		}
	}
	return unmatched
}

func matchTestSelector(selector, name string) bool {
	current := make([]bool, len(name)+1)
	current[0] = true
	for index := 0; index < len(selector); index++ {
		next := make([]bool, len(name)+1)
		switch selector[index] {
		case '*':
			next[0] = current[0]
			for nameIndex := 1; nameIndex <= len(name); nameIndex++ {
				next[nameIndex] = next[nameIndex-1] || current[nameIndex]
			}
		case '?':
			for nameIndex := 1; nameIndex <= len(name); nameIndex++ {
				next[nameIndex] = current[nameIndex-1]
			}
		default:
			for nameIndex := 1; nameIndex <= len(name); nameIndex++ {
				next[nameIndex] = current[nameIndex-1] &&
					selector[index] == name[nameIndex-1]
			}
		}
		current = next
	}
	return current[len(name)]
}

func writeTestCatalogs(
	writer io.Writer,
	tasks []testRunJob,
	catalogs [][]string,
) error {
	var output strings.Builder
	if len(tasks) == 1 {
		for _, test := range catalogs[0] {
			output.WriteString(test)
			output.WriteByte('\n')
		}
	} else {
		for index, task := range tasks {
			if index != 0 {
				output.WriteByte('\n')
			}
			output.WriteString(task.source)
			output.WriteString(":\n")
			for _, test := range catalogs[index] {
				output.WriteString("  ")
				output.WriteString(test)
				output.WriteByte('\n')
			}
		}
	}
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("write test list: %w", err)
	}
	return nil
}

func renderTestCommand(binary string, arguments []string) []byte {
	command := append([]string{binary}, arguments...)
	for index, argument := range command {
		command[index] = quoteShellArgument(argument)
	}
	return []byte(strings.Join(command, " ") + "\n")
}
