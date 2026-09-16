package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	if err := runHard(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if code, ok := runProgramExitCode(err); ok {
			os.Exit(code)
		}
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}
}

func runHard(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var options projectOptions
	parsed, err := parseArguments(args, stdout, stderr, &options)
	if err != nil || parsed.command == "" {
		return err
	}
	if parsed.command == "version" {
		return writeVersion(stdout)
	}
	runtimeRoot, err := executableRuntimeRoot()
	if err != nil {
		return err
	}
	configuration, err := loadConfiguration(runtimeRoot)
	if err != nil {
		return err
	}
	if parsed.command == "environment" {
		return writeEnvironmentReport(configuration, parsed.noColor, stdout)
	}
	return runConfiguredCommand(parsed, options, configuration, stdin, stdout, stderr)
}

func runConfiguredCommand(parsed arguments, options projectOptions, configuration configuration, stdin io.Reader, stdout, stderr io.Writer) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	project, session, err := prepareProject(&parsed, options, configuration.root, workingDirectory)
	if err != nil {
		return err
	}
	defer session.close()
	paths, excluded := project.sourcePaths(parsed)
	for {
		progress := newProgressBar(stdout, -1, parsed.verbose, parsed.silent, parsed.noColor)
		current := configuration
		var resolver *githubSnapshotResolver
		diagnostics := stderr
		var staged *resolutionWriter
		if session != nil {
			current.root, err = session.view(configuration, progress)
			if err != nil {
				return errors.Join(err, progress.finish())
			}
			resolver = newGitHubSnapshotResolver(current.root, progress)
			resolver.session = session
			staged = &resolutionWriter{target: stderr}
			session.onCommit = staged.activate
			diagnostics = staged
		}
		progress.updateStep("Searching source files")
		sources, err := discoverSourcesFrom(parsed.command, paths, workingDirectory, excluded)
		if err != nil {
			return errors.Join(err, progress.finish())
		}
		err = executeSourceCommand(parsed, current, sources, progress, resolver, stdin, stdout, diagnostics)
		if session != nil && session.changed {
			// Refresh the locked include view; no binary was executed.
			continue
		}
		if staged != nil {
			err = errors.Join(err, staged.activate())
		}
		return err
	}
}

func executeSourceCommand(parsed arguments, configuration configuration, sources []string, progress *progressBar, resolver *githubSnapshotResolver, stdin io.Reader, stdout, stderr io.Writer) error {
	cflags := effectiveCFlags(configuration.cflags, configuration.root, configuration.runtimeRoot)
	switch parsed.command {
	case "build":
		return buildSourcesWithProgressExecutable(
			configuration.root, configuration.runtimeRoot, configuration.env,
			configuration.executableSuffix, configuration.cc, cflags, configuration.ldflags,
			configuration.entrypoints, sources, parsed.output, parsed.jobs,
			parsed.verbose, parsed.silent, progress, stderr, parsed.noCache, resolver,
		)
	case "run":
		return runSourcesWithProgressExecutable(
			configuration.root, configuration.runtimeRoot, configuration.env,
			configuration.executableSuffix, configuration.executableRunner,
			configuration.cc, cflags, configuration.ldflags, configuration.entrypoints,
			sources, parsed.programArguments, parsed.jobs, parsed.verbose, parsed.silent,
			progress, stdin, stdout, stderr, parsed.noCache, resolver,
		)
	case "format":
		progress.setTotal(1 + len(sources))
		return formatSourcesWithProgress(
			configuration.runtimeRoot, parsed.format, sources, parsed.jobs,
			parsed.verbose, parsed.silent, parsed.noColor, progress, stdout, stderr,
		)
	case "fetch":
		return fetchSourcesWithCache(configuration.root, configuration.env, cflags, sources, parsed.jobs, progress, stderr, parsed.noCache, resolver)
	case "test":
		return testSourcesWithProgressSelectionExecutable(
			configuration.root, configuration.runtimeRoot, configuration.env,
			configuration.executableSuffix, configuration.executableRunner, configuration.cc,
			cflags, configuration.ldflags, sources, parsed.jobs, parsed.verbose, parsed.silent,
			parsed.noColor, progress, stdout, stderr, parsed.noCache,
			parsed.listTests, parsed.testSelectors, resolver,
		)
	}
	return fmt.Errorf("unknown command: %s", parsed.command)
}

func executableRuntimeRoot() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine executable path: %w", err)
	}
	return resolveRuntimeRoot(executable)
}

func resolveRuntimeRoot(executable string) (string, error) {
	realExecutable, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executable path %s: %w", executable, err)
	}
	realExecutable, err = filepath.Abs(realExecutable)
	if err != nil {
		return "", fmt.Errorf("make executable path absolute %s: %w", realExecutable, err)
	}
	return filepath.Dir(realExecutable), nil
}

func discoverSourcesWithProgress(command string, paths []string, progress *progressBar) ([]string, error) {
	progress.updateStep("Searching source files")
	return discoverSources(command, paths)
}
