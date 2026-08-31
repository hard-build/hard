package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteEnvironmentReport(t *testing.T) {
	runtimeRoot := t.TempDir()
	resourceDirectory := filepath.Join(runtimeRoot, "lib", "clang", "22", "include")
	if err := os.MkdirAll(resourceDirectory, 0o755); err != nil {
		t.Fatalf("create resource directory: %v", err)
	}
	compiler := filepath.Join(t.TempDir(), "fake-c++")
	if err := os.WriteFile(
		compiler,
		[]byte("#!/bin/sh\ncase \"$1\" in\n--version) printf 'fake compiler 1.2\\nmore\\n' ;;\n-dumpmachine) printf 'x86_64-example\\n' ;;\nesac\n"),
		0o755,
	); err != nil {
		t.Fatalf("write compiler: %v", err)
	}
	config := configuration{
		root:             "/hard data",
		runtimeRoot:      runtimeRoot,
		env:              "cross-cache",
		cc:               compiler,
		ldflags:          []string{"-static", "-Wl,argument with space"},
		entrypoints:      []string{"main", "custom entry"},
		executableSuffix: ".exe",
		executableRunner: "wine",
	}
	var output bytes.Buffer
	if err := writeEnvironmentReport(config, []string{"-std=c++20", "-I/hard data/source"}, true, &output); err != nil {
		t.Fatalf("writeEnvironmentReport() error = %v", err)
	}

	for _, wanted := range []string{
		"HARD BUILD ENVIRONMENT\n",
		"\nRUNTIME\n",
		"  Version             " + hardVersion(),
		"  Runtime root        " + runtimeRoot,
		"  Environment         cross-cache",
		"  Data root           /hard data",
		"\nSYSTEM\n",
		"  Logical CPUs        ",
		"\nCOMPILER\n",
		"  Command             " + compiler,
		"  Executable          " + compiler,
		"  Version             fake compiler 1.2",
		"  Target              x86_64-example",
		"  Binary suffix       .exe",
		"  Runner              wine",
		"\nBUILD CONFIGURATION\n",
		"  CFLAGS\n    -std=c++20\n    '-I/hard data/source'",
		"  LDFLAGS\n    -static\n    '-Wl,argument with space'",
		"  Entry points\n    main\n    'custom entry'",
		"\nPARSER\n",
		"  libclang            ",
		"  Resource directory  " + resourceDirectory,
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Errorf("environment report does not contain %q:\n%s", wanted, output.String())
		}
	}
}

func TestWriteEnvironmentReportContinuesAfterUnavailableCompiler(t *testing.T) {
	config := configuration{
		root:        t.TempDir(),
		runtimeRoot: t.TempDir(),
		env:         "host",
		cc:          filepath.Join(t.TempDir(), "missing-c++"),
	}
	var output bytes.Buffer
	if err := writeEnvironmentReport(config, nil, true, &output); err != nil {
		t.Fatalf("writeEnvironmentReport() error = %v", err)
	}
	for _, wanted := range []string{
		"  Executable          unavailable",
		"  Version             unavailable",
		"  Target              unavailable",
		"  Binary suffix       none",
		"  Runner              direct",
		"  CFLAGS\n    none",
		"  Resource directory  system default",
	} {
		if !strings.Contains(output.String(), wanted) {
			t.Errorf("environment report does not contain %q:\n%s", wanted, output.String())
		}
	}
}

