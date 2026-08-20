package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTestSourcesEmptySelectionDoesNotRequirePkgConfig(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := testSources(
		t.TempDir(),
		"host",
		"missing-c++",
		nil,
		nil,
		nil,
		1,
		false,
		false,
		true,
		io.Discard,
		io.Discard,
	); err != nil {
		t.Fatalf("testSources() error = %v", err)
	}
}

func TestPkgConfigFlagsParsesOutputAndPreservesDiagnostics(t *testing.T) {
	directory := t.TempDir()
	pkgConfig := filepath.Join(directory, "pkg-config")
	script := "#!/bin/sh\n" +
		"printf 'pkg-config warning\\n' >&2\n" +
		"printf '%s\\n' \"-DGTEST=1 '-DNAME=two words'\"\n"
	if err := os.WriteFile(pkgConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pkg-config: %v", err)
	}
	t.Setenv("PATH", directory)

	var stderr bytes.Buffer
	got, err := pkgConfigFlags("--cflags", t.TempDir(), &stderr)
	if err != nil {
		t.Fatalf("pkgConfigFlags() error = %v", err)
	}
	want := []string{"-DGTEST=1", "-DNAME=two words"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pkgConfigFlags() = %#v, want %#v", got, want)
	}
	if got := stderr.String(); got != "pkg-config warning\n" {
		t.Fatalf("pkgConfigFlags() stderr = %q", got)
	}
}

func TestPkgConfigFlagsReportsCommandFailure(t *testing.T) {
	directory := t.TempDir()
	pkgConfig := filepath.Join(directory, "pkg-config")
	script := "#!/bin/sh\n" +
		"printf 'gtest_main is unavailable\\n' >&2\n" +
		"exit 7\n"
	if err := os.WriteFile(pkgConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pkg-config: %v", err)
	}
	t.Setenv("PATH", directory)

	_, err := pkgConfigFlags("--libs", t.TempDir(), io.Discard)
	if err == nil {
		t.Fatal("pkgConfigFlags() error = nil")
	}
	for _, want := range []string{"pkg-config --libs gtest_main", "exit status 7", "gtest_main is unavailable"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("pkgConfigFlags() error = %q, want %q", err, want)
		}
	}
}

func TestTestSourcesBuildsAndRunsEveryTest(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "shared.h", "#pragma once\nint answer();\n")
	writeBuildFile(t, project, "shared.cpp", "#include \"shared.h\"\nint answer() { return 42; }\n")
	writeBuildFile(t, project, "pass_test.cpp", "#include \"shared.h\"\n")
	writeBuildFile(t, project, "fail_test.cpp", "#include \"shared.h\"\n")
	compiler, log := installTestTools(t)
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := testSources(
		root,
		"host",
		compiler,
		[]string{"-DHARD=1"},
		[]string{"-Wl,hard"},
		[]string{"pass_test.cpp", "fail_test.cpp"},
		2,
		true,
		false,
		true,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("testSources() error = nil")
	}
	if !strings.Contains(err.Error(), "test fail_test.cpp: exit status 7") {
		t.Fatalf("testSources() error = %q", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Parsing pass_test.cpp\n",
		"Parsing fail_test.cpp\n",
		"Parsing shared.h\n",
		"Linking pass_test\n",
		"Testing pass_test\n",
		"ran pass_test --gtest_color=no\n",
		"Linking fail_test\n",
		"Testing fail_test\n",
		"ran fail_test --gtest_color=no\n",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("testSources() output does not contain %q: %q", want, output)
		}
	}
	for _, source := range []string{"pass_test.cpp", "fail_test.cpp", "shared.cpp"} {
		object, err := objectFilePath(root, "host", source)
		if err != nil {
			t.Fatalf("objectFilePath(%q) error = %v", source, err)
		}
		if _, err := os.Stat(object); err != nil {
			t.Errorf("stat object for %s: %v", source, err)
		}
	}
	for _, source := range []string{"pass_test.cpp", "fail_test.cpp"} {
		artifact, err := binaryArtifactPath(root, "host", source)
		if err != nil {
			t.Fatalf("binaryArtifactPath(%q) error = %v", source, err)
		}
		if _, err := os.Stat(artifact); err != nil {
			t.Errorf("stat test binary for %s: %v", source, err)
		}
		if _, err := os.Stat(filepath.Join(project, sourceBinaryName(source))); !os.IsNotExist(err) {
			t.Errorf("test binary was copied into project for %s: %v", source, err)
		}
	}
	passObject, err := objectFilePath(root, "host", "pass_test.cpp")
	if err != nil {
		t.Fatalf("objectFilePath(pass_test.cpp) error = %v", err)
	}
	sharedObject, err := objectFilePath(root, "host", "shared.cpp")
	if err != nil {
		t.Fatalf("objectFilePath(shared.cpp) error = %v", err)
	}
	forward, err := forwardHeaderPath(root, "host", filepath.Join(project, "shared.h"))
	if err != nil {
		t.Fatalf("forwardHeaderPath(shared.h) error = %v", err)
	}
	passArtifact, err := binaryArtifactPath(root, "host", "pass_test.cpp")
	if err != nil {
		t.Fatalf("binaryArtifactPath(pass_test.cpp) error = %v", err)
	}
	for _, want := range []string{
		"Compiling pass_test.cpp\n" + string(renderCompileCommand(
			compiler,
			[]string{"-DHARD=1", "-DGTEST=1"},
			[]string{forward},
			"pass_test.cpp",
			passObject,
		)),
		"Linking pass_test\n" + string(renderLinkCommand(
			compiler,
			[]string{"-Wl,hard", "-lgtest_main", "-lgtest"},
			[]string{passObject, sharedObject},
			passArtifact,
		)),
		"Testing pass_test\n" +
			string(renderTestCommand(
				passArtifact,
				[]string{"--gtest_color=no"},
			)) +
			"ran pass_test --gtest_color=no\n",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("testSources() verbose output does not contain %q: %q", want, output)
		}
	}

	toolLog := readTestFile(t, log)
	for _, want := range []string{
		"pkg-config --cflags gtest_main",
		"pkg-config --libs gtest_main",
		"-DHARD=1",
		"-DGTEST=1",
		"-Wl,hard",
		"-lgtest_main",
		"-lgtest",
	} {
		if !strings.Contains(toolLog, want) {
			t.Errorf("tool log does not contain %q: %q", want, toolLog)
		}
	}
	if got := strings.Count(output, "Compiling shared.cpp\n"); got != 1 {
		t.Errorf("shared.cpp compile count = %d, want 1: %q", got, output)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("testSources() stderr = %q", got)
	}
}

