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
)

type runExitError struct {
	code int
}

func (err *runExitError) Error() string {
	return fmt.Sprintf("program exited with status %d", err.code)
}

func runSourcesWithProgress(
	root string,
	runtimeRoot string,
	environment string,
	compiler string,
	cflags []string,
	ldflags []string,
	configuredEntryPoints []string,
	sources []string,
	arguments []string,
	jobs int,
	verbose bool,
	silent bool,
	progress *progressBar,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	noCache bool,
) error {
	return runSourcesWithProgressExecutable(
		root,
		runtimeRoot,
		environment,
		"",
		"",
		compiler,
		cflags,
		ldflags,
		configuredEntryPoints,
		sources,
		arguments,
		jobs,
		verbose,
		silent,
		progress,
		stdin,
		stdout,
		stderr,
		noCache,
	)
}

func runSourcesWithProgressExecutable(
	root string,
	runtimeRoot string,
	environment string,
	executableSuffix string,
	executableRunner string,
	compiler string,
	cflags []string,
	ldflags []string,
	configuredEntryPoints []string,
	sources []string,
	arguments []string,
	jobs int,
	verbose bool,
	silent bool,
	progress *progressBar,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	noCache bool,
) error {
	if len(sources) == 0 {
		return errors.Join(validateRunLinks(nil), progress.finish())
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
	if err := errors.Join(failures...); err != nil {
		return errors.Join(err, progress.finish())
	}
	link, err := planRunLinkWithLibrariesExecutable(
		root,
		environment,
		executableSuffix,
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		workingDirectory,
	)
	if err != nil {
		return errors.Join(err, progress.finish())
	}

	progress.setTotal(2 + len(sources))
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

	var diagnostics bytes.Buffer
	_, cached, err := linkBinaryWithCache(
		cache,
		compiler,
		ldflags,
		link.objects,
		link.source,
		link.artifact,
		workingDirectory,
		&diagnostics,
	)
	var command []byte
	if verbose && !silent && !cached {
		command = renderLinkCommand(compiler, ldflags, link.objects, link.artifact)
	}
	step := "Linking " + link.display
	if cached {
		step += " (CACHED)"
	}
	progress.complete(step, command)
	if diagnostics.Len() != 0 && (!silent || err != nil) {
		if _, writeError := io.Copy(stderr, &diagnostics); writeError != nil {
			return errors.Join(fmt.Errorf("write link diagnostics: %w", writeError), progress.finish())
		}
	}
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	if err := progress.finish(); err != nil {
		return err
	}

	if verbose && !silent {
		if _, err := stdout.Write(renderRunCommandWithRunner(executableRunner, link.artifact, arguments)); err != nil {
			return fmt.Errorf("write run command: %w", err)
		}
	}
	return runProgramWithRunner(
		executableRunner,
		link.artifact,
		arguments,
		workingDirectory,
		stdin,
		stdout,
		stderr,
	)
}

func planRunLink(
	root string,
	environment string,
	sources []string,
	dependenciesBySource [][]string,
	entryPointsBySource []string,
	rootSourceCount int,
	workingDirectory string,
) (linkJob, error) {
	return planRunLinkWithLibraries(
		root,
		environment,
		sources,
		dependenciesBySource,
		nil,
		entryPointsBySource,
		rootSourceCount,
		workingDirectory,
	)
}

func planRunLinkWithLibraries(
	root string,
	environment string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	workingDirectory string,
) (linkJob, error) {
	return planRunLinkWithLibrariesExecutable(
		root,
		environment,
		"",
		sources,
		dependenciesBySource,
		librariesBySource,
		entryPointsBySource,
		rootSourceCount,
		workingDirectory,
	)
}

func planRunLinkWithLibrariesExecutable(
	root string,
	environment string,
	executableSuffix string,
	sources []string,
	dependenciesBySource [][]string,
	librariesBySource [][]libraryArtifact,
	entryPointsBySource []string,
	rootSourceCount int,
	workingDirectory string,
) (linkJob, error) {
	if len(dependenciesBySource) != len(sources) {
		return linkJob{}, fmt.Errorf(
			"dependency source count does not match sources: %d != %d",
			len(dependenciesBySource),
			len(sources),
		)
	}
	if librariesBySource != nil && len(librariesBySource) != len(sources) {
		return linkJob{}, fmt.Errorf(
			"library source count does not match sources: %d != %d",
			len(librariesBySource),
			len(sources),
		)
	}
	if len(entryPointsBySource) != len(sources) {
		return linkJob{}, fmt.Errorf(
			"entry source count does not match sources: %d != %d",
			len(entryPointsBySource),
			len(sources),
		)
	}
	if rootSourceCount < 0 || rootSourceCount > len(sources) {
		return linkJob{}, fmt.Errorf(
			"root source count is outside source range: %d not in [0, %d]",
			rootSourceCount,
			len(sources),
		)
	}

	links := make([]linkJob, 0)
	entryIndex := -1
	for index := 0; index < rootSourceCount; index++ {
		if entryPointsBySource[index] == "" {
			continue
		}
		entryIndex = index
		links = append(links, linkJob{source: sources[index]})
	}
	if err := validateRunLinks(links); err != nil {
		return linkJob{}, err
	}

	objectIndexes, err := resolveLinkObjectIndexes(
		sources,
		dependenciesBySource,
		entryPointsBySource,
		entryIndex,
		workingDirectory,
	)
	if err != nil {
		return linkJob{}, fmt.Errorf("resolve link objects for %s: %w", sources[entryIndex], err)
	}
	objects := make([]string, 0, len(objectIndexes))
	for _, objectIndex := range objectIndexes {
		object, err := objectFilePath(root, environment, sources[objectIndex])
		if err != nil {
			return linkJob{}, err
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
		return linkJob{}, err
	}
	display := strings.TrimSuffix(sources[entryIndex], filepath.Ext(sources[entryIndex]))
	if filepath.IsAbs(display) {
		if relative, err := filepath.Rel(workingDirectory, display); err == nil {
			display = relative
		}
	}
	return linkJob{
		source:   sources[entryIndex],
		objects:  objects,
		artifact: artifact,
		display:  display,
	}, nil
}

func validateRunLinks(links []linkJob) error {
	if len(links) == 1 {
		return nil
	}
	if len(links) == 0 {
		return errors.New("run requires exactly one entry source, found 0")
	}
	candidates := make([]string, 0, len(links))
	for _, link := range links {
		candidates = append(candidates, link.source)
	}
	return fmt.Errorf(
		"run requires exactly one entry source, found %d: %s",
		len(links),
		strings.Join(candidates, ", "),
	)
}

func runProgram(
	binary string,
	arguments []string,
	workingDirectory string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return runProgramWithRunner(
		"",
		binary,
		arguments,
		workingDirectory,
		stdin,
		stdout,
		stderr,
	)
}

func runProgramWithRunner(
	runner string,
	binary string,
	arguments []string,
	workingDirectory string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	executable, executableArguments := programInvocation(runner, binary, arguments)
	command := exec.Command(executable, executableArguments...)
	command.Dir = workingDirectory
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return &runExitError{code: exitError.ExitCode()}
		}
		return fmt.Errorf("run program %s: %w", binary, err)
	}
	return nil
}

func runProgramExitCode(err error) (int, bool) {
	var exitError *runExitError
	if !errors.As(err, &exitError) {
		return 0, false
	}
	if exitError.code < 1 {
		return 1, true
	}
	return exitError.code, true
}

func renderRunCommand(binary string, arguments []string) []byte {
	return renderRunCommandWithRunner("", binary, arguments)
}

func renderRunCommandWithRunner(runner, binary string, arguments []string) []byte {
	executable, executableArguments := programInvocation(runner, binary, arguments)
	command := append([]string{executable}, executableArguments...)
	for index, argument := range command {
		command[index] = quoteShellArgument(argument)
	}
	return []byte(strings.Join(command, " ") + "\n")
}
