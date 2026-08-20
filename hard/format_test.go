package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFormatPath(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, "format.v1")
	writeTestFormatFile(t, root, "team/debug.yaml")

	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "default format",
			format: defaultFormat,
			want:   filepath.Join(root, "format", "format.v1"),
		},
		{
			name:   "nested format",
			format: "team/debug.yaml",
			want:   filepath.Join(root, "format", "team", "debug.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveFormatPath(root, tt.format)
			if err != nil {
				t.Fatalf("resolveFormatPath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveFormatPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveFormatPathRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()

	tests := []string{
		"",
		filepath.Join(string(filepath.Separator), "tmp", "format.yaml"),
		"../format.yaml",
		"nested/../../format.yaml",
	}
	for _, format := range tests {
		t.Run(format, func(t *testing.T) {
			if _, err := resolveFormatPath(root, format); err == nil {
				t.Fatalf("resolveFormatPath(%q) error = nil", format)
			}
		})
	}
}

func TestResolveFormatPathRejectsInvalidFile(t *testing.T) {
	root := t.TempDir()
	formatDirectory := filepath.Join(root, "format")
	if err := os.MkdirAll(filepath.Join(formatDirectory, "directory"), 0o755); err != nil {
		t.Fatalf("create format directory: %v", err)
	}

	for _, format := range []string{"missing.v1", "directory"} {
		t.Run(format, func(t *testing.T) {
			if _, err := resolveFormatPath(root, format); err == nil {
				t.Fatalf("resolveFormatPath(%q) error = nil", format)
			}
		})
	}
}

func TestResolveFormatPathRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, "../outside.yaml")
	formatDirectory := filepath.Join(root, "format")
	if err := os.MkdirAll(formatDirectory, 0o755); err != nil {
		t.Fatalf("create format directory: %v", err)
	}
	if err := os.Symlink("../outside.yaml", filepath.Join(formatDirectory, "external.yaml")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	if _, err := resolveFormatPath(root, "external.yaml"); err == nil {
		t.Fatal("resolveFormatPath() error = nil")
	}
}

func TestResolveFormatPathAllowsInternalSymlink(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, "real.yaml")
	formatDirectory := filepath.Join(root, "format")
	formatPath := filepath.Join(formatDirectory, "alias.yaml")
	if err := os.Symlink("real.yaml", formatPath); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	got, err := resolveFormatPath(root, "alias.yaml")
	if err != nil {
		t.Fatalf("resolveFormatPath() error = %v", err)
	}
	if got != formatPath {
		t.Fatalf("resolveFormatPath() = %q, want %q", got, formatPath)
	}
}

func TestFormatSourcesRunsOneProcessPerSource(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	log, _ := installFakeClangFormat(t)
	sources := []string{"first.cpp", "directory/second.h"}

	err := formatSources(root, defaultFormat, sources, defaultJobs, false, false, false, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}

	formatPath := filepath.Join(root, "format", defaultFormat)
	want := strings.Join([]string{
		"CALL",
		"--style=file:" + formatPath,
		"-i",
		"first.cpp",
		"CALL",
		"--style=file:" + formatPath,
		"-i",
		"directory/second.h",
		"",
	}, "\n")
	if got := readTestFile(t, log); got != want {
		t.Fatalf("clang-format log = %q, want %q", got, want)
	}
}

func TestFormatSourcesContinuesAfterFileFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	log, _ := installFakeClangFormat(t)
	sources := []string{"before.cpp", "fail.cpp", "after.cpp"}

	err := formatSources(root, defaultFormat, sources, defaultJobs, false, false, false, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("formatSources() error = nil")
	}
	if !strings.Contains(err.Error(), "fail.cpp") {
		t.Fatalf("formatSources() error = %q, want it to contain fail.cpp", err)
	}

	got := readTestFile(t, log)
	if strings.Count(got, "CALL\n") != len(sources) {
		t.Fatalf("clang-format calls = %q, want %d calls", got, len(sources))
	}
	if !strings.Contains(got, "after.cpp\n") {
		t.Fatalf("clang-format log = %q, want call after failure", got)
	}
}

func TestFormatSourcesStopsWhenClangFormatIsMissing(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	t.Setenv("PATH", "")

	err := formatSources(
		root,
		defaultFormat,
		[]string{"first.cpp", "second.cpp"},
		defaultJobs,
		false,
		false,
		false,
		io.Discard,
		io.Discard,
	)
	if err == nil {
		t.Fatal("formatSources() error = nil")
	}
	if !strings.Contains(err.Error(), "clang-format") {
		t.Fatalf("formatSources() error = %q, want it to contain clang-format", err)
	}
}

