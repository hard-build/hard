package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestWrapperRunsHostBackendWithoutTarget(t *testing.T) {
	home := t.TempDir()
	backend := filepath.Join(home, ".local", "libexec", "hard", "hard")
	if err := os.MkdirAll(filepath.Dir(backend), 0o755); err != nil {
		t.Fatalf("create backend directory: %v", err)
	}
	writeWrapperExecutable(
		t,
		backend,
		"#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$BACKEND_LOG\"\n",
	)
	backendLog := filepath.Join(t.TempDir(), "backend.log")
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	binDirectory := installFakeWrapperDocker(t, dockerLog)

	command := exec.Command(wrapperPath(t), "run", "--", "--target", "linux.v1")
	command.Env = wrapperTestEnvironment(map[string]string{
		"BACKEND_LOG": backendLog,
		"HOME":        home,
		"PATH":        binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host wrapper error = %v, output = %q", err, output)
	}

	want := []string{"run", "--", "--target", "linux.v1"}
	if got := readWrapperArguments(t, backendLog); !reflect.DeepEqual(got, want) {
		t.Fatalf("host backend arguments = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("docker log error = %v, want not exist", err)
	}
}

func TestWrapperRunsExplicitHostTargetWithBundledTools(t *testing.T) {
	home := t.TempDir()
	runtimeRoot := filepath.Join(home, ".local", "libexec", "hard")
	backend := filepath.Join(runtimeRoot, "hard")
	if err := os.MkdirAll(filepath.Join(runtimeRoot, "bin"), 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	writeWrapperExecutable(
		t,
		backend,
		"#!/bin/sh\nprintf '%s\\0' \"$PATH\" \"$@\" > \"$BACKEND_LOG\"\n",
	)
	backendLog := filepath.Join(t.TempDir(), "backend.log")
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	binDirectory := installFakeWrapperDocker(t, dockerLog)
	originalPath := binDirectory + string(os.PathListSeparator) + os.Getenv("PATH")

	command := exec.Command(wrapperPath(t), "build", "--target=host", "source.cpp")
	command.Env = wrapperTestEnvironment(map[string]string{
		"BACKEND_LOG": backendLog,
		"HOME":        home,
		"PATH":        originalPath,
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host wrapper error = %v, output = %q", err, output)
	}

	want := []string{
		filepath.Join(runtimeRoot, "bin") + string(os.PathListSeparator) + originalPath,
		"build",
		"source.cpp",
	}
	if got := readWrapperArguments(t, backendLog); !reflect.DeepEqual(got, want) {
		t.Fatalf("host backend arguments = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("docker log error = %v, want not exist", err)
	}
}

func TestWrapperUsesInstalledLinuxDefaultTarget(t *testing.T) {
	home := t.TempDir()
	runtimeRoot := filepath.Join(home, ".local", "libexec", "hard")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "default-target"), []byte("linux.v1\n"), 0o644); err != nil {
		t.Fatalf("write default target: %v", err)
	}
	hardRoot := filepath.Join(home, ".local", "share", "hard")
	if err := os.MkdirAll(hardRoot, 0o755); err != nil {
		t.Fatalf("create HARD_ROOT: %v", err)
	}
	project := t.TempDir()
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	binDirectory := installFakeWrapperDocker(t, dockerLog)

	command := exec.Command(wrapperPath(t), "build", "source.cpp")
	command.Dir = project
	command.Env = wrapperTestEnvironment(map[string]string{
		"HOME": home,
		"PATH": binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("target wrapper error = %v, output = %q", err, output)
	}

	arguments := readWrapperArguments(t, dockerLog)
	if !containsWrapperArgument(arguments, "ghcr.io/hard-build/hard:linux.v1") {
		t.Fatalf("docker arguments = %#v, want linux.v1 image", arguments)
	}
	if got := arguments[len(arguments)-2:]; !reflect.DeepEqual(got, []string{"build", "source.cpp"}) {
		t.Fatalf("backend arguments = %#v, want build arguments", got)
	}
}

func TestWrapperRunsLinuxTargetInDocker(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "separate target value",
			args: []string{"--target", "linux.v1", "build", "source file.cpp"},
			want: []string{"build", "source file.cpp"},
		},
		{
			name: "target between backend arguments",
			args: []string{"build", "first.cpp", "--target=linux.v1", "second.cpp", "--", "--target", "program-value"},
			want: []string{"build", "first.cpp", "second.cpp", "--", "--target", "program-value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			hardRoot := filepath.Join(t.TempDir(), "hard data")
			if err := os.MkdirAll(hardRoot, 0o755); err != nil {
				t.Fatalf("create HARD_ROOT: %v", err)
			}
			project := filepath.Join(t.TempDir(), "project directory")
			if err := os.MkdirAll(project, 0o755); err != nil {
				t.Fatalf("create project: %v", err)
			}
			dockerLog := filepath.Join(t.TempDir(), "docker.log")
			binDirectory := installFakeWrapperDocker(t, dockerLog)

			command := exec.Command(wrapperPath(t), tt.args...)
			command.Dir = project
			command.Env = wrapperTestEnvironment(map[string]string{
				"HARD_CC":          "host-compiler-must-not-be-forwarded",
				"HARD_CFLAGS":      "host-cflags-must-not-be-forwarded",
				"HARD_ENTRYPOINTS": "host-entrypoints-must-not-be-forwarded",
				"HARD_ENV":         "host-environment-must-not-be-forwarded",
				"HARD_LDFLAGS":     "host-ldflags-must-not-be-forwarded",
				"HARD_ROOT":        hardRoot,
				"HOME":             home,
				"PATH":             binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
			})
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("target wrapper error = %v, output = %q", err, output)
			}

			want := []string{
				"run",
				"--rm",
				"--interactive",
				"--pull=missing",
				"--user",
				strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
				"--mount",
				"type=bind,source=" + hardRoot + ",target=/hard",
				"--mount",
				"type=bind,source=" + project + ",target=" + project,
				"--workdir",
				project,
				"ghcr.io/hard-build/hard:linux.v1",
			}
			want = append(want, tt.want...)
			if got := readWrapperArguments(t, dockerLog); !reflect.DeepEqual(got, want) {
				t.Fatalf("docker arguments = %#v, want %#v", got, want)
			}
		})
	}
}

func TestWrapperUsesDefaultHardRoot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	binDirectory := installFakeWrapperDocker(t, dockerLog)

	command := exec.Command(wrapperPath(t), "--target=linux.v1", "build")
	command.Dir = project
	command.Env = wrapperTestEnvironment(map[string]string{
		"HARD_ROOT": "",
		"HOME":      home,
		"PATH":      binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("target wrapper error = %v, output = %q", err, output)
	}

	wantMount := "type=bind,source=" + filepath.Join(home, ".local", "share", "hard") + ",target=/hard"
	if got := readWrapperArguments(t, dockerLog); !containsWrapperArgument(got, wantMount) {
		t.Fatalf("docker arguments = %#v, want %q", got, wantMount)
	}
}

func TestWrapperRejectsInvalidTarget(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing value", args: []string{"build", "--target"}, wantErr: "--target requires a value"},
		{name: "empty value", args: []string{"--target=", "build"}, wantErr: "--target requires a value"},
		{
			name:    "duplicate target",
			args:    []string{"--target=linux.v1", "build", "--target", "linux.v1"},
			wantErr: "--target may only be specified once",
		},
		{name: "unknown target", args: []string{"--target=linux.v2", "build"}, wantErr: "unknown target: linux.v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerLog := filepath.Join(t.TempDir(), "docker.log")
			binDirectory := installFakeWrapperDocker(t, dockerLog)
			command := exec.Command(wrapperPath(t), tt.args...)
			command.Env = wrapperTestEnvironment(map[string]string{
				"HOME": t.TempDir(),
				"PATH": binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
			})
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Run(); err == nil {
				t.Fatal("wrapper error = nil")
			}
			if !strings.Contains(stderr.String(), "hard: "+tt.wantErr) {
				t.Fatalf("wrapper stderr = %q, want %q", stderr.String(), tt.wantErr)
			}
			if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("docker log error = %v, want not exist", err)
			}
		})
	}
}

func wrapperPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "hard.sh"))
	if err != nil {
		t.Fatalf("resolve wrapper path: %v", err)
	}
	return path
}

func installFakeWrapperDocker(t *testing.T, log string) string {
	t.Helper()
	directory := t.TempDir()
	executable := filepath.Join(directory, "docker")
	writeWrapperExecutable(
		t,
		executable,
		"#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$DOCKER_LOG\"\n",
	)
	t.Setenv("DOCKER_LOG", log)
	return directory
}

func writeWrapperExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func readWrapperArguments(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read argument log %s: %v", path, err)
	}
	parts := bytes.Split(contents, []byte{0})
	parts = parts[:len(parts)-1]
	arguments := make([]string, len(parts))
	for index, part := range parts {
		arguments[index] = string(part)
	}
	return arguments
}

func containsWrapperArgument(arguments []string, wanted string) bool {
	for _, argument := range arguments {
		if argument == wanted {
			return true
		}
	}
	return false
}

func wrapperTestEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[name]; !overridden {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}
