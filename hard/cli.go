package main

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"
)

const defaultFormat = "format.v1"
const defaultJobs = 1

type arguments struct {
	command string
	paths   []string
	verbose bool
	silent  bool
	noColor bool
	noCache bool
	jobs    int
	format  string
	output  string
}

func parseArguments(args []string, stdout, stderr io.Writer) (arguments, error) {
	var parsed arguments
	root := newRootCommand(&parsed)
	root.SetArgs(normalizeJobArguments(args))
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	return parsed, err
}

func newRootCommand(parsed *arguments) *cobra.Command {
	var verbose bool
	var silent bool
	var noColor bool
	var noCache bool
	jobs := defaultJobs
	format := defaultFormat
	var output string

	root := &cobra.Command{
		Use:           "hard",
		Short:         "Build C++ projects without hand-written build files",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("a command is required")
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print debug information")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	root.PersistentFlags().IntVarP(
		&jobs,
		"jobs",
		"j",
		defaultJobs,
		"number of parallel jobs; omit the value or use 0 for all CPUs",
	)
	root.SetHelpCommand(&cobra.Command{
		Use:    "_help",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("unknown command")
		},
	})
	formatCommand := newPathCommand(
		"format",
		"Format C++ sources",
		&verbose,
		&noColor,
		&jobs,
		&format,
		&silent,
		nil,
		nil,
		parsed,
	)
	formatCommand.Flags().StringVar(
		&format,
		"format",
		defaultFormat,
		"format style file under HARD_ROOT/format",
	)
	formatCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	buildCommand := newPathCommand(
		"build",
		"Build C++ sources",
		&verbose,
		&noColor,
		&jobs,
		nil,
		&silent,
		&output,
		&noCache,
		parsed,
	)
	buildCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	buildCommand.Flags().StringVarP(&output, "output", "o", "", "copy binary to file or directory")
	buildCommand.Flags().BoolVar(&noCache, "no-cache", false, "rebuild without using cached results")
	fetchCommand := newPathCommand(
		"fetch",
		"Download C++ dependencies",
		&verbose,
		&noColor,
		&jobs,
		nil,
		&silent,
		nil,
		nil,
		parsed,
	)
	fetchCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	testCommand := newPathCommand(
		"test",
		"Build and run C++ tests",
		&verbose,
		&noColor,
		&jobs,
		nil,
		&silent,
		nil,
		&noCache,
		parsed,
	)
	testCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	testCommand.Flags().BoolVar(&noCache, "no-cache", false, "rebuild and rerun tests without using cached results")
	root.AddCommand(
		formatCommand,
		buildCommand,
		fetchCommand,
		testCommand,
	)

	return root
}

func newPathCommand(
	name string,
	description string,
	verbose *bool,
	noColor *bool,
	jobs *int,
	format *string,
	silent *bool,
	output *string,
	noCache *bool,
	parsed *arguments,
) *cobra.Command {
	return &cobra.Command{
		Use:   name + " [path...]",
		Short: description,
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, paths []string) error {
			jobCount, err := resolveJobCount(*jobs)
			if err != nil {
				return err
			}
			if len(paths) == 0 {
				paths = []string{"."}
			}

			result := arguments{
				command: name,
				paths:   paths,
				verbose: *verbose,
				noColor: *noColor,
				jobs:    jobCount,
			}
			if format != nil {
				result.format = *format
			}
			if silent != nil {
				result.silent = *silent
			}
			if output != nil {
				result.output = *output
			}
			if noCache != nil {
				result.noCache = *noCache
			}
			*parsed = result
			return nil
		},
	}
}

func normalizeJobArguments(args []string) []string {
	normalized := append([]string(nil), args...)
	for index, argument := range normalized {
		if argument == "-j" || argument == "--jobs" {
			normalized[index] = "--jobs=0"
		}
	}
	return normalized
}

func resolveJobCount(jobs int) (int, error) {
	if jobs < 0 {
		return 0, fmt.Errorf("jobs must not be negative: %d", jobs)
	}
	if jobs == 0 {
		return runtime.NumCPU(), nil
	}
	return jobs, nil
}