func TestFormatSourcesAllowsEmptySelection(t *testing.T) {
	t.Setenv("PATH", "")

	if err := formatSources(
		t.TempDir(),
		"missing.v1",
		nil,
		defaultJobs,
		false,
		false,
		false,
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}
}

func TestFormatSourcesOutputModes(t *testing.T) {
	tests := []struct {
		name       string
		verbose    bool
		silent     bool
		noColor    bool
		wantDiff   bool
		wantOutput bool
		wantColor  bool
	}{
		{name: "normal progress", wantOutput: true, wantColor: true},
		{name: "colored verbose diff", verbose: true, wantDiff: true, wantOutput: true, wantColor: true},
		{name: "plain verbose diff", verbose: true, noColor: true, wantDiff: true, wantOutput: true},
		{name: "silent", silent: true},
		{name: "silent overrides verbose", verbose: true, silent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFormatFile(t, root, defaultFormat)
			_, binDirectory := installFakeClangFormat(t)
			t.Setenv("PATH", binDirectory)
			source := filepath.Join(t.TempDir(), "source.cpp")
			before := "int main(){return 0;}\n"
			after := "int main() { return 0; }\n"
			writeTestFile(t, source, before)
			t.Setenv("FORMAT_REPLACEMENT", after)

			var stdout bytes.Buffer
			err := formatSources(
				root,
				defaultFormat,
				[]string{source},
				defaultJobs,
				tt.verbose,
				tt.silent,
				tt.noColor,
				&stdout,
				io.Discard,
			)
			if err != nil {
				t.Fatalf("formatSources() error = %v", err)
			}
			if got := readTestFile(t, source); got != after {
				t.Fatalf("formatted source = %q, want %q", got, after)
			}

			gotOutput := stdout.String()
			if (gotOutput != "") != tt.wantOutput {
				t.Fatalf("formatSources() output = %q, wantOutput = %t", gotOutput, tt.wantOutput)
			}
			gotDiff := strings.Contains(gotOutput, "--- "+source)
			if gotDiff != tt.wantDiff {
				t.Fatalf("formatSources() output = %q, wantDiff = %t", gotOutput, tt.wantDiff)
			}
			if strings.Contains(gotOutput, "\x1b[") != tt.wantColor {
				t.Fatalf("formatSources() output color = %q, wantColor = %t", gotOutput, tt.wantColor)
			}
			if tt.wantOutput && (!strings.Contains(gotOutput, "[1/1]") || !strings.Contains(gotOutput, source)) {
				t.Fatalf("formatSources() output = %q, want progress entry", gotOutput)
			}
		})
	}
}

func TestFormatSourcesContinuesSearchProgress(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	_, _ = installFakeClangFormat(t)
	source := filepath.Join(t.TempDir(), "source.cpp")
	writeTestFile(t, source, "int main() { return 0; }\n")

	var stdout bytes.Buffer
	progress := newProgressBar(&stdout, -1, true, false, true)
	progress.updateStep("Searching source files")
	progress.setTotal(2)
	if err := formatSourcesWithProgress(
		root,
		defaultFormat,
		[]string{source},
		1,
		true,
		false,
		true,
		progress,
		&stdout,
		io.Discard,
	); err != nil {
		t.Fatalf("formatSourcesWithProgress() error = %v", err)
	}

	output := stdout.String()
	wants := []string{
		"[1/?] Searching source files\n",
		"[2/2] " + source + "\n",
	}
	previous := -1
	for _, want := range wants {
		index := strings.Index(output, want)
		if index < 0 {
			t.Fatalf("format output does not contain %q: %q", want, output)
		}
		if index <= previous {
			t.Fatalf("format output order is wrong for %q: %q", want, output)
		}
		previous = index
	}
}

func TestFormatSourcesSilentModeOnlyWritesErrors(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	binDirectory := t.TempDir()
	executable := filepath.Join(binDirectory, "clang-format")
	script := "#!/bin/sh\n" +
		"printf 'regular output\\n'\n" +
		"printf 'format error\\n' >&2\n" +
		"exit 7\n"
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake clang-format: %v", err)
	}
	t.Setenv("PATH", binDirectory)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := formatSources(
		root,
		defaultFormat,
		[]string{"source.cpp"},
		defaultJobs,
		true,
		true,
		false,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("formatSources() error = nil")
	}
	if stdout.Len() != 0 {
		t.Fatalf("formatSources() stdout = %q, want empty output", stdout.String())
	}
	if got := stderr.String(); got != "format error\n" {
		t.Fatalf("formatSources() stderr = %q, want format error", got)
	}
}

