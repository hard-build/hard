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

func TestWrapperRunsHostBackendFromPortableLayoutWithoutTarget(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(t.TempDir(), "portable hard")
	wrapper := installWrapperAtPrefix(t, prefix)
	runtimeRoot := filepath.Join(prefix, "libexec", "hard")
	backend := filepath.Join(runtimeRoot, "hard")
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

	command := exec.Command(wrapper, "run", "--", "--target", "linux64")
	command.Env = wrapperTestEnvironment(map[string]string{
		"BACKEND_LOG": backendLog,
		"HOME":        home,
		"PATH":        binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host wrapper error = %v, output = %q", err, output)
	}

	want := []string{"run", "--", "--target", "linux64"}
	if got := readWrapperArguments(t, backendLog); !reflect.DeepEqual(got, want) {
		t.Fatalf("host backend arguments = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("docker log error = %v, want not exist", err)
	}
}

func TestWrapperRunsExplicitHostTargetWithBundledTools(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(home, ".local")
	wrapper := installWrapperAtPrefix(t, prefix)
	runtimeRoot := filepath.Join(prefix, "libexec", "hard")
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

	command := exec.Command(wrapper, "build", "--target=host", "source.cpp")
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

func TestWrapperRunsHostBackendThroughPath(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(t.TempDir(), "installed hard")
	installWrapperAtPrefix(t, prefix)
	runtimeRoot := filepath.Join(prefix, "libexec", "hard")
	backend := filepath.Join(runtimeRoot, "hard")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	writeWrapperExecutable(
		t,
		backend,
		"#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$BACKEND_LOG\"\n",
	)
	backendLog := filepath.Join(t.TempDir(), "backend.log")
	path := filepath.Join(prefix, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	command := exec.Command("env", "hard", "--target=host", "build", "source.cpp")
	command.Env = wrapperTestEnvironment(map[string]string{
		"BACKEND_LOG": backendLog,
		"HOME":        home,
		"PATH":        path,
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host wrapper error = %v, output = %q", err, output)
	}
	want := []string{"build", "source.cpp"}
	if got := readWrapperArguments(t, backendLog); !reflect.DeepEqual(got, want) {
		t.Fatalf("host backend arguments = %#v, want %#v", got, want)
	}
}

func TestWrapperRunsCompletionThroughHostBackend(t *testing.T) {
	for _, request := range []string{"__complete", "__completeNoDesc"} {
		for _, arguments := range [][]string{{request, "bu"}, {request, "run", "--", "--target="}} {
			arguments := arguments
			t.Run(request+"/"+strings.Join(arguments[1:], "_"), func(t *testing.T) {
				home := t.TempDir()
				prefix := filepath.Join(t.TempDir(), "installed hard")
				wrapper := installWrapperAtPrefix(t, prefix)
				runtimeRoot := filepath.Join(prefix, "libexec", "hard")
				backend := filepath.Join(runtimeRoot, "hard")
				if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
					t.Fatalf("create runtime directory: %v", err)
				}
				writeWrapperExecutable(
					t,
					backend,
					"#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$BACKEND_LOG\"\n",
				)
				if err := os.WriteFile(
					filepath.Join(runtimeRoot, "default-target"),
					[]byte("linux64\n"),
					0o644,
				); err != nil {
					t.Fatalf("write default target: %v", err)
				}
				backendLog := filepath.Join(t.TempDir(), "backend.log")
				dockerLog := filepath.Join(t.TempDir(), "docker.log")
				binDirectory := installFakeWrapperDocker(t, dockerLog)

				command := exec.Command(wrapper, arguments...)
				command.Env = wrapperTestEnvironment(map[string]string{
					"BACKEND_LOG": backendLog,
					"HOME":        home,
					"PATH":        binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
				})
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("completion wrapper error = %v, output = %q", err, output)
				}
				if got := readWrapperArguments(t, backendLog); !reflect.DeepEqual(got, arguments) {
					t.Fatalf("host backend arguments = %#v, want %#v", got, arguments)
				}
				if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("docker log error = %v, want not exist", err)
				}
			})
		}
	}
}

func TestWrapperCompletesTargetsWithoutBackendOrDocker(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "attached value",
			args: []string{"__complete", "--target="},
			want: []string{
				"host",
				"linux64",
				"linux64:v3.0-ubuntu.22.04",
				"linux64:v3.0-alpine.3.22-static",
				":4",
			},
		},
		{
			name: "separate prefixed value without descriptions",
			args: []string{"__completeNoDesc", "--target", "linux64:v3.0-a"},
			want: []string{"linux64:v3.0-alpine.3.22-static", ":4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := filepath.Join(t.TempDir(), "installed hard")
			wrapper := installWrapperAtPrefix(t, prefix)
			runtimeRoot := filepath.Join(prefix, "libexec", "hard")
			backend := filepath.Join(runtimeRoot, "hard")
			if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
				t.Fatalf("create runtime directory: %v", err)
			}
			writeWrapperExecutable(
				t,
				backend,
				"#!/bin/sh\nprintf '%s\\0' \"$@\" > \"$BACKEND_LOG\"\n",
			)
			if err := os.WriteFile(
				filepath.Join(runtimeRoot, "default-target"),
				[]byte("linux64\n"),
				0o644,
			); err != nil {
				t.Fatalf("write default target: %v", err)
			}
			backendLog := filepath.Join(t.TempDir(), "backend.log")
			dockerLog := filepath.Join(t.TempDir(), "docker.log")
			binDirectory := installFakeWrapperDocker(t, dockerLog)

			command := exec.Command(wrapper, tt.args...)
			command.Env = wrapperTestEnvironment(map[string]string{
				"BACKEND_LOG": backendLog,
				"HOME":        t.TempDir(),
				"PATH":        binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
			})
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("completion wrapper error = %v, output = %q", err, output)
			}
			got := strings.Split(strings.TrimSpace(string(output)), "\n")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("completion output = %#v, want %#v", got, tt.want)
			}
			if _, err := os.Stat(backendLog); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("backend log error = %v, want not exist", err)
			}
			if _, err := os.Stat(dockerLog); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("docker log error = %v, want not exist", err)
			}
		})
	}
}

