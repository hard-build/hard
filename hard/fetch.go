package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func fetchSources(
	root string,
	cflags []string,
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
	progress := newProgressBar(stdout, 1, verbose, silent, noColor)
	return fetchSourcesWithProgress(root, cflags, sources, jobs, progress, stderr)
}

func fetchSourcesWithProgress(
	root string,
	cflags []string,
	sources []string,
	jobs int,
	progress *progressBar,
	stderr io.Writer,
) error {
	if len(sources) == 0 {
		progress.setTotal(1)
		return progress.finish()
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return errors.Join(fmt.Errorf("determine working directory: %w", err), progress.finish())
	}
	resolver := newGitHubSnapshotResolver(root, progress)
	activity := func(path string) {
		progress.updateStep("Parsing " + buildParsingDisplayPath(root, path, workingDirectory))
	}
	err = fetchSourceDependenciesWithActivity(
		resolver,
		cflags,
		sources,
		jobs,
		workingDirectory,
		stderr,
		activity,
	)
	progress.setTotal(1)
	return errors.Join(err, progress.finish())
}

func fetchSourceDependencies(
	resolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
) error {
	return fetchSourceDependenciesWithActivity(
		resolver,
		cflags,
		sources,
		jobs,
		workingDirectory,
		stderr,
		nil,
	)
}

func fetchSourceDependenciesWithActivity(
	resolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
	activity func(string),
) error {
	if len(sources) == 0 {
		return nil
	}
	if jobs < 1 {
		return fmt.Errorf("jobs must be positive: %d", jobs)
	}
	_, _, _, failures, err := discoverBuildSourceClosureWithActivity(
		resolver,
		cflags,
		nil,
		sources,
		jobs,
		workingDirectory,
		stderr,
		activity,
	)
	return errors.Join(err, errors.Join(failures...))
}