func TestTestSourcesDoesNotGenerateForwardForEnvironmentSupportHeader(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	supportDirectory := t.TempDir()
	supportHeader := filepath.Join(supportDirectory, "hard.h")
	writeBuildFile(t, supportDirectory, "hard.h", "#pragma once\nstruct hard_support {};\n")
	environmentHeader := filepath.Join(root, "env", "host", "hard.h")
	if err := os.MkdirAll(filepath.Dir(environmentHeader), 0o755); err != nil {
		t.Fatalf("create environment directory: %v", err)
	}
	if err := os.Symlink(supportHeader, environmentHeader); err != nil {
		t.Fatalf("create environment support-header symlink: %v", err)
	}
	writeBuildFile(t, project, "hard.h", "#pragma once\nstruct project_type {};\n")
	writeBuildFile(t, project, "pass_test.cpp", "#include \"hard.h\"\n")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	cflags := []string{"-include", environmentHeader}
	var stdout bytes.Buffer
	if err := testSources(
		root,
		"host",
		compiler,
		cflags,
		nil,
		[]string{"pass_test.cpp"},
		1,
		true,
		false,
		true,
		&stdout,
		io.Discard,
	); err != nil {
		t.Fatalf("testSources() error = %v", err)
	}

	projectHeader := filepath.Join(project, "hard.h")
	projectForward, err := forwardHeaderPath(root, "host", projectHeader)
	if err != nil {
		t.Fatalf("forwardHeaderPath(project hard.h) error = %v", err)
	}
	if _, err := os.Stat(projectForward); err != nil {
		t.Fatalf("stat project forward header: %v", err)
	}
	supportForward, err := forwardHeaderPath(root, "host", supportHeader)
	if err != nil {
		t.Fatalf("forwardHeaderPath(hard.h) error = %v", err)
	}
	if _, err := os.Stat(supportForward); !os.IsNotExist(err) {
		t.Fatalf("stat support forward header error = %v, want not exist", err)
	}

	object, err := objectFilePath(root, "host", "pass_test.cpp")
	if err != nil {
		t.Fatalf("objectFilePath(pass_test.cpp) error = %v", err)
	}
	output := stdout.String()
	wantCommand := string(renderCompileCommand(
		compiler,
		[]string{"-include", environmentHeader, "-DGTEST=1"},
		[]string{projectForward},
		"pass_test.cpp",
		object,
	))
	for _, want := range []string{
		"Parsing hard.h\n",
		"Compiling pass_test.cpp\n" + wantCommand,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("testSources() output does not contain %q: %q", want, output)
		}
	}
	for _, forbidden := range []string{
		"Parsing " + buildParsingDisplayPath(root, supportHeader, project),
		supportForward,
	} {
		if strings.Contains(output, forbidden) {
			t.Errorf("testSources() output contains %q: %q", forbidden, output)
		}
	}
}

