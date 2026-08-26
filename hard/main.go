package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	parsed, err := parseArguments(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}
	if parsed.command == "" {
		return
	}

	runtimeRoot, err := executableRuntimeRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}
	configuration, err := loadConfiguration(runtimeRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}
	cflags := effectiveCFlags(configuration.cflags, configuration.root, configuration.runtimeRoot)
	if parsed.command == "environment" {
		if err := writeEnvironmentReport(configuration, cflags, parsed.noColor, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "hard: %v\n", err)
			os.Exit(1)
		}
		return
	}

	progress := newProgressBar(os.Stdout, -1, parsed.verbose, parsed.silent, parsed.noColor)
	sources, err := discoverSourcesWithProgress(parsed.command, parsed.paths, progress)
	if err != nil {
		err = errors.Join(err, progress.finish())
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}

	if parsed.command == "build" {
		if err := buildSourcesWithProgressExecutable(
			configuration.root,
			configuration.runtimeRoot,
			configuration.env,
			configuration.executableSuffix,
			configuration.cc,
			cflags,
			configuration.ldflags,
			configuration.entrypoints,
			sources,
			parsed.output,
			parsed.jobs,
			parsed.verbose,
			parsed.silent,
			progress,
			os.Stderr,
			parsed.noCache,
		); err != nil {
			fmt.Fprintf(os.Stderr, "hard: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if parsed.command == "run" {
		err := runSourcesWithProgressExecutable(
			configuration.root,
			configuration.runtimeRoot,
			configuration.env,
			configuration.executableSuffix,
			configuration.executableRunner,
			configuration.cc,
			cflags,
			configuration.ldflags,
			configuration.entrypoints,
			sources,
			parsed.programArguments,
			parsed.jobs,
			parsed.verbose,
			parsed.silent,
			progress,
			os.Stdin,
			os.Stdout,
			os.Stderr,
			parsed.noCache,
		)
		if err != nil {
			if code, ok := runProgramExitCode(err); ok {
				os.Exit(code)
			}
			fmt.Fprintf(os.Stderr, "hard: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if parsed.command == "format" {
		progress.setTotal(1 + len(sources))
		if err := formatSourcesWithProgress(
			configuration.runtimeRoot,
			parsed.format,
			sources,
			parsed.jobs,
			parsed.verbose,
			parsed.silent,
			parsed.noColor,
			progress,
			os.Stdout,
			os.Stderr,
		); err != nil {
			fmt.Fprintf(os.Stderr, "hard: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if parsed.command == "fetch" {
		if err := fetchSourcesWithProgress(
			configuration.root,
			cflags,
			sources,
			parsed.jobs,
			progress,
			os.Stderr,
		); err != nil {
			fmt.Fprintf(os.Stderr, "hard: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := testSourcesWithProgressSelectionExecutable(
		configuration.root,
		configuration.runtimeRoot,
		configuration.env,
		configuration.executableSuffix,
		configuration.executableRunner,
		configuration.cc,
		cflags,
		configuration.ldflags,
		sources,
		parsed.jobs,
		parsed.verbose,
		parsed.silent,
		parsed.noColor,
		progress,
		os.Stdout,
		os.Stderr,
		parsed.noCache,
		parsed.listTests,
		parsed.testSelectors,
	); err != nil {
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}
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
