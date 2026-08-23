package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunProgramForwardsStreamsAndExitCode(t *testing.T) {
	project := t.TempDir()
	program := filepath.Join(project, "program")
	writeBuildFile(
		t,
		project,
		"program",
		"#!/bin/sh\n"+
			"printf 'cwd=%s\\n' \"$PWD\"\n"+
			"printf 'argument=%s\\n' \"$1\"\n"+
			"IFS= read -r input\n"+
			"printf 'stdin=%s\\n' \"$input\"\n"+
			"printf 'stderr=%s\\n' \"$2\" >&2\n",
	)
	if err := os.Chmod(program, 0o755); err != nil {
		t.Fatalf("make program executable: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runProgram(
		program,
		[]string{"two words", "diagnostic"},
		project,
		strings.NewReader("input value\n"),
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("runProgram() error = %v", err)
	}
	wantStdout := "cwd=" + project + "\nargument=two words\nstdin=input value\n"
	if got := stdout.String(); got != wantStdout {
		t.Fatalf("runProgram() stdout = %q, want %q", got, wantStdout)
	}
	if got, want := stderr.String(), "stderr=diagnostic\n"; got != want {
		t.Fatalf("runProgram() stderr = %q, want %q", got, want)
	}

	writeBuildFile(t, project, "failure", "#!/bin/sh\nexit 7\n")
	failure := filepath.Join(project, "failure")
	if err := os.Chmod(failure, 0o755); err != nil {
		t.Fatalf("make failure program executable: %v", err)
	}
	err = runProgram(failure, nil, project, nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("runProgram(failure) error = nil")
	}
	code, ok := runProgramExitCode(err)
	if !ok || code != 7 {
		t.Fatalf("runProgramExitCode() = %d, %v, want 7, true; error: %v", code, ok, err)
	}
	if _, ok := runProgramExitCode(os.ErrNotExist); ok {
		t.Fatal("runProgramExitCode() ok = true for a non-program error")
	}
}

func TestValidateRunLinks(t *testing.T) {
	if err := validateRunLinks([]linkJob{{source: "app.cpp"}}); err != nil {
		t.Fatalf("validateRunLinks() error = %v", err)
	}
	if err := validateRunLinks(nil); err == nil ||
		!strings.Contains(err.Error(), "exactly one entry source, found 0") {
		t.Fatalf("validateRunLinks(nil) error = %v", err)
	}
	err := validateRunLinks([]linkJob{
		{source: "first.cpp"},
		{source: "nested/second.cpp"},
	})
	if err == nil {
		t.Fatal("validateRunLinks(multiple) error = nil")
	}
	for _, wanted := range []string{"found 2", "first.cpp", "nested/second.cpp"} {
		if !strings.Contains(err.Error(), wanted) {
			t.Fatalf("validateRunLinks(multiple) error = %q, want %q", err, wanted)
		}
	}
}

func TestRunSourcesBuildsOnlyInternalArtifactAndForwardsIO(t *testing.T) {
	compiler, err := exec.LookPath("c++")
	if err != nil {
		t.Skipf("c++ is unavailable: %v", err)
	}
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(
		t,
		project,
		"app.cpp",
		"#include <iostream>\n"+
			"#include <string>\n"+
			"int main(int argc, char** argv) {\n"+
			"    std::string input;\n"+
			"    std::getline(std::cin, input);\n"+
			"    std::cout << argv[1] << ':' << input << '\\n';\n"+
			"    std::cerr << argv[2] << '\\n';\n"+
			"    return argc == 3 ? 0 : 1;\n"+
			"}\n",
	)
	withWorkingDirectory(t, project)

	var stdout, stderr bytes.Buffer
	progress := newProgressBar(&stdout, -1, false, true, true)
	err = runSourcesWithProgress(
		root,
		"integration",
		compiler,
		[]string{"-std=c++20"},
		[]string{"-std=c++20"},
		[]string{"main", "_start"},
		[]string{"app.cpp"},
		[]string{"two words", "diagnostic"},
		2,
		false,
		true,
		progress,
		strings.NewReader("input value\n"),
		&stdout,
		&stderr,
		false,
	)
	if err != nil {
		t.Fatalf("runSourcesWithProgress() error = %v", err)
	}
	if got, want := stdout.String(), "two words:input value\n"; got != want {
		t.Fatalf("run stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "diagnostic\n"; got != want {
		t.Fatalf("run stderr = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(project, "app")); !os.IsNotExist(err) {
		t.Fatalf("source-side binary exists or stat failed: %v", err)
	}
	artifact, err := binaryArtifactPath(root, "integration", "app.cpp")
	if err != nil {
		t.Fatalf("binaryArtifactPath() error = %v", err)
	}
	info, err := os.Stat(artifact)
	if err != nil {
		t.Fatalf("stat internal artifact: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("internal artifact is not executable: %v", info.Mode())
	}
}

func TestRunSourcesUsesBuildCacheButAlwaysRunsProgram(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "app.cpp", "int main() { return 0; }\n")
	compiler := filepath.Join(t.TempDir(), "c++")
	compilerLog := filepath.Join(t.TempDir(), "compiler.log")
	runLog := filepath.Join(t.TempDir(), "run.log")
	compilerScript := "#!/bin/sh\n" +
		"printf 'compiler\\n' >> \"$COMPILER_LOG\"\n" +
		"output=''\n" +
		"compile='false'\n" +
		"previous=''\n" +
		"for argument in \"$@\"; do\n" +
		"    if [ \"$argument\" = '-c' ]; then compile='true'; fi\n" +
		"    if [ \"$previous\" = '-o' ]; then output=$argument; fi\n" +
		"    previous=$argument\n" +
		"done\n" +
		"if [ \"$compile\" = 'true' ]; then\n" +
		"    printf 'object\\n' > \"$output\"\n" +
		"else\n" +
		"    printf '%s\\n' '#!/bin/sh' 'printf \"run\\n\" >> \"$RUN_LOG\"' > \"$output\"\n" +
		"    chmod +x \"$output\"\n" +
		"fi\n"
	if err := os.WriteFile(compiler, []byte(compilerScript), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}
	t.Setenv("COMPILER_LOG", compilerLog)
	t.Setenv("RUN_LOG", runLog)
	withWorkingDirectory(t, project)

	run := func(noCache bool) string {
		t.Helper()
		var stdout bytes.Buffer
		progress := newProgressBar(&stdout, -1, true, false, true)
		err := runSourcesWithProgress(
			root,
			"host",
			compiler,
			nil,
			nil,
			[]string{"main", "_start"},
			[]string{"app.cpp"},
			nil,
			1,
			true,
			false,
			progress,
			nil,
			&stdout,
			io.Discard,
			noCache,
		)
		if err != nil {
			t.Fatalf("runSourcesWithProgress(noCache=%v) error = %v", noCache, err)
		}
		if strings.Contains(stdout.String(), "Copying ") {
			t.Fatalf("run progress contains delivery step: %q", stdout.String())
		}
		return stdout.String()
	}

	first := run(false)
	artifact, err := binaryArtifactPath(root, "host", "app.cpp")
	if err != nil {
		t.Fatalf("binaryArtifactPath() error = %v", err)
	}
	if strings.Contains(first, "(CACHED)") {
		t.Fatalf("first run unexpectedly used cache: %q", first)
	}
	if !strings.HasSuffix(first, string(renderRunCommand(artifact, nil))) {
		t.Fatalf("first run does not end with execution command %q: %q", artifact, first)
	}
	if got := countRunTestLines(t, compilerLog, "compiler\n"); got != 2 {
		t.Fatalf("compiler invocations after first run = %d, want 2", got)
	}
	if got := countRunTestLines(t, runLog, "run\n"); got != 1 {
		t.Fatalf("program invocations after first run = %d, want 1", got)
	}

	second := run(false)
	for _, wanted := range []string{"Compiling app.cpp (CACHED)", "Linking app (CACHED)"} {
		if !strings.Contains(second, wanted) {
			t.Fatalf("second run does not contain %q: %q", wanted, second)
		}
	}
	if !strings.HasSuffix(second, string(renderRunCommand(artifact, nil))) {
		t.Fatalf("cached run does not end with execution command %q: %q", artifact, second)
	}
	if got := countRunTestLines(t, compilerLog, "compiler\n"); got != 2 {
		t.Fatalf("compiler invocations after cached run = %d, want 2", got)
	}
	if got := countRunTestLines(t, runLog, "run\n"); got != 2 {
		t.Fatalf("program invocations after cached run = %d, want 2", got)
	}

	third := run(true)
	if strings.Contains(third, "(CACHED)") {
		t.Fatalf("no-cache run unexpectedly used cache: %q", third)
	}
	if got := countRunTestLines(t, compilerLog, "compiler\n"); got != 4 {
		t.Fatalf("compiler invocations after no-cache run = %d, want 4", got)
	}
	if got := countRunTestLines(t, runLog, "run\n"); got != 3 {
		t.Fatalf("program invocations after no-cache run = %d, want 3", got)
	}
	if _, err := os.Stat(filepath.Join(project, "app")); !os.IsNotExist(err) {
		t.Fatalf("source-side binary exists or stat failed: %v", err)
	}
}

func TestRunSourcesRejectsMultipleEntrySourcesBeforeCompilation(t *testing.T) {
	root := t.TempDir()
	project := t.TempDir()
	writeBuildFile(t, project, "first.cpp", "int main() { return 0; }\n")
	writeBuildFile(t, project, "second.cpp", "int main() { return 0; }\n")
	withWorkingDirectory(t, project)

	progress := newProgressBar(io.Discard, -1, false, true, true)
	err := runSourcesWithProgress(
		root,
		"host",
		filepath.Join(project, "compiler-must-not-run"),
		nil,
		nil,
		[]string{"main", "_start"},
		[]string{"first.cpp", "second.cpp"},
		nil,
		1,
		false,
		true,
		progress,
		nil,
		io.Discard,
		io.Discard,
		false,
	)
	if err == nil {
		t.Fatal("runSourcesWithProgress() error = nil")
	}
	for _, wanted := range []string{"found 2", "first.cpp", "second.cpp"} {
		if !strings.Contains(err.Error(), wanted) {
			t.Fatalf("runSourcesWithProgress() error = %q, want %q", err, wanted)
		}
	}
}

func TestRenderRunCommandQuotesArguments(t *testing.T) {
	got := string(renderRunCommand("/tmp/program path", []string{"plain", "two words", "it's"}))
	want := "'/tmp/program path' plain 'two words' 'it'\"'\"'s'\n"
	if got != want {
		t.Fatalf("renderRunCommand() = %q, want %q", got, want)
	}
}

func countRunTestLines(t *testing.T, path, line string) int {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Count(string(contents), line)
}
