package main

import (
	"bytes"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want arguments
	}{
		{
			name: "format defaults to current directory",
			args: []string{"format"},
			want: arguments{command: "format", paths: []string{"."}, format: defaultFormat},
		},
		{
			name: "format accepts custom style",
			args: []string{"format", "--format=custom.v1", "src"},
			want: arguments{command: "format", paths: []string{"src"}, format: "custom.v1"},
		},
		{
			name: "format flag between paths",
			args: []string{"format", "src", "--format=team/debug.yaml", "include"},
			want: arguments{
				command: "format",
				paths:   []string{"src", "include"},
				format:  "team/debug.yaml",
			},
		},
		{
			name: "short silent flag",
			args: []string{"format", "-s", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				silent:  true,
				format:  defaultFormat,
			},
		},
		{
			name: "long silent flag with verbose",
			args: []string{"-v", "format", "--silent", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				verbose: true,
				silent:  true,
				format:  defaultFormat,
			},
		},
		{
			name: "build accepts one path",
			args: []string{"build", "src"},
			want: arguments{command: "build", paths: []string{"src"}},
		},
		{
			name: "build accepts silent flag",
			args: []string{"build", "-s", "src"},
			want: arguments{command: "build", paths: []string{"src"}, silent: true},
		},
		{
			name: "build accepts exact output",
			args: []string{"build", "src", "-o", "bin/application"},
			want: arguments{command: "build", paths: []string{"src"}, output: "bin/application"},
		},
		{
			name: "build accepts output directory",
			args: []string{"build", "--output=bin/", "src"},
			want: arguments{command: "build", paths: []string{"src"}, output: "bin/"},
		},
		{
			name: "build accepts no-cache flag",
			args: []string{"build", "--no-cache", "src"},
			want: arguments{command: "build", paths: []string{"src"}, noCache: true},
		},
		{
			name: "run defaults to current directory",
			args: []string{"run"},
			want: arguments{command: "run", paths: []string{"."}},
		},
		{
			name: "run accepts build flags and path",
			args: []string{"-v", "run", "--no-cache", "-s", "src/app.cpp"},
			want: arguments{
				command: "run",
				paths:   []string{"src/app.cpp"},
				verbose: true,
				silent:  true,
				noCache: true,
			},
		},
		{
			name: "run passes arguments after separator unchanged",
			args: []string{
				"run",
				"src/app.cpp",
				"--",
				"--mode",
				"two words",
				"-j",
				"--no-cache",
			},
			want: arguments{
				command:          "run",
				paths:            []string{"src/app.cpp"},
				programArguments: []string{"--mode", "two words", "-j", "--no-cache"},
			},
		},
		{
			name: "run separator defaults path to current directory",
			args: []string{"run", "--", "input.txt"},
			want: arguments{
				command:          "run",
				paths:            []string{"."},
				programArguments: []string{"input.txt"},
			},
		},
		{
			name: "fetch defaults to current directory",
			args: []string{"fetch"},
			want: arguments{command: "fetch", paths: []string{"."}},
		},
		{
			name: "fetch accepts silent flag and jobs",
			args: []string{"fetch", "-s", "-j4", "src", "tests"},
			want: arguments{
				command: "fetch",
				paths:   []string{"src", "tests"},
				silent:  true,
				jobs:    4,
			},
		},
		{
			name: "test accepts multiple paths",
			args: []string{"test", "src", "tests/unit"},
			want: arguments{command: "test", paths: []string{"src", "tests/unit"}},
		},
		{
			name: "test accepts silent flag",
			args: []string{"test", "-s", "tests/unit"},
			want: arguments{command: "test", paths: []string{"tests/unit"}, silent: true},
		},
		{
			name: "test accepts no-cache flag",
			args: []string{"test", "tests/unit", "--no-cache"},
			want: arguments{command: "test", paths: []string{"tests/unit"}, noCache: true},
		},
		{
			name: "test lists selected source tests",
			args: []string{"test", "--list-tests", "tests/unit"},
			want: arguments{command: "test", paths: []string{"tests/unit"}, listTests: true},
		},
		{
			name: "test accepts repeated selectors",
			args: []string{
				"test",
				"--test=Random.Returns*",
				"tests/random.test.cpp",
				"--test",
				"Parser.Test?",
			},
			want: arguments{
				command:       "test",
				paths:         []string{"tests/random.test.cpp"},
				testSelectors: []string{"Random.Returns*", "Parser.Test?"},
			},
		},
		{
			name: "test selector defaults to current directory",
			args: []string{"test", "--test=Random.ReturnsValue"},
			want: arguments{
				command:       "test",
				paths:         []string{"."},
				testSelectors: []string{"Random.ReturnsValue"},
			},
		},
		{
			name: "short verbose flag before command",
			args: []string{"-v", "build", "src"},
			want: arguments{command: "build", paths: []string{"src"}, verbose: true},
		},
		{
			name: "long verbose flag after command",
			args: []string{"format", "--verbose", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				verbose: true,
				format:  defaultFormat,
			},
		},
		{
			name: "verbose flag between paths",
			args: []string{"test", "src", "-v", "tests/unit"},
			want: arguments{command: "test", paths: []string{"src", "tests/unit"}, verbose: true},
		},
		{
			name: "no color flag before command",
			args: []string{"--no-color", "build", "src"},
			want: arguments{command: "build", paths: []string{"src"}, noColor: true},
		},
		{
			name: "no color flag after command",
			args: []string{"format", "--no-color", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				noColor: true,
				format:  defaultFormat,
			},
		},
		{
			name: "no color flag between paths",
			args: []string{"test", "src", "--no-color", "tests/unit"},
			want: arguments{
				command: "test",
				paths:   []string{"src", "tests/unit"},
				noColor: true,
			},
		},
		{
			name: "attached short jobs value",
			args: []string{"format", "-j4", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				jobs:    4,
				format:  defaultFormat,
			},
		},
		{
			name: "jobs before command",
			args: []string{"-j2", "build", "src"},
			want: arguments{command: "build", paths: []string{"src"}, jobs: 2},
		},
		{
			name: "jobs between paths",
			args: []string{"format", "src", "-j2", "include"},
			want: arguments{
				command: "format",
				paths:   []string{"src", "include"},
				jobs:    2,
				format:  defaultFormat,
			},
		},
		{
			name: "long jobs value",
			args: []string{"build", "--jobs=3", "src"},
			want: arguments{command: "build", paths: []string{"src"}, jobs: 3},
		},
		{
			name: "short jobs without value uses all CPUs",
			args: []string{"format", "-j", "src"},
			want: arguments{
				command: "format",
				paths:   []string{"src"},
				jobs:    runtime.NumCPU(),
				format:  defaultFormat,
			},
		},
		{
			name: "long jobs without value uses all CPUs",
			args: []string{"build", "--jobs", "src"},
			want: arguments{command: "build", paths: []string{"src"}, jobs: runtime.NumCPU()},
		},
		{
			name: "zero jobs uses all CPUs",
			args: []string{"test", "-j0", "tests"},
			want: arguments{command: "test", paths: []string{"tests"}, jobs: runtime.NumCPU()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got, err := parseArguments(tt.args, &stdout, &stderr)
			if err != nil {
				t.Fatalf("parseArguments() error = %v", err)
			}
			want := tt.want
			if want.jobs == 0 {
				want.jobs = defaultJobs
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("parseArguments() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseArgumentsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing command", wantErr: "a command is required"},
		{name: "unknown command", args: []string{"deps"}, wantErr: "unknown command"},
		{name: "help command", args: []string{"help"}, wantErr: "unknown command"},
		{name: "internal help command", args: []string{"_help"}, wantErr: "unknown command"},
		{name: "unknown flag", args: []string{"build", "--workers", "4"}, wantErr: "unknown flag"},
		{name: "negative jobs", args: []string{"format", "--jobs=-1"}, wantErr: "jobs must not be negative"},
		{name: "build rejects format flag", args: []string{"build", "--format=custom.v1"}, wantErr: "unknown flag"},
		{name: "fetch rejects format flag", args: []string{"fetch", "--format=custom.v1"}, wantErr: "unknown flag"},
		{name: "test rejects format flag", args: []string{"test", "--format=custom.v1"}, wantErr: "unknown flag"},
		{name: "format rejects output flag", args: []string{"format", "-o", "binary"}, wantErr: "unknown shorthand flag"},
		{name: "fetch rejects output flag", args: []string{"fetch", "--output=binary"}, wantErr: "unknown flag"},
		{name: "test rejects output flag", args: []string{"test", "--output=binary"}, wantErr: "unknown flag"},
		{name: "format rejects no-cache flag", args: []string{"format", "--no-cache"}, wantErr: "unknown flag"},
		{name: "fetch rejects no-cache flag", args: []string{"fetch", "--no-cache"}, wantErr: "unknown flag"},
		{
			name:    "test rejects list with selector",
			args:    []string{"test", "--list-tests", "--test=Suite.Case"},
			wantErr: "--list-tests and --test cannot be used together",
		},
		{
			name:    "test rejects empty selector",
			args:    []string{"test", "--test="},
			wantErr: "test selector must not be empty",
		},
		{
			name:    "test rejects selector separator",
			args:    []string{"test", "--test=Suite.One:Suite.Two"},
			wantErr: "test selector must not contain ':'",
		},
		{
			name:    "test rejects negative filter syntax",
			args:    []string{"test", "--test=Suite.*-Suite.Slow"},
			wantErr: "test selector must not contain '-'",
		},
		{name: "run rejects format flag", args: []string{"run", "--format=custom.v1"}, wantErr: "unknown flag"},
		{name: "run rejects output flag", args: []string{"run", "--output=binary"}, wantErr: "unknown flag"},
		{name: "run rejects list-tests flag", args: []string{"run", "--list-tests"}, wantErr: "unknown flag"},
		{name: "run rejects test selector", args: []string{"run", "--test=Suite.Case"}, wantErr: "unknown flag"},
		{name: "build rejects list-tests flag", args: []string{"build", "--list-tests"}, wantErr: "unknown flag"},
		{name: "build rejects test selector", args: []string{"build", "--test=Suite.Case"}, wantErr: "unknown flag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := parseArguments(tt.args, &stdout, &stderr)
			if err == nil {
				t.Fatal("parseArguments() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseArguments() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestShellCompletion(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		notWant []string
	}{
		{
			name: "public commands only",
			args: []string{"__complete", ""},
			want: []string{
				"build\tBuild C++ sources",
				"fetch\tDownload C++ dependencies",
				"format\tFormat C++ sources",
				"run\tBuild and run a C++ program",
				"test\tBuild and run C++ tests",
				":4",
			},
			notWant: []string{"_help", "completion"},
		},
		{
			name: "public commands without descriptions",
			args: []string{"__completeNoDesc", ""},
			want: []string{
				"build\n",
				"fetch\n",
				"format\n",
				"run\n",
				"test\n",
				":4",
			},
			notWant: []string{"_help", "completion"},
		},
		{
			name: "default format",
			args: []string{"__complete", "format", "--format="},
			want: []string{"format.v1", ":4"},
		},
		{
			name: "path completion",
			args: []string{"__complete", "build", ""},
			want: []string{":0"},
		},
		{
			name: "bare jobs remains a partial flag",
			args: []string{"__complete", "-j"},
			want: []string{"-j\tnumber of parallel jobs", ":4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			parsed, err := parseArguments(tt.args, &stdout, &stderr)
			if err != nil {
				t.Fatalf("parseArguments() error = %v, stderr = %q", err, stderr.String())
			}
			if !reflect.DeepEqual(parsed, arguments{}) {
				t.Fatalf("parseArguments() = %#v, want no executable command", parsed)
			}
			output := stdout.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("completion does not contain %q:\n%s", want, output)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(output, notWant) {
					t.Errorf("completion contains internal value %q:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestBackendShellCompletionDoesNotOwnTargetValues(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_, err := parseArguments([]string{"__complete", "--target=linux64:v"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("parseArguments() error = %v, stderr = %q", err, stderr.String())
	}
	for _, target := range []string{
		"linux64:v3.0-ubuntu.22.04",
		"linux64:v3.0-alpine.3.22-static",
	} {
		if strings.Contains(stdout.String(), target) {
			t.Errorf("backend completion contains wrapper target %q:\n%s", target, stdout.String())
		}
	}
}

func TestCompletionScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  []string
	}{
		{shell: "bash", want: []string{"# bash completion V2 for hard", "__start_hard()"}},
		{shell: "zsh", want: []string{"#compdef hard", "compdef _hard hard"}},
		{shell: "fish", want: []string{"# fish completion for hard", "complete -c hard"}},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			parsed, err := parseArguments([]string{"completion", tt.shell}, &stdout, &stderr)
			if err != nil {
				t.Fatalf("parseArguments() error = %v, stderr = %q", err, stderr.String())
			}
			if !reflect.DeepEqual(parsed, arguments{}) {
				t.Fatalf("parseArguments() = %#v, want no executable command", parsed)
			}
			output := stdout.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("%s completion does not contain %q", tt.shell, want)
				}
			}
			if strings.Contains(output, "_help") {
				t.Errorf("%s completion contains internal help command", tt.shell)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: []string{
				"build       Build C++ sources",
				"fetch       Download C++ dependencies",
				"format      Format C++ sources",
				"run         Build and run a C++ program",
				"test        Build and run C++ tests",
				"-j, --jobs",
				"(default 1)",
				"--no-color",
				"--target string",
				"supported: host, linux64, linux64:vX.Y-ubuntu.YY.MM, linux64:vX.Y-alpine.A.B-static",
				"-v, --verbose",
			},
		},
		{
			name: "format",
			args: []string{"format", "--help"},
			want: []string{
				"hard format [path...]",
				"--format string",
				"format style file installed with hard",
				"(default \"format.v1\")",
				"-j, --jobs",
				"--no-color",
				"-s, --silent",
				"-v, --verbose",
			},
		},
		{
			name: "build",
			args: []string{"build", "--help"},
			want: []string{
				"hard build [path...]",
				"-j, --jobs",
				"--no-cache",
				"--no-color",
				"-o, --output",
				"-s, --silent",
				"-v, --verbose",
			},
		},
		{
			name: "fetch",
			args: []string{"fetch", "--help"},
			want: []string{"hard fetch [path...]", "-j, --jobs", "--no-color", "-s, --silent", "-v, --verbose"},
		},
		{
			name: "run",
			args: []string{"run", "--help"},
			want: []string{
				"hard run [path...] [-- program-argument...]",
				"-j, --jobs",
				"--no-cache",
				"--no-color",
				"-s, --silent",
				"-v, --verbose",
			},
		},
		{
			name: "test",
			args: []string{"test", "--help"},
			want: []string{
				"hard test [--list-tests] [--test=<selector>]... [path...]",
				"-j, --jobs",
				"--list-tests",
				"--no-cache",
				"--no-color",
				"-s, --silent",
				"--test stringArray",
				"-v, --verbose",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := parseArguments(tt.args, &stdout, &stderr)
			if err != nil {
				t.Fatalf("parseArguments() error = %v", err)
			}

			help := stdout.String()
			for _, want := range tt.want {
				if !strings.Contains(help, want) {
					t.Errorf("help does not contain %q:\n%s", want, help)
				}
			}
			for _, command := range []string{"deps", "completion", "help"} {
				if strings.Contains(help, "\n  "+command+" ") {
					t.Errorf("help contains undocumented command %s:\n%s", command, help)
				}
			}
			if tt.name != "format" && strings.Contains(help, "--format") {
				t.Errorf("help contains format-only flag:\n%s", help)
			}
			if tt.name == "root" && strings.Contains(help, "--silent") {
				t.Errorf("help contains unsupported silent flag:\n%s", help)
			}
			if tt.name != "build" && strings.Contains(help, "--output") {
				t.Errorf("help contains build-only output flag:\n%s", help)
			}
			if tt.name != "build" && tt.name != "run" && tt.name != "test" && strings.Contains(help, "--no-cache") {
				t.Errorf("help contains build/run/test-only no-cache flag:\n%s", help)
			}
			if tt.name != "test" && (strings.Contains(help, "--list-tests") || strings.Contains(help, "--test")) {
				t.Errorf("help contains test-only selection flag:\n%s", help)
			}
		})
	}
}
