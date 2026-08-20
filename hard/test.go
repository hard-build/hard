package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/mattn/go-shellwords"
)

const googleTestPackage = "gtest_main"

type testPlan struct {
	source               string
	sources              []string
	dependenciesBySource [][]string
	headers              []string
	compileIndexes       []int
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

type testForwardJob struct {
	index  int
	header string
}

type testForwardResult struct {
	index int
	err   error
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
	)
}

func testSourcesWithProgress(
	root string,
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
	activity := func(path string) {
		progress.updateStep("Parsing " + buildParsingDisplayPath(root, path, workingDirectory))
	}
	preparationResults := prepareTestsWithActivity(
		githubResolver,
		testCFlags,
		sources,
		jobs,
		workingDirectory,
		activity,
	)
	supportHeader := environmentSupportHeader(root, environment, workingDirectory)
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
		result.plan.headers = removeDependencyPath(result.plan.headers, supportHeader)
		plans = append(plans, result.plan)
	}

	plans, forwardFailures := generateTestForwardDeclarationsWithActivity(
		root,
		environment,
		plans,
		testCFlags,
		jobs,
		workingDirectory,
		activity,
	)
	failures = append(failures, forwardFailures...)

	plans, compileSources, compileDependencies, err := mergeTestCompileSources(
		root,
		environment,
		plans,
	)
	if err != nil {
		progress.setTotal(1)
		return errors.Join(errors.Join(failures...), err, progress.finish())
	}

	progress.setTotal(1 + len(compileSources) + 2*len(plans))
	compileResults, err := compileSourceBatch(
		root,
		environment,
		compiler,
		testCFlags,
		compileSources,
		compileDependencies,
		jobs,
		verbose,
		silent,
		workingDirectory,
		progress,
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

	runResults := runTests(runJobs, jobs, verbose, silent, noColor, workingDirectory, progress)
	if err := progress.finish(); err != nil {
		failures = append(failures, err)
	}
	var failedTestOutputs [][]byte
	for _, result := range runResults {
		if result.err != nil {
			failures = append(failures, result.err)
			if (!verbose || silent) && len(result.output) != 0 {
				failedTestOutputs = append(failedTestOutputs, result.output)
			}
		}
	}
	failedTestOutputWriter := stderr
	if !verbose && !silent {
		failedTestOutputWriter = stdout
	}
	for _, output := range failedTestOutputs {
		if _, err := io.Copy(failedTestOutputWriter, bytes.NewReader(output)); err != nil {
			failures = append(failures, fmt.Errorf("write failed test output: %w", err))
		}
	}
	return errors.Join(failures...)
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
	var diagnostics bytes.Buffer
	sources, dependenciesBySource, _, failures, err := discoverBuildSourceClosureWithActivity(
		githubResolver,
		cflags,
		nil,
		[]string{testSource},
		1,
		workingDirectory,
		&diagnostics,
		activity,
	)
	if err != nil {
		return testPlan{}, append([]byte(nil), diagnostics.Bytes()...),
			fmt.Errorf("prepare test %s: %w", testSource, err)
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
	preparationError := errors.Join(failures...)
	if preparationError != nil {
		return testPlan{}, append([]byte(nil), diagnostics.Bytes()...),
			fmt.Errorf("prepare test %s: %w", testSource, preparationError)
	}
	return testPlan{
		source:               testSource,
		sources:              sources,
		dependenciesBySource: dependenciesBySource,
		headers:              paths,
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
				plan, diagnostics, err := prepareTestWithActivity(
					githubResolver,
					cflags,
					job.source,
					workingDirectory,
					activity,
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

func generateTestForwardDeclarations(
	root string,
	environment string,
	plans []testPlan,
	cflags []string,
	jobs int,
	workingDirectory string,
) ([]testPlan, []error) {
	return generateTestForwardDeclarationsWithActivity(
		root,
		environment,
		plans,
		cflags,
		jobs,
		workingDirectory,
		nil,
	)
}

func generateTestForwardDeclarationsWithActivity(
	root string,
	environment string,
	plans []testPlan,
	cflags []string,
	jobs int,
	workingDirectory string,
	activity func(string),
) ([]testPlan, []error) {
	if len(plans) == 0 {
		return nil, nil
	}
	headerIndexes := make(map[string]int)
	headers := make([]string, 0)
	for _, plan := range plans {
		for _, header := range plan.headers {
			if _, ok := headerIndexes[header]; ok {
				continue
			}
			headerIndexes[header] = len(headers)
			headers = append(headers, header)
		}
	}
	if len(headers) == 0 {
		return plans, nil
	}

	workerCount := jobs
	if workerCount > len(headers) {
		workerCount = len(headers)
	}
	queue := make(chan testForwardJob)
	results := make(chan testForwardResult, len(headers))
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range queue {
				results <- testForwardResult{
					index: job.index,
					err: generateForwardDeclarationsWithFlagsAndActivity(
						root,
						environment,
						[]string{job.header},
						cflags,
						1,
						workingDirectory,
						activity,
					),
				}
			}
		}()
	}
	go func() {
		defer close(queue)
		for index, header := range headers {
			queue <- testForwardJob{index: index, header: header}
		}
	}()
	workers.Wait()
	close(results)

	forwardErrors := make([]error, len(headers))
	for result := range results {
		forwardErrors[result.index] = result.err
	}
	ready := make([]testPlan, 0, len(plans))
	var failures []error
	for _, plan := range plans {
		var planFailures []error
		for _, header := range plan.headers {
			if err := forwardErrors[headerIndexes[header]]; err != nil {
				planFailures = append(planFailures, err)
			}
		}
		if err := errors.Join(planFailures...); err != nil {
			failures = append(failures, fmt.Errorf("prepare test %s: %w", plan.source, err))
			continue
		}
		ready = append(ready, plan)
	}
	return ready, failures
}

func mergeTestCompileSources(
	root string,
	environment string,
	plans []testPlan,
) ([]testPlan, []string, [][]string, error) {
	compileIndexes := make(map[string]int)
	sources := make([]string, 0)
	dependenciesBySource := make([][]string, 0)
	for planIndex := range plans {
		plans[planIndex].compileIndexes = make([]int, len(plans[planIndex].sources))
		for sourceIndex, source := range plans[planIndex].sources {
			object, err := objectFilePath(root, environment, source)
			if err != nil {
				return nil, nil, nil, err
			}
			if compileIndex, ok := compileIndexes[object]; ok {
				if !equalStringSlices(
					dependenciesBySource[compileIndex],
					plans[planIndex].dependenciesBySource[sourceIndex],
				) {
					return nil, nil, nil, fmt.Errorf(
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
		}
	}
	return plans, sources, dependenciesBySource, nil
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
						command = renderLinkCommand(
							compiler,
							ldflags,
							job.objects,
							job.artifact,
						)
					}
					progress.complete("Linking "+sourceBinaryName(job.source), command)
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
	arguments := testBinaryArguments(noColor)

	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range queue {
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
				}
				var detail []byte
				if verbose && !silent {
					detail = append(detail, renderTestCommand(job.binary, arguments)...)
					detail = append(detail, output.Bytes()...)
				}
				progress.complete("Testing "+sourceBinaryName(job.source), detail)
				results <- testRunResult{
					index:  job.index,
					output: append([]byte(nil), output.Bytes()...),
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

func testBinaryArguments(noColor bool) []string {
	if noColor {
		return []string{"--gtest_color=no"}
	}
	return []string{"--gtest_color=yes"}
}

func renderTestCommand(binary string, arguments []string) []byte {
	command := append([]string{binary}, arguments...)
	for index, argument := range command {
		command[index] = quoteShellArgument(argument)
	}
	return []byte(strings.Join(command, " ") + "\n")
}