func TestTestSourcesContinuesAfterCompileFailure(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "shared.h", "#pragma once\n")
	writeBuildFile(t, project, "shared.cpp", "")
	writeBuildFile(t, project, "broken_test.cpp", "#include \"shared.h\"\n")
	writeBuildFile(t, project, "pass_test.cpp", "#include \"shared.h\"\n")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := testSources(
		root,
		"host",
		compiler,
		nil,
		nil,
		[]string{"broken_test.cpp", "pass_test.cpp"},
		1,
		false,
		false,
		true,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("testSources() error = nil")
	}
	if !strings.Contains(err.Error(), "compile broken_test.cpp: exit status 9") {
		t.Fatalf("testSources() error = %q", err)
	}
	if strings.Contains(stdout.String(), "Testing broken_test") {
		t.Fatalf("failed test was executed: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[6/8] Testing pass_test") {
		t.Fatalf("later test was not executed: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ran pass_test") || strings.Contains(stderr.String(), "ran pass_test") {
		t.Fatalf("successful test output was printed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if got := stderr.String(); got != "compile failed for broken_test.cpp\n" {
		t.Fatalf("testSources() stderr = %q", got)
	}
}

func TestTestSourcesNormalOutputHidesSuccessAndReportsFailure(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "shared.h", "#pragma once\n")
	writeBuildFile(t, project, "shared.cpp", "")
	writeBuildFile(t, project, "pass_test.cpp", "#include \"shared.h\"\n")
	writeBuildFile(t, project, "fail_test.cpp", "#include \"shared.h\"\n")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := testSources(
		root,
		"host",
		compiler,
		nil,
		nil,
		[]string{"pass_test.cpp", "fail_test.cpp"},
		1,
		false,
		false,
		true,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("testSources() error = nil")
	}
	if !strings.Contains(err.Error(), "test fail_test.cpp: exit status 7") {
		t.Fatalf("testSources() error = %q", err)
	}
	if !strings.Contains(stdout.String(), "\r[8/8] Testing fail_test\n") {
		t.Fatalf("testSources() common progress = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ran pass_test") || strings.Contains(stderr.String(), "ran pass_test") {
		t.Fatalf("successful test output was printed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "\nran fail_test --gtest_color=no\n") {
		t.Fatalf("testSources() failed output = %q", stdout.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("testSources() stderr = %q, want empty output", got)
	}
}

func TestTestSourcesContinuesSearchProgressThroughParsing(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "pass_test.cpp", "")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	progress := newProgressBar(&stdout, -1, true, false, true)
	progress.updateStep("Searching source files")
	if err := testSourcesWithProgress(
		root,
		"host",
		compiler,
		nil,
		nil,
		[]string{"pass_test.cpp"},
		1,
		true,
		false,
		true,
		progress,
		&stdout,
		io.Discard,
	); err != nil {
		t.Fatalf("testSourcesWithProgress() error = %v", err)
	}

	output := stdout.String()
	wants := []string{
		"[1/?] Searching source files\n",
		"[1/?] Parsing pass_test.cpp\n",
		"[2/4] Compiling pass_test.cpp\n",
		"[3/4] Linking pass_test\n",
		"[4/4] Testing pass_test\n",
	}
	previous := -1
	for _, want := range wants {
		index := strings.Index(output, want)
		if index < 0 {
			t.Fatalf("test output does not contain %q: %q", want, output)
		}
		if index <= previous {
			t.Fatalf("test output order is wrong for %q: %q", want, output)
		}
		previous = index
	}
}

func TestTestSourcesSilentOutputOnlyReportsFailures(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "shared.h", "#pragma once\n")
	writeBuildFile(t, project, "shared.cpp", "")
	writeBuildFile(t, project, "pass_test.cpp", "#include \"shared.h\"\n")
	writeBuildFile(t, project, "fail_test.cpp", "#include \"shared.h\"\n")
	compiler, _ := installTestTools(t)
	withWorkingDirectory(t, project)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := testSources(
		root,
		"host",
		compiler,
		nil,
		nil,
		[]string{"pass_test.cpp", "fail_test.cpp"},
		1,
		true,
		true,
		true,
		&stdout,
		&stderr,
	)
	if err == nil {
		t.Fatal("testSources() error = nil")
	}
	if stdout.Len() != 0 {
		t.Fatalf("testSources() stdout = %q, want empty output", stdout.String())
	}
	if got := stderr.String(); got != "ran fail_test --gtest_color=no\n" {
		t.Fatalf("testSources() stderr = %q", got)
	}
}

func TestTestSourcesUsesJobsAcrossTestFiles(t *testing.T) {
	tests := []struct {
		name string
		jobs int
		want int
	}{
		{name: "one job", jobs: 1, want: 1},
		{name: "two jobs", jobs: 2, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			project := t.TempDir()
			sources := []string{
				"first_test.cpp",
				"second_test.cpp",
				"third_test.cpp",
				"fourth_test.cpp",
			}
			for _, source := range sources {
				writeBuildFile(t, project, source, "")
			}
			compiler, log := installParallelTestTools(t)
			withWorkingDirectory(t, project)

			if err := testSources(
				root,
				"host",
				compiler,
				nil,
				nil,
				sources,
				tt.jobs,
				false,
				true,
				true,
				io.Discard,
				io.Discard,
			); err != nil {
				t.Fatalf("testSources() error = %v", err)
			}

			events := readTestFile(t, log)
			for _, phase := range []string{"compile", "link", "test"} {
				if got := maximumPhaseParallelism(events, phase); got != tt.want {
					t.Errorf(
						"%s maximum parallelism = %d, want %d:\n%s",
						phase,
						got,
						tt.want,
						events,
					)
				}
			}
		})
	}
}

func TestTestBinaryArgumentsAndCommand(t *testing.T) {
	if got, want := testBinaryArguments(false), []string{"--gtest_color=yes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("testBinaryArguments(false) = %#v, want %#v", got, want)
	}
	if got, want := testBinaryArguments(true), []string{"--gtest_color=no"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("testBinaryArguments(true) = %#v, want %#v", got, want)
	}
	got := string(renderTestCommand("/tmp/test binary", []string{"--gtest_filter=two words", "O'Reilly"}))
	want := "'/tmp/test binary' '--gtest_filter=two words' 'O'\"'\"'Reilly'\n"
	if got != want {
		t.Fatalf("renderTestCommand() = %q, want %q", got, want)
	}
}

func TestRunTestsForcesAndPreservesGoogleTestColor(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "color_test")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" != '--gtest_color=yes' ]; then exit 8; fi\n" +
		"printf '\\033[31mfailed test output\\033[0m\\n'\n" +
		"exit 7\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake test: %v", err)
	}

	var stdout bytes.Buffer
	progress := newProgressBar(&stdout, 1, true, false, true)
	results := runTests(
		[]testRunJob{{source: "color_test.cpp", binary: binary}},
		1,
		true,
		false,
		false,
		directory,
		progress,
	)
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}
	if len(results) != 1 || results[0] == nil {
		t.Fatalf("runTests() results = %#v", results)
	}
	if results[0].err == nil || !strings.Contains(results[0].err.Error(), "exit status 7") {
		t.Fatalf("runTests() error = %v, want exit status 7", results[0].err)
	}
	coloredOutput := "\x1b[31mfailed test output\x1b[0m\n"
	if got := string(results[0].output); got != coloredOutput {
		t.Fatalf("runTests() failed output = %q, want %q", got, coloredOutput)
	}
	for _, want := range []string{"--gtest_color=yes", coloredOutput} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("runTests() verbose output does not contain %q: %q", want, stdout.String())
		}
	}
}

