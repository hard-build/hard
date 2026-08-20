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
	"sync"

	"github.com/pmezard/go-difflib/difflib"
)

type formatJob struct {
	index  int
	source string
}

type formatResult struct {
	index       int
	output      []byte
	diagnostics []byte
	err         error
}

func formatSources(
	root string,
	format string,
	sources []string,
	jobs int,
	verbose bool,
	silent bool,
	noColor bool,
	stdout io.Writer,
	stderr io.Writer,
) error {
	progress := newProgressBar(stdout, len(sources), verbose, silent, noColor)
	return formatSourcesWithProgress(
		root,
		format,
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

func formatSourcesWithProgress(
	root string,
	format string,
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

	formatPath, err := resolveFormatPath(root, format)
	if err != nil {
		return errors.Join(err, progress.finish())
	}

	workerCount := jobs
	if workerCount > len(sources) {
		workerCount = len(sources)
	}

	queue := make(chan formatJob)
	results := make(chan formatResult, len(sources))
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

					var output bytes.Buffer
					var diagnostics bytes.Buffer
					fatal, diff, err := formatSource(
						formatPath,
						job.source,
						verbose,
						silent,
						noColor,
						&output,
						&diagnostics,
					)
					if fatal {
						stopWork()
					}
					progress.complete(job.source, diff)
					results <- formatResult{
						index:       job.index,
						output:      append([]byte(nil), output.Bytes()...),
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
		for index, source := range sources {
			select {
			case <-stop:
				return
			case queue <- formatJob{index: index, source: source}:
			}
		}
	}()

	workers.Wait()
	close(results)
	ordered := make([]*formatResult, len(sources))
	for result := range results {
		result := result
		ordered[result.index] = &result
	}

	var failures []error
	if err := progress.finish(); err != nil {
		failures = append(failures, err)
	}
	for _, result := range ordered {
		if result == nil {
			continue
		}
		if !silent {
			if _, err := io.Copy(stdout, bytes.NewReader(result.output)); err != nil {
				return fmt.Errorf("write format output: %w", err)
			}
		}
		if _, err := io.Copy(stderr, bytes.NewReader(result.diagnostics)); err != nil {
			return fmt.Errorf("write format diagnostics: %w", err)
		}
		if result.err != nil {
			failures = append(failures, result.err)
		}
	}
	return errors.Join(failures...)
}

func formatSource(
	formatPath string,
	source string,
	verbose bool,
	silent bool,
	noColor bool,
	stdout io.Writer,
	stderr io.Writer,
) (bool, []byte, error) {
	var before []byte
	var err error
	if verbose && !silent {
		before, err = os.ReadFile(source)
		if err != nil {
			return false, nil, fmt.Errorf("read %s before formatting: %w", source, err)
		}
	}

	command := exec.Command(
		"clang-format",
		"--style=file:"+formatPath,
		"-i",
		source,
	)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return true, nil, fmt.Errorf("run clang-format: %w", err)
		}
		return false, nil, fmt.Errorf("format %s: %w", source, err)
	}
	if !verbose || silent {
		return false, nil, nil
	}

	after, err := os.ReadFile(source)
	if err != nil {
		return false, nil, fmt.Errorf("read %s after formatting: %w", source, err)
	}
	diff, err := sourceDiff(before, after, source, noColor)
	return false, diff, err
}

func sourceDiff(before, after []byte, source string, noColor bool) ([]byte, error) {
	var output bytes.Buffer
	err := difflib.WriteUnifiedDiff(&output, difflib.UnifiedDiff{
		A:        diffLines(before),
		B:        diffLines(after),
		FromFile: source,
		ToFile:   source,
		Context:  3,
	})
	if err != nil {
		return nil, fmt.Errorf("diff %s: %w", source, err)
	}
	if noColor || output.Len() == 0 {
		return output.Bytes(), nil
	}
	return colorizeDiff(output.String()), nil
}

func diffLines(contents []byte) []string {
	if len(contents) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(contents), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func colorizeDiff(diff string) []byte {
	var output strings.Builder
	for _, line := range strings.SplitAfter(diff, "\n") {
		color := ""
		switch {
		case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			color = "\x1b[1m"
		case strings.HasPrefix(line, "@@"):
			color = progressCyan
		case strings.HasPrefix(line, "-"):
			color = "\x1b[31m"
		case strings.HasPrefix(line, "+"):
			color = progressGreen
		}
		if color == "" {
			output.WriteString(line)
			continue
		}
		output.WriteString(color)
		output.WriteString(strings.TrimSuffix(line, "\n"))
		output.WriteString(progressReset)
		if strings.HasSuffix(line, "\n") {
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}

func resolveFormatPath(root, format string) (string, error) {
	formatDirectory := filepath.Join(root, "format")
	if format == "" {
		return "", fmt.Errorf("format file is empty")
	}
	if filepath.IsAbs(format) {
		return "", fmt.Errorf("format file must be relative to %s", formatDirectory)
	}

	formatPath := filepath.Join(formatDirectory, format)
	relativePath, err := filepath.Rel(formatDirectory, formatPath)
	if err != nil {
		return "", fmt.Errorf("resolve format file %s: %w", format, err)
	}
	if pathEscapesDirectory(relativePath) {
		return "", fmt.Errorf("format file escapes %s: %s", formatDirectory, format)
	}

	info, err := os.Stat(formatPath)
	if err != nil {
		return "", fmt.Errorf("format file %s: %w", formatPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("format file is not a regular file: %s", formatPath)
	}

	realDirectory, err := filepath.EvalSymlinks(formatDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve format directory %s: %w", formatDirectory, err)
	}
	realDirectory, err = filepath.Abs(realDirectory)
	if err != nil {
		return "", fmt.Errorf("make format directory absolute %s: %w", formatDirectory, err)
	}
	realFormatPath, err := filepath.EvalSymlinks(formatPath)
	if err != nil {
		return "", fmt.Errorf("resolve format file %s: %w", formatPath, err)
	}
	realFormatPath, err = filepath.Abs(realFormatPath)
	if err != nil {
		return "", fmt.Errorf("make format file absolute %s: %w", formatPath, err)
	}
	relativeRealPath, err := filepath.Rel(realDirectory, realFormatPath)
	if err != nil {
		return "", fmt.Errorf("compare format file %s with %s: %w", formatPath, formatDirectory, err)
	}
	if pathEscapesDirectory(relativeRealPath) {
		return "", fmt.Errorf("format file resolves outside %s: %s", formatDirectory, format)
	}

	return formatPath, nil
}

func pathEscapesDirectory(path string) bool {
	return path == ".." ||
		strings.HasPrefix(path, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(path)
}
