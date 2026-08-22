package main

import (
	"errors"
	"fmt"
	"os"
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

	configuration, err := loadConfiguration()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}

	progress := newProgressBar(os.Stdout, -1, parsed.verbose, parsed.silent, parsed.noColor)
	sources, err := discoverSourcesWithProgress(parsed.command, parsed.paths, progress)
	if err != nil {
		err = errors.Join(err, progress.finish())
		fmt.Fprintf(os.Stderr, "hard: %v\n", err)
		os.Exit(1)
	}

	if parsed.command == "build" {
		if err := buildSourcesWithProgress(
			configuration.root,
			configuration.env,
			configuration.cc,
			configuration.cflags,
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

	if parsed.command == "format" {
		progress.setTotal(1 + len(sources))
		if err := formatSourcesWithProgress(
			configuration.root,
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
			configuration.cflags,
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
	if err := testSourcesWithProgressSelection(
		configuration.root,
		configuration.env,
		configuration.cc,
		configuration.cflags,
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

func discoverSourcesWithProgress(command string, paths []string, progress *progressBar) ([]string, error) {
	progress.updateStep("Searching source files")
	return discoverSources(command, paths)
}
