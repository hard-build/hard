package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAppendExecutableSuffix(t *testing.T) {
	tests := []struct {
		path   string
		suffix string
		want   string
	}{
		{path: "program", want: "program"},
		{path: "program", suffix: ".exe", want: "program.exe"},
		{path: "program.exe", suffix: ".exe", want: "program.exe"},
		{path: "program", suffix: ".wasm", want: "program.wasm"},
	}

	for _, tt := range tests {
		if got := appendExecutableSuffix(tt.path, tt.suffix); got != tt.want {
			t.Errorf("appendExecutableSuffix(%q, %q) = %q, want %q", tt.path, tt.suffix, got, tt.want)
		}
	}
}

func TestExecutableSuffixControlsInferredBinaryPaths(t *testing.T) {
	root := t.TempDir()
	workingDirectory := t.TempDir()
	source := filepath.Join(workingDirectory, "app.cpp")
	if err := os.WriteFile(source, []byte("int main() {}\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	artifact, err := binaryArtifactPathWithSuffix(root, "cross", source, ".exe")
	if err != nil {
		t.Fatalf("binaryArtifactPathWithSuffix() error = %v", err)
	}
	if !strings.HasSuffix(artifact, filepath.Join("app.exe")) {
		t.Fatalf("binaryArtifactPathWithSuffix() = %q, want app.exe suffix", artifact)
	}

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "default",
			want: filepath.Join(workingDirectory, "app.exe"),
		},
		{
			name:   "directory",
			output: filepath.Join(workingDirectory, "output") + string(filepath.Separator),
			want:   filepath.Join(workingDirectory, "output", "app.exe"),
		},
		{
			name:   "explicit file",
			output: filepath.Join(workingDirectory, "custom-name"),
			want:   filepath.Join(workingDirectory, "custom-name"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := planLinkJobsWithLibrariesExecutable(
				root,
				"cross",
				".exe",
				[]string{source},
				[][]string{{}},
				nil,
				[]string{"main"},
				1,
				tt.output,
				workingDirectory,
			)
			if err != nil {
				t.Fatalf("planLinkJobsWithLibrariesExecutable() error = %v", err)
			}
			if len(tasks) != 1 {
				t.Fatalf("task count = %d, want 1", len(tasks))
			}
			if got := tasks[0].destination; got != tt.want {
				t.Fatalf("destination = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHardEnvironmentDoesNotControlExecutableSuffix(t *testing.T) {
	artifact, err := binaryArtifactPath(t.TempDir(), "windows64:anything", filepath.Join(t.TempDir(), "app.cpp"))
	if err != nil {
		t.Fatalf("binaryArtifactPath() error = %v", err)
	}
	if strings.HasSuffix(artifact, ".exe") {
		t.Fatalf("binaryArtifactPath() = %q, want no implicit suffix", artifact)
	}
}

func TestRunProgramUsesConfiguredRunner(t *testing.T) {
	directory := t.TempDir()
	log := filepath.Join(directory, "runner.log")
	runner := filepath.Join(directory, "runner")
	if err := os.WriteFile(
		runner,
		[]byte("#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$RUNNER_LOG\"\nprintf output\nprintf diagnostics >&2\n"),
		0o755,
	); err != nil {
		t.Fatalf("write fake runner: %v", err)
	}
	t.Setenv("RUNNER_LOG", log)

	binary := filepath.Join(directory, "program.exe")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runProgramWithRunner(
		runner,
		binary,
		[]string{"plain", "two words"},
		directory,
		nil,
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("runProgramWithRunner() error = %v", err)
	}
	if got, want := stdout.String(), "output"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "diagnostics"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got, want := readNullSeparatedArguments(t, log), []string{binary, "plain", "two words"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("runner arguments = %#v, want %#v", got, want)
	}
	if got, want := string(renderRunCommandWithRunner(
		runner,
		binary,
		[]string{"two words"},
	)), runner+" "+binary+" 'two words'\n"; got != want {
		t.Fatalf("renderRunCommandWithRunner() = %q, want %q", got, want)
	}
}

func TestTestProgramUsesConfiguredRunner(t *testing.T) {
	directory := t.TempDir()
	runner := filepath.Join(directory, "runner")
	if err := os.WriteFile(runner, []byte("#!/bin/sh\nprintf 'test-output\\n'\n"), 0o755); err != nil {
		t.Fatalf("write fake runner: %v", err)
	}

	job := testRunJob{
		source: "example.test.cpp",
		binary: filepath.Join(directory, "example.test.exe"),
		runner: runner,
	}
	cached, output, err := runTestWithCache(
		nil,
		job,
		[]string{"--gtest_color=no"},
		directory,
	)
	if err != nil {
		t.Fatalf("runTestWithCache() error = %v", err)
	}
	if cached {
		t.Fatal("runTestWithCache() cached = true")
	}
	if got, want := string(output), "test-output\n"; got != want {
		t.Fatalf("runTestWithCache() output = %q, want %q", got, want)
	}
	if got, want := string(renderTestCommandWithRunner(
		runner,
		job.binary,
		[]string{"--gtest_color=no"},
	)), runner+" "+job.binary+" --gtest_color=no\n"; got != want {
		t.Fatalf("renderTestCommandWithRunner() = %q, want %q", got, want)
	}
}

func readNullSeparatedArguments(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argument log: %v", err)
	}
	parts := bytes.Split(contents, []byte{0})
	if len(parts) != 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	arguments := make([]string, len(parts))
	for index, part := range parts {
		arguments[index] = string(part)
	}
	return arguments
}
