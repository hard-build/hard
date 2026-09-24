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
	resolvers ...*githubSnapshotResolver,
) error {
	return fetchSourcesWithCache(root, "", cflags, sources, jobs, progress, stderr, false, resolvers...)
}

func fetchSourcesWithCache(
	root string,
	environment string,
	cflags []string,
	sources []string,
	jobs int,
	progress *progressBar,
	stderr io.Writer,
	noCache bool,
	resolvers ...*githubSnapshotResolver,
) error {
	resolver := invocationRepositoryResolver(root, progress, resolvers)
	if len(sources) == 0 {
		progress.setTotal(1)
		return errors.Join(resolver.commitDependencies(), progress.finish())
	}
	if jobs < 1 {
		return errors.Join(fmt.Errorf("jobs must be positive: %d", jobs), progress.finish())
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return errors.Join(fmt.Errorf("determine working directory: %w", err), progress.finish())
	}
	cache, err := newArtifactCache(!noCache, resolver)
	if err != nil {
		return errors.Join(err, progress.finish())
	}
	libraryManager := newLibraryManager(
		root,
		"",
		"",
		jobs,
		false,
		false,
		workingDirectory,
		resolver,
		nil,
		progress,
		stderr,
	)
	activity := func(path string, cached bool) {
		message := "Parsing " + buildParsingDisplayPath(root, path, workingDirectory)
		if cached {
			message += " (CACHED)"
		}
		progress.updateStep(message)
	}
	_, _, _, _, _, _, failures, err := discoverBuildSourceClosureWithLibraries(
		root,
		environment,
		"",
		resolver,
		cflags,
		nil,
		sources,
		jobs,
		workingDirectory,
		stderr,
		activity,
		cache,
		libraryManager,
	)
	err = errors.Join(err, errors.Join(failures...))
	progress.setTotal(1)
	if err == nil {
		err = resolver.commitDependencies()
	}
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
	return fetchSourceDependenciesWithLibraries(
		resolver,
		cflags,
		sources,
		jobs,
		workingDirectory,
		stderr,
		activity,
		nil,
	)
}

func fetchSourceDependenciesWithLibraries(
	resolver *githubSnapshotResolver,
	cflags []string,
	sources []string,
	jobs int,
	workingDirectory string,
	stderr io.Writer,
	activity func(string),
	libraryManager *libraryManager,
) error {
	if len(sources) == 0 {
		return nil
	}
	if jobs < 1 {
		return fmt.Errorf("jobs must be positive: %d", jobs)
	}
	if libraryManager == nil {
		var root string
		var progress *progressBar
		if resolver != nil {
			root, progress = resolver.root, resolver.progress
		}
		libraryManager = newLibraryManager(root, "", "", jobs, false, false, workingDirectory, resolver, nil, progress, stderr)
	}
	var parsingActivity func(string, bool)
	if activity != nil {
		parsingActivity = func(path string, _ bool) {
			activity(path)
		}
	}
	_, _, _, _, _, _, failures, err := discoverBuildSourceClosureWithLibraries(
		"",
		"",
		"",
		resolver,
		cflags,
		nil,
		sources,
		jobs,
		workingDirectory,
		stderr,
		parsingActivity,
		nil,
		libraryManager,
	)
	return errors.Join(err, errors.Join(failures...))
}