func maximumPhaseParallelism(events, phase string) int {
	active := 0
	maximum := 0
	for _, line := range strings.Split(events, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case phase + "-start":
			active++
			if active > maximum {
				maximum = active
			}
		case phase + "-end":
			active--
		}
	}
	return maximum
}

func installParallelTestTools(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	log := filepath.Join(t.TempDir(), "parallel.log")
	t.Setenv("PARALLEL_LOG", log)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	pkgConfig := filepath.Join(directory, "pkg-config")
	pkgConfigScript := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"\t--cflags|--libs) exit 0 ;;\n" +
		"\t*) exit 8 ;;\n" +
		"esac\n"
	if err := os.WriteFile(pkgConfig, []byte(pkgConfigScript), 0o755); err != nil {
		t.Fatalf("write fake pkg-config: %v", err)
	}

	compiler := filepath.Join(directory, "c++")
	compilerScript := "#!/bin/sh\n" +
		"mode=link\n" +
		"source=''\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tcase \"$argument\" in\n" +
		"\t\t-c) mode=compile ;;\n" +
		"\t\t*.c|*.cc|*.cpp|*.c++) source=$argument ;;\n" +
		"\tesac\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"log_phase() {\n" +
		"\tprintf '%s-start %s\\n' \"$1\" \"$2\" >> \"$PARALLEL_LOG\"\n" +
		"\t/bin/sleep 0.10\n" +
		"\tprintf '%s-end %s\\n' \"$1\" \"$2\" >> \"$PARALLEL_LOG\"\n" +
		"}\n" +
		"if [ \"$mode\" = compile ]; then\n" +
		"\tlog_phase compile \"$source\"\n" +
		"\tprintf 'object\\n' > \"$output\"\n" +
		"\texit 0\n" +
		"fi\n" +
		"log_phase link \"$output\"\n" +
		"printf '%s\\n' '#!/bin/sh' > \"$output\"\n" +
		"printf '%s\\n' 'name=${0##*/}' >> \"$output\"\n" +
		"printf '%s\\n' 'printf \"test-start %s\\n\" \"$name\" >> \"$PARALLEL_LOG\"' >> \"$output\"\n" +
		"printf '%s\\n' '/bin/sleep 0.10' >> \"$output\"\n" +
		"printf '%s\\n' 'printf \"test-end %s\\n\" \"$name\" >> \"$PARALLEL_LOG\"' >> \"$output\"\n" +
		"/bin/chmod 755 \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(compilerScript), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	return compiler, log
}

func installTestTools(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	log := filepath.Join(t.TempDir(), "tools.log")
	t.Setenv("TOOL_LOG", log)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	pkgConfig := filepath.Join(directory, "pkg-config")
	pkgConfigScript := "#!/bin/sh\n" +
		"printf 'pkg-config %s %s\\n' \"$1\" \"$2\" >> \"$TOOL_LOG\"\n" +
		"case \"$1\" in\n" +
		"\t--cflags) printf '%s\\n' '-DGTEST=1' ;;\n" +
		"\t--libs) printf '%s\\n' '-lgtest_main -lgtest' ;;\n" +
		"\t*) exit 8 ;;\n" +
		"esac\n"
	if err := os.WriteFile(pkgConfig, []byte(pkgConfigScript), 0o755); err != nil {
		t.Fatalf("write fake pkg-config: %v", err)
	}

	compiler := filepath.Join(directory, "c++")
	compilerScript := "#!/bin/sh\n" +
		"printf 'compiler' >> \"$TOOL_LOG\"\n" +
		"for argument in \"$@\"; do printf ' %s' \"$argument\" >> \"$TOOL_LOG\"; done\n" +
		"printf '\\n' >> \"$TOOL_LOG\"\n" +
		"mode=link\n" +
		"source=''\n" +
		"output=''\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"\tcase \"$argument\" in\n" +
		"\t\t-c) mode=compile ;;\n" +
		"\t\t*.c|*.cc|*.cpp|*.c++) source=$argument ;;\n" +
		"\tesac\n" +
		"\tif [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"\tprevious=$argument\n" +
		"done\n" +
		"if [ \"$mode\" = compile ]; then\n" +
		"\tif [ \"$source\" = broken_test.cpp ]; then\n" +
		"\t\tprintf 'compile failed for broken_test.cpp\\n' >&2\n" +
		"\t\texit 9\n" +
		"\tfi\n" +
		"\tprintf 'object\\n' > \"$output\"\n" +
		"\texit 0\n" +
		"fi\n" +
		"printf '%s\\n' '#!/bin/sh' > \"$output\"\n" +
		"printf '%s\\n' 'name=${0##*/}' >> \"$output\"\n" +
		"printf '%s\\n' 'printf \"ran %s %s\\n\" \"$name\" \"$*\"' >> \"$output\"\n" +
		"printf '%s\\n' 'case \"$name\" in fail_test) exit 7 ;; esac' >> \"$output\"\n" +
		"/bin/chmod 755 \"$output\"\n"
	if err := os.WriteFile(compiler, []byte(compilerScript), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	return compiler, log
}