func TestWrapperUsesInstalledLinuxDefaultTarget(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(home, ".local")
	wrapper := installWrapperAtPrefix(t, prefix)
	runtimeRoot := filepath.Join(prefix, "libexec", "hard")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatalf("create runtime directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runtimeRoot, "default-target"), []byte("linux64\n"), 0o644); err != nil {
		t.Fatalf("write default target: %v", err)
	}
	hardRoot := filepath.Join(home, ".local", "share", "hard")
	if err := os.MkdirAll(hardRoot, 0o755); err != nil {
		t.Fatalf("create HARD_ROOT: %v", err)
	}
	project := t.TempDir()
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	binDirectory := installFakeWrapperDocker(t, dockerLog)

	command := exec.Command(wrapper, "build", "source.cpp")
	command.Dir = project
	command.Env = wrapperTestEnvironment(map[string]string{
		"HOME": home,
		"PATH": binDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("target wrapper error = %v, output = %q", err, output)
	}

	arguments := readWrapperArguments(t, dockerLog)
	if !containsWrapperArgument(arguments, "ghcr.io/hard-build/linux64:latest") {
		t.Fatalf("docker arguments = %#v, want linux64 latest image", arguments)
	}
	if !containsWrapperArgument(arguments, "--pull=always") {
		t.Fatalf("docker arguments = %#v, want latest pull policy", arguments)
	}
	if got := arguments[len(arguments)-2:]; !reflect.DeepEqual(got, []string{"build", "source.cpp"}) {
		t.Fatalf("backend arguments = %#v, want build arguments", got)
	}
}

func TestWrapperRunsLinuxTargetInDocker(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		image string
		pull  string
		want  []string
	}{
		{
			name:  "separate target value",
			args:  []string{"--target", "linux64", "build", "source file.cpp"},
			image: "ghcr.io/hard-build/linux64:latest",
			pull:  "always",
			want:  []string{"build", "source file.cpp"},
		},
		{
			name:  "versioned target between backend arguments",
			args:  []string{"build", "first.cpp", "--target=linux64:v2.0-ubuntu.22.04", "second.cpp", "--", "--target", "program-value"},
			image: "ghcr.io/hard-build/linux64:v2.0-ubuntu.22.04",
			pull:  "missing",
			want:  []string{"build", "first.cpp", "second.cpp", "--", "--target", "program-value"},
		},
		{
			name:  "future versioned target",
			args:  []string{"--target=linux64:v12.34-ubuntu.24.04", "build"},
			image: "ghcr.io/hard-build/linux64:v12.34-ubuntu.24.04",
			pull:  "missing",
			want:  []string{"build"},
		},
		{
			name:  "Alpine static target",
			args:  []string{"build", "--target=linux64:v2.0-alpine.3.22-static"},
			image: "ghcr.io/hard-build/linux64:v2.0-alpine.3.22-static",
			pull:  "missing",
			want:  []string{"build"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := filepath.Join(t.TempDir(), "wrapper only")
			wrapper := installWrapperAtPrefix(t, prefix)
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

			command := exec.Command(wrapper, tt.args...)
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
				"--pull=" + tt.pull,
				"--user",
				strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
				"--mount",
				"type=bind,source=" + hardRoot + ",target=/hard",
				"--mount",
				"type=bind,source=" + project + ",target=" + project,
				"--workdir",
				project,
				tt.image,
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

	command := exec.Command(wrapperPath(t), "--target=linux64", "build")
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
			args:    []string{"--target=linux64", "build", "--target", "linux64:v2.0-ubuntu.22.04"},
			wantErr: "--target may only be specified once",
		},
		{name: "legacy target", args: []string{"--target=linux.v1", "build"}, wantErr: "unknown target: linux.v1"},
		{name: "missing image version", args: []string{"--target=linux64:v2.0", "build"}, wantErr: "unknown target: linux64:v2.0"},
		{name: "invalid hard version", args: []string{"--target=linux64:v2.x-ubuntu.22.04", "build"}, wantErr: "unknown target: linux64:v2.x-ubuntu.22.04"},
		{name: "invalid Ubuntu version", args: []string{"--target=linux64:v2.0-ubuntu.22", "build"}, wantErr: "unknown target: linux64:v2.0-ubuntu.22"},
		{
			name:    "Alpine target without static suffix",
			args:    []string{"--target=linux64:v2.0-alpine.3.22", "build"},
			wantErr: "unknown target: linux64:v2.0-alpine.3.22",
		},
		{
			name:    "invalid Alpine version",
			args:    []string{"--target=linux64:v2.0-alpine.3-static", "build"},
			wantErr: "unknown target: linux64:v2.0-alpine.3-static",
		},
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

func installWrapperAtPrefix(t *testing.T, prefix string) string {
	t.Helper()
	wrapper := filepath.Join(prefix, "bin", "hard")
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatalf("create wrapper directory: %v", err)
	}
	contents, err := os.ReadFile(wrapperPath(t))
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}
	writeWrapperExecutable(t, wrapper, string(contents))
	return wrapper
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
