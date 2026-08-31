package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const defaultFormat = "format.v1"
const defaultJobs = 1

type arguments struct {
	command          string
	paths            []string
	programArguments []string
	verbose          bool
	silent           bool
	noColor          bool
	noCache          bool
	listTests        bool
	testSelectors    []string
	jobs             int
	format           string
	output           string
}

func parseArguments(args []string, stdout, stderr io.Writer) (arguments, error) {
	var parsed arguments
	completionRequest := isShellCompletionRequest(args)
	completionGeneration := len(args) > 0 && args[0] == "completion"
	root := newRootCommand(&parsed, completionRequest || completionGeneration)
	if completionRequest {
		root.SetArgs(append([]string(nil), args...))
	} else {
		root.SetArgs(normalizeJobArguments(args))
	}
	var completionOutput bytes.Buffer
	if completionRequest {
		root.SetOut(&completionOutput)
	} else {
		root.SetOut(stdout)
	}
	root.SetErr(stderr)

	err := root.Execute()
	if completionRequest {
		if outputErr := writeShellCompletion(stdout, completionOutput.String()); err == nil {
			err = outputErr
		}
	}
	return parsed, err
}

func newRootCommand(parsed *arguments, includeWrapperFlags bool) *cobra.Command {
	var verbose bool
	var silent bool
	var noColor bool
	var noCache bool
	var listTests bool
	var testSelectors []string
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
			HiddenDefaultCmd: true,
		},
	}
	root.SetHelpTemplate(root.HelpTemplate() + `
Wrapper options:
      --target string   select an execution target (supported: host, linux64, windows64, versioned targets, docker://image)
`)

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print debug information")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	if includeWrapperFlags {
		root.PersistentFlags().String("target", "", "select an execution target (supported: host, linux64, windows64, versioned targets, docker://image)")
	}
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
		nil,
		nil,
		parsed,
	)
	formatCommand.Flags().StringVar(
		&format,
		"format",
		defaultFormat,
		"format style file installed with hard",
	)
	if err := formatCommand.RegisterFlagCompletionFunc(
		"format",
		cobra.FixedCompletions(
			[]string{defaultFormat},
			cobra.ShellCompDirectiveNoFileComp,
		),
	); err != nil {
		panic(err)
	}
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
		nil,
		nil,
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
		nil,
		nil,
		parsed,
	)
	fetchCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	runCommand := newRunCommand(
		&verbose,
		&noColor,
		&jobs,
		&silent,
		&noCache,
		parsed,
	)
	runCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	runCommand.Flags().BoolVar(&noCache, "no-cache", false, "rebuild without using cached results")
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
		&listTests,
		&testSelectors,
		parsed,
	)
	testCommand.Use = "test [--list-tests] [--test=<selector>]... [path...]"
	testCommand.Flags().BoolVarP(&silent, "silent", "s", false, "only print errors")
	testCommand.Flags().BoolVar(&noCache, "no-cache", false, "rebuild and rerun tests without using cached results")
	testCommand.Flags().BoolVar(&listTests, "list-tests", false, "list tests without running them")
	testCommand.Flags().StringArrayVar(&testSelectors, "test", nil, "run tests matching selector; may be repeated")
	environmentCommand := newEnvironmentCommand(
		&verbose,
		&noColor,
		&jobs,
		parsed,
	)
	versionCommand := newVersionCommand(
		&verbose,
		&noColor,
		&jobs,
		parsed,
	)
	root.AddCommand(
		formatCommand,
		buildCommand,
		environmentCommand,
		fetchCommand,
		runCommand,
		testCommand,
		versionCommand,
	)

	return root
}

func newEnvironmentCommand(
	verbose *bool,
	noColor *bool,
	jobs *int,
	parsed *arguments,
) *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Describe the build environment",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			jobCount, err := resolveJobCount(*jobs)
			if err != nil {
				return err
			}
			*parsed = arguments{
				command: "environment",
				verbose: *verbose,
				noColor: *noColor,
				jobs:    jobCount,
			}
			return nil
		},
	}
}

func newVersionCommand(
	verbose *bool,
	noColor *bool,
	jobs *int,
	parsed *arguments,
) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print hard version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			jobCount, err := resolveJobCount(*jobs)
			if err != nil {
				return err
			}
			*parsed = arguments{
				command: "version",
				verbose: *verbose,
				noColor: *noColor,
				jobs:    jobCount,
			}
			return nil
		},
	}
}

func isShellCompletionRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd
}

func writeShellCompletion(output io.Writer, completion string) error {
	for _, line := range strings.SplitAfter(completion, "\n") {
		value := strings.TrimSuffix(line, "\n")
		if value == "_help" || strings.HasPrefix(value, "_help\t") {
			continue
		}
		if _, err := io.WriteString(output, line); err != nil {
			return fmt.Errorf("write shell completion: %w", err)
		}
	}
	return nil
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
	listTests *bool,
	testSelectors *[]string,
	parsed *arguments,
) *cobra.Command {
	return &cobra.Command{
		Use:   name + " [path...]",
		Short: description,
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, paths []string) error {
			if listTests != nil && testSelectors != nil {
				if err := validateTestSelection(*listTests, *testSelectors); err != nil {
					return err
				}
			}
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
			if listTests != nil {
				result.listTests = *listTests
			}
			if testSelectors != nil {
				result.testSelectors = append([]string(nil), (*testSelectors)...)
			}
			*parsed = result
			return nil
		},
	}
}

func newRunCommand(
	verbose *bool,
	noColor *bool,
	jobs *int,
	silent *bool,
	noCache *bool,
	parsed *arguments,
) *cobra.Command {
	return &cobra.Command{
		Use:   "run [path...] [-- program-argument...]",
		Short: "Build and run a C++ program",
		Args:  cobra.ArbitraryArgs,
		RunE: func(command *cobra.Command, values []string) error {
			jobCount, err := resolveJobCount(*jobs)
			if err != nil {
				return err
			}
			paths := values
			var programArguments []string
			if separator := command.ArgsLenAtDash(); separator >= 0 {
				paths = values[:separator]
				programArguments = append([]string(nil), values[separator:]...)
			}
			if len(paths) == 0 {
				paths = []string{"."}
			}
			*parsed = arguments{
				command:          "run",
				paths:            paths,
				programArguments: programArguments,
				verbose:          *verbose,
				silent:           *silent,
				noColor:          *noColor,
				noCache:          *noCache,
				jobs:             jobCount,
			}
			return nil
		},
	}
}

func validateTestSelection(listTests bool, selectors []string) error {
	if listTests && len(selectors) != 0 {
		return errors.New("--list-tests and --test cannot be used together")
	}
	for _, selector := range selectors {
		if selector == "" {
			return errors.New("test selector must not be empty")
		}
		if strings.Contains(selector, ":") {
			return fmt.Errorf("test selector must not contain ':': %s", selector)
		}
		if strings.Contains(selector, "-") {
			return fmt.Errorf("test selector must not contain '-': %s", selector)
		}
	}
	return nil
}

func normalizeJobArguments(args []string) []string {
	normalized := append([]string(nil), args...)
	for index, argument := range normalized {
		if argument == "--" {
			break
		}
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