func TestFormatSourcesDoesNotWriteDiffForUnchangedFile(t *testing.T) {
	systemPath := os.Getenv("PATH")
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	_, binDirectory := installFakeClangFormat(t)
	t.Setenv("PATH", binDirectory+string(os.PathListSeparator)+systemPath)
	source := filepath.Join(t.TempDir(), "source.cpp")
	writeTestFile(t, source, "int main() { return 0; }\n")

	var stdout bytes.Buffer
	err := formatSources(
		root,
		defaultFormat,
		[]string{source},
		defaultJobs,
		true,
		false,
		false,
		&stdout,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}
	if strings.Contains(stdout.String(), "--- "+source) {
		t.Fatalf("formatSources() output = %q, want no diff", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[1/1]") || !strings.Contains(stdout.String(), source) {
		t.Fatalf("formatSources() output = %q, want progress entry", stdout.String())
	}
}

func TestFormatSourcesDoesNotNeedDiffWithoutVerbose(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	_, _ = installFakeClangFormat(t)

	if err := formatSources(
		root,
		defaultFormat,
		[]string{"source.cpp"},
		defaultJobs,
		false,
		false,
		false,
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}
}

func TestFormatSourcesRunsInParallel(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	log := installParallelClangFormat(t)

	err := formatSources(
		root,
		defaultFormat,
		[]string{"first.cpp", "second.cpp", "third.cpp"},
		2,
		false,
		true,
		true,
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}

	active := 0
	maxActive := 0
	started := 0
	for _, line := range strings.Split(strings.TrimSpace(readTestFile(t, log)), "\n") {
		switch {
		case strings.HasPrefix(line, "START "):
			active++
			started++
			if active > maxActive {
				maxActive = active
			}
		case strings.HasPrefix(line, "END "):
			active--
		default:
			t.Fatalf("unexpected parallel log line %q", line)
		}
	}
	if started != 3 {
		t.Fatalf("started files = %d, want 3", started)
	}
	if maxActive != 2 {
		t.Fatalf("maximum active formatters = %d, want 2; log = %q", maxActive, readTestFile(t, log))
	}
	if active != 0 {
		t.Fatalf("active formatters after completion = %d", active)
	}
}

func TestFormatSourcesWritesVerboseProgressAndDiffTogether(t *testing.T) {
	systemPath := os.Getenv("PATH")
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	_, binDirectory := installFakeClangFormat(t)
	t.Setenv("PATH", binDirectory+string(os.PathListSeparator)+systemPath)
	sourceDirectory := t.TempDir()
	first := filepath.Join(sourceDirectory, "first.cpp")
	second := filepath.Join(sourceDirectory, "second.cpp")
	writeTestFile(t, first, "int first(){return 1;}\n")
	writeTestFile(t, second, "int second(){return 2;}\n")
	t.Setenv("FORMAT_REPLACEMENT", "int formatted() { return 0; }\n")

	var stdout bytes.Buffer
	err := formatSources(
		root,
		defaultFormat,
		[]string{first, second},
		2,
		true,
		false,
		true,
		&stdout,
		io.Discard,
	)
	if err != nil {
		t.Fatalf("formatSources() error = %v", err)
	}

	output := stdout.String()
	firstProgress := strings.Index(output, first)
	firstDiff := strings.Index(output, "--- "+first)
	secondProgress := strings.Index(output, second)
	secondDiff := strings.Index(output, "--- "+second)
	if firstProgress < 0 || firstDiff < firstProgress || secondProgress < 0 || secondDiff < secondProgress {
		t.Fatalf("formatSources() output does not keep progress with its diff: %q", output)
	}
	if strings.Count(output, "[1/2]") != 1 || strings.Count(output, "[2/2]") != 1 {
		t.Fatalf("formatSources() output counters = %q", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("formatSources() output contains color: %q", output)
	}
}

func TestFormatSourcesStopsSchedulingAfterFatalStartError(t *testing.T) {
	root := t.TempDir()
	writeTestFormatFile(t, root, defaultFormat)
	binDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDirectory, "clang-format"), nil, 0o644); err != nil {
		t.Fatalf("write non-executable clang-format: %v", err)
	}
	t.Setenv("PATH", binDirectory)

	var output bytes.Buffer
	err := formatSources(
		root,
		defaultFormat,
		[]string{"first.cpp", "second.cpp", "third.cpp"},
		1,
		false,
		false,
		true,
		&output,
		io.Discard,
	)
	if err == nil {
		t.Fatal("formatSources() error = nil")
	}
	if !strings.Contains(err.Error(), "clang-format") {
		t.Fatalf("formatSources() error = %q, want clang-format", err)
	}
	if !strings.Contains(output.String(), "[1/3]") || strings.Contains(output.String(), "[3/3]") {
		t.Fatalf("formatSources() progress = %q, want one scheduled file", output.String())
	}
}