func TestRenderEnvironmentReport(t *testing.T) {
	report := environmentReport{
		version:            "v4.0-development",
		executable:         "/runtime/hard",
		runtimeRoot:        "/runtime",
		environment:        "host",
		root:               "/hard",
		operatingSystem:    "Example Linux 1.0",
		kernel:             "Linux 1.0.0",
		architecture:       "x86_64",
		cpu:                "Example CPU",
		logicalCPUs:        8,
		libc:               "glibc 2.39",
		compilerCommand:    "c++",
		compilerExecutable: "/usr/bin/c++",
		compilerVersion:    "Example C++ 1.0",
		compilerTarget:     "x86_64-linux-gnu",
		executableSuffix:   "none",
		executableRunner:   "direct",
		cflags:             []string{"-std=c++20", "-DNAME=hello world"},
		ldflags:            []string{"-static"},
		libclang:           "Clang 18.1.8",
		resourceDirectory:  "system default",
	}
	want := strings.Join([]string{
		"HARD BUILD ENVIRONMENT",
		environmentRule,
		"",
		"RUNTIME",
		"  Version             v4.0-development",
		"  Executable          /runtime/hard",
		"  Runtime root        /runtime",
		"  Environment         host",
		"  Data root           /hard",
		"",
		"SYSTEM",
		"  Operating system    Example Linux 1.0",
		"  Kernel              Linux 1.0.0",
		"  Architecture        x86_64",
		"  CPU                 Example CPU",
		"  Logical CPUs        8",
		"  libc                glibc 2.39",
		"",
		"COMPILER",
		"  Command             c++",
		"  Executable          /usr/bin/c++",
		"  Version             Example C++ 1.0",
		"  Target              x86_64-linux-gnu",
		"  Binary suffix       none",
		"  Runner              direct",
		"",
		"BUILD CONFIGURATION",
		"  CFLAGS",
		"    -std=c++20",
		"    '-DNAME=hello world'",
		"",
		"  LDFLAGS",
		"    -static",
		"",
		"  Entry points",
		"    none",
		"",
		"PARSER",
		"  libclang            Clang 18.1.8",
		"  Resource directory  system default",
		"",
	}, "\n")
	if got := renderEnvironmentReport(report, true); got != want {
		t.Fatalf("renderEnvironmentReport() =\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderEnvironmentReportUsesColor(t *testing.T) {
	report := environmentReport{
		executableSuffix:  "none",
		executableRunner:  "direct",
		resourceDirectory: unavailableDiagnostic,
	}
	colored := renderEnvironmentReport(report, false)
	for _, sequence := range []string{
		environmentBold + environmentCyan,
		environmentDim + environmentCyan,
		environmentBold + environmentGreen,
		environmentCyan,
		environmentYellow,
		environmentReset,
	} {
		if !strings.Contains(colored, sequence) {
			t.Errorf("colored report does not contain %q: %q", sequence, colored)
		}
	}

	plain := renderEnvironmentReport(report, true)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain report contains ANSI escapes: %q", plain)
	}
	replacer := strings.NewReplacer(
		environmentReset, "",
		environmentBold, "",
		environmentDim, "",
		environmentGreen, "",
		environmentYellow, "",
		environmentCyan, "",
	)
	if got := replacer.Replace(colored); got != plain {
		t.Fatalf("colored report without ANSI = %q, want %q", got, plain)
	}
}

func TestReadOSReleaseName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(path, []byte("NAME=Example\nVERSION='1.0 stable'\nPRETTY_NAME=\"Example Linux 1.0\"\n"), 0o644); err != nil {
		t.Fatalf("write os-release: %v", err)
	}
	got, err := readOSReleaseName(path)
	if err != nil {
		t.Fatalf("readOSReleaseName() error = %v", err)
	}
	if want := "Example Linux 1.0"; got != want {
		t.Fatalf("readOSReleaseName() = %q, want %q", got, want)
	}
}

func TestReadCPUModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cpuinfo")
	if err := os.WriteFile(path, []byte("processor: 0\nmodel name: Example CPU 9000\n"), 0o644); err != nil {
		t.Fatalf("write cpuinfo: %v", err)
	}
	got, err := readCPUModel(path)
	if err != nil {
		t.Fatalf("readCPUModel() error = %v", err)
	}
	if want := "Example CPU 9000"; got != want {
		t.Fatalf("readCPUModel() = %q, want %q", got, want)
	}
}

func TestWriteEnvironmentReportReturnsOutputError(t *testing.T) {
	err := writeEnvironmentReport(
		configuration{runtimeRoot: t.TempDir(), cc: filepath.Join(t.TempDir(), "missing-c++")},
		nil,
		true,
		environmentErrorWriter{},
	)
	if err == nil || !strings.Contains(err.Error(), "write environment report") {
		t.Fatalf("writeEnvironmentReport() error = %v, want output error", err)
	}
}

type environmentErrorWriter struct{}

func (environmentErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}