func TestSourceDiff(t *testing.T) {
	tests := []struct {
		name    string
		noColor bool
		before  string
		after   string
		want    []string
		color   bool
	}{
		{
			name:   "colored change",
			before: "int value=1;\n",
			after:  "int value = 1;\n",
			want:   []string{"--- directory/source.cpp", "+++ directory/source.cpp", "-int value=1;", "+int value = 1;"},
			color:  true,
		},
		{
			name:    "plain change",
			noColor: true,
			before:  "int value=1;\n",
			after:   "int value = 1;\n",
			want:    []string{"--- directory/source.cpp", "+++ directory/source.cpp", "-int value=1;", "+int value = 1;"},
		},
		{
			name:    "empty file",
			noColor: true,
			after:   "int value = 1;\n",
			want:    []string{"--- directory/source.cpp", "+++ directory/source.cpp", "+int value = 1;"},
		},
		{name: "unchanged", before: "same\n", after: "same\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := sourceDiff(
				[]byte(tt.before),
				[]byte(tt.after),
				"directory/source.cpp",
				tt.noColor,
			)
			if err != nil {
				t.Fatalf("sourceDiff() error = %v", err)
			}
			got := string(diff)
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("sourceDiff() output does not contain %q: %q", want, got)
				}
			}
			if strings.Contains(got, "\x1b[") != tt.color {
				t.Fatalf("sourceDiff() output color = %q, wantColor = %t", got, tt.color)
			}
			if strings.HasSuffix(got, " \n") {
				t.Fatalf("sourceDiff() output has an artificial empty context line: %q", got)
			}
		})
	}
}

func writeTestFormatFile(t *testing.T, root, format string) {
	t.Helper()
	path := filepath.Join(root, "format", filepath.FromSlash(format))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("BasedOnStyle: LLVM\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func installFakeClangFormat(t *testing.T) (string, string) {
	t.Helper()
	binDirectory := t.TempDir()
	executable := filepath.Join(binDirectory, "clang-format")
	script := "#!/bin/sh\n" +
		"{\n" +
		"\tprintf '%s\\n' CALL\n" +
		"\tprintf '%s\\n' \"$@\"\n" +
		"} >> \"$FORMAT_LOG\"\n" +
		"case \"$3\" in\n" +
		"\t*fail.cpp) exit 7 ;;\n" +
		"esac\n" +
		"if [ -n \"$FORMAT_REPLACEMENT\" ]; then\n" +
		"\tprintf '%s' \"$FORMAT_REPLACEMENT\" > \"$3\"\n" +
		"fi\n"
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake clang-format: %v", err)
	}

	log := filepath.Join(t.TempDir(), "clang-format.log")
	t.Setenv("PATH", binDirectory)
	t.Setenv("FORMAT_LOG", log)
	t.Setenv("FORMAT_REPLACEMENT", "")
	return log, binDirectory
}

func installParallelClangFormat(t *testing.T) string {
	t.Helper()
	binDirectory := t.TempDir()
	executable := filepath.Join(binDirectory, "clang-format")
	script := "#!/bin/sh\n" +
		"printf 'START %s\\n' \"$3\" >> \"$FORMAT_LOG\"\n" +
		"/bin/sleep 0.1\n" +
		"printf 'END %s\\n' \"$3\" >> \"$FORMAT_LOG\"\n"
	if err := os.WriteFile(executable, []byte(script), 0o755); err != nil {
		t.Fatalf("write parallel clang-format: %v", err)
	}

	log := filepath.Join(t.TempDir(), "parallel.log")
	t.Setenv("PATH", binDirectory)
	t.Setenv("FORMAT_LOG", log)
	return log
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
