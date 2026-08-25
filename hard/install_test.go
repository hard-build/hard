package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptInstallsReleaseAndConfiguresShell(t *testing.T) {
	tests := []struct {
		name            string
		shell           string
		configPath      string
		pathEntry       string
		completionEntry string
	}{
		{
			name:            "bash",
			shell:           "/bin/bash",
			configPath:      ".bashrc",
			pathEntry:       `export PATH="$HOME/.local/bin:$PATH"`,
			completionEntry: `[ -r "$HOME/.local/share/bash-completion/completions/hard" ] && . "$HOME/.local/share/bash-completion/completions/hard"`,
		},
		{
			name:            "zsh",
			shell:           "/usr/bin/zsh",
			configPath:      ".zshrc",
			pathEntry:       `export PATH="$HOME/.local/bin:$PATH"`,
			completionEntry: `autoload -Uz compinit && compinit && . "$HOME/.local/share/zsh/site-functions/_hard"`,
		},
		{
			name:       "fish",
			shell:      "/usr/bin/fish",
			configPath: filepath.Join(".config", "fish", "config.fish"),
			pathEntry:  `fish_add_path "$HOME/.local/bin"`,
		},
		{
			name:       "posix fallback",
			shell:      "/bin/dash",
			configPath: ".profile",
			pathEntry:  `export PATH="$HOME/.local/bin:$PATH"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInstallScript(t, installScriptOptions{shell: tt.shell})
			if result.err != nil {
				t.Fatalf("install error = %v, output = %q", result.err, result.output)
			}

			assertInstallFileMode(t, filepath.Join(result.home, ".local", "bin", "hard"), 0o755)
			assertInstallFileMode(
				t,
				filepath.Join(result.home, ".local", "libexec", "hard", "hard"),
				0o755,
			)
			for _, path := range []string{
				filepath.Join(result.home, ".local", "share", "bash-completion", "completions", "hard"),
				filepath.Join(result.home, ".local", "share", "zsh", "site-functions", "_hard"),
				filepath.Join(result.home, ".local", "share", "fish", "vendor_completions.d", "hard.fish"),
			} {
				assertInstallFileMode(t, path, 0o644)
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read completion file %s: %v", path, err)
				}
				if !strings.Contains(string(contents), "completion for hard") {
					t.Fatalf("completion file %s = %q, want completion marker", path, contents)
				}
			}
			libclang, err := os.Lstat(filepath.Join(
				result.home,
				".local",
				"libexec",
				"hard",
				"lib",
				"libclang.so",
			))
			if err != nil {
				t.Fatalf("stat libclang symlink: %v", err)
			}
			if libclang.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("libclang mode = %v, want symlink", libclang.Mode())
			}
			if _, err := os.Stat(filepath.Join(
				result.home,
				".local",
				"libexec",
				"hard",
				"default-target",
			)); !os.IsNotExist(err) {
				t.Fatalf("default target error = %v, want not exist", err)
			}
			if info, err := os.Stat(filepath.Join(result.home, ".local", "share", "hard")); err != nil {
				t.Fatalf("stat data root: %v", err)
			} else if !info.IsDir() {
				t.Fatal("data root is not a directory")
			}

			config, err := os.ReadFile(filepath.Join(result.home, tt.configPath))
			if err != nil {
				t.Fatalf("read shell config: %v", err)
			}
			wantConfig := tt.pathEntry + "\n"
			if tt.completionEntry != "" {
				wantConfig += "\n" + tt.completionEntry + "\n"
			}
			if got, want := string(config), wantConfig; got != want {
				t.Fatalf("shell config = %q, want %q", got, want)
			}
			if !strings.Contains(result.output, tt.pathEntry) {
				t.Fatalf("install output = %q, want current-shell command %q", result.output, tt.pathEntry)
			}
			if tt.completionEntry != "" &&
				!strings.Contains(result.output, "enabled "+filepath.Base(tt.shell)+" completion") {
				t.Fatalf("install output = %q, want completion enabled message", result.output)
			}
			if result.packageLog != "" {
				t.Errorf("package log = %q, want no package-manager calls", result.packageLog)
			}
			if result.serviceLog != "" {
				t.Errorf("service log = %q, want no service changes", result.serviceLog)
			}
		})
	}
}

func TestInstallScriptDoesNotDuplicateConfiguredPath(t *testing.T) {
	tests := []struct {
		name    string
		options installScriptOptions
	}{
		{
			name: "repeated installation",
			options: installScriptOptions{
				shell:       "/bin/bash",
				repetitions: 2,
			},
		},
		{
			name: "equivalent existing entry",
			options: installScriptOptions{
				shell:               "/bin/bash",
				shellConfigContents: `export PATH="$PATH:$HOME/.local/bin"` + "\n",
			},
		},
		{
			name: "path already active",
			options: installScriptOptions{
				shell:                   "/bin/bash",
				includeInstallBinInPath: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInstallScript(t, tt.options)
			if result.err != nil {
				t.Fatalf("install error = %v, output = %q", result.err, result.output)
			}
			config, err := os.ReadFile(filepath.Join(result.home, ".bashrc"))
			if err != nil {
				t.Fatalf("read shell config: %v", err)
			}
			if count := strings.Count(string(config), ".local/bin"); count != 1 {
				t.Fatalf("shell config = %q, path occurrences = %d, want 1", config, count)
			}
			if count := strings.Count(string(config), "bash-completion/completions/hard"); count != 2 {
				t.Fatalf("shell config = %q, completion path occurrences = %d, want 2", config, count)
			}
			if tt.options.includeInstallBinInPath &&
				!strings.Contains(result.output, "is already present in PATH") {
				t.Fatalf("install output = %q, want existing PATH message", result.output)
			}
		})
	}
}

func TestInstallScriptStopsOnChecksumMismatch(t *testing.T) {
	result := runInstallScript(t, installScriptOptions{badChecksum: true})
	if result.err == nil {
		t.Fatalf("install error = nil, output = %q", result.output)
	}
	if result.packageLog != "" {
		t.Fatalf("package log = %q, want no system package changes", result.packageLog)
	}
	if _, err := os.Stat(filepath.Join(result.home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("installation directory error = %v, want not exist", err)
	}
}

func TestInstallScriptAcceptsMultiDigitReleaseTag(t *testing.T) {
	result := runInstallScript(t, installScriptOptions{
		releaseTag: "v12.34",
		shell:      "/bin/bash",
	})
	if result.err != nil {
		t.Fatalf("install error = %v, output = %q", result.err, result.output)
	}
}

func TestInstallScriptRejectsInvalidLatestReleaseTag(t *testing.T) {
	result := runInstallScript(t, installScriptOptions{
		releaseTag: "v1.0.0",
		shell:      "/bin/bash",
	})
	if result.err == nil {
		t.Fatalf("install error = nil, output = %q", result.output)
	}
	if !strings.Contains(result.output, "latest release has an invalid tag: v1.0.0") {
		t.Fatalf("install output = %q, want invalid release tag error", result.output)
	}
	if result.packageLog != "" {
		t.Fatalf("package log = %q, want no system package changes", result.packageLog)
	}
	if _, err := os.Stat(filepath.Join(result.home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("installation directory error = %v, want not exist", err)
	}
}

func TestInstallScriptRejectsArguments(t *testing.T) {
	result := runInstallScript(t, installScriptOptions{arguments: []string{"docker"}})
	if result.err == nil {
		t.Fatalf("install error = nil, output = %q", result.output)
	}
	if !strings.Contains(result.output, "no arguments are accepted") {
		t.Fatalf("install output = %q, want argument error", result.output)
	}
	if _, err := os.Stat(filepath.Join(result.home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("installation directory error = %v, want not exist", err)
	}
}

func TestInstallScriptRestoresPreviousRuntime(t *testing.T) {
	result := runInstallScript(t, installScriptOptions{failRuntimeMove: true})
	if result.err == nil {
		t.Fatalf("install error = nil, output = %q", result.output)
	}
	marker, err := os.ReadFile(filepath.Join(
		result.home,
		".local",
		"libexec",
		"hard",
		"previous-runtime",
	))
	if err != nil {
		t.Fatalf("read previous runtime marker: %v", err)
	}
	if got := string(marker); got != "previous\n" {
		t.Fatalf("previous runtime marker = %q, want %q", got, "previous\n")
	}
	stagingDirectories, err := filepath.Glob(filepath.Join(
		result.home,
		".local",
		"libexec",
		".hard-install.*",
	))
	if err != nil {
		t.Fatalf("find staging directories: %v", err)
	}
	if len(stagingDirectories) != 0 {
		t.Fatalf("staging directories = %#v, want none", stagingDirectories)
	}
}

type installScriptOptions struct {
	arguments               []string
	badChecksum             bool
	failRuntimeMove         bool
	includeInstallBinInPath bool
	releaseTag              string
	repetitions             int
	shell                   string
	shellConfigContents     string
}

type installScriptResult struct {
	home       string
	output     string
	packageLog string
	serviceLog string
	err        error
}

func runInstallScript(t *testing.T, options installScriptOptions) installScriptResult {
	t.Helper()
	if options.releaseTag == "" {
		options.releaseTag = "v1.0"
	}
	if options.repetitions == 0 {
		options.repetitions = 1
	}
	if options.shell == "" {
		options.shell = "/bin/bash"
	}
	temporaryDirectory := t.TempDir()
	home := filepath.Join(temporaryDirectory, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatalf("create home: %v", err)
	}
	if options.failRuntimeMove {
		previousRuntime := filepath.Join(home, ".local", "libexec", "hard")
		if err := os.MkdirAll(previousRuntime, 0o755); err != nil {
			t.Fatalf("create previous runtime: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(previousRuntime, "previous-runtime"),
			[]byte("previous\n"),
			0o644,
		); err != nil {
			t.Fatalf("write previous runtime marker: %v", err)
		}
	}
	if options.shellConfigContents != "" {
		shellConfig := installTestShellConfig(home, options.shell)
		if err := os.MkdirAll(filepath.Dir(shellConfig), 0o755); err != nil {
			t.Fatalf("create shell config directory: %v", err)
		}
		if err := os.WriteFile(
			shellConfig,
			[]byte(options.shellConfigContents),
			0o644,
		); err != nil {
			t.Fatalf("write shell config: %v", err)
		}
	}
	archive, checksum := createInstallArchive(
		t, temporaryDirectory, options.badChecksum, options.releaseTag,
	)
	fakeBin := filepath.Join(temporaryDirectory, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatalf("create fake bin: %v", err)
	}
	packageLog := filepath.Join(temporaryDirectory, "packages.log")
	serviceLog := filepath.Join(temporaryDirectory, "services.log")
	dockerState := filepath.Join(temporaryDirectory, "docker.ready")

	writeInstallTestExecutable(t, filepath.Join(fakeBin, "curl"), `#!/bin/sh
output=
url=
while [ "$#" -gt 0 ]; do
	case "$1" in
		--output)
			output=$2
			shift 2
			;;
		--write-out)
			shift 2
			;;
		https://*)
			url=$1
			shift
			;;
		*) shift ;;
	esac
done
case "$url" in
	https://github.com/hard-build/hard/releases/latest)
		printf 'https://github.com/hard-build/hard/releases/tag/%s' "$INSTALL_TEST_RELEASE_TAG"
		;;
	"https://github.com/hard-build/hard/releases/download/$INSTALL_TEST_RELEASE_TAG/hard-$INSTALL_TEST_RELEASE_TAG.tar.gz")
		cp "$INSTALL_TEST_ARCHIVE" "$output"
		;;
	"https://github.com/hard-build/hard/releases/download/$INSTALL_TEST_RELEASE_TAG/hard-$INSTALL_TEST_RELEASE_TAG.tar.gz.sha256")
		cp "$INSTALL_TEST_CHECKSUM" "$output"
		;;
	*) exit 1 ;;
esac
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "sudo"), `#!/bin/sh
if [ "${1:-}" = -v ]; then
	exit 0
fi
exec "$@"
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "apt-get"), `#!/bin/sh
printf 'apt-get' >> "$INSTALL_TEST_PACKAGE_LOG"
printf ' %s' "$@" >> "$INSTALL_TEST_PACKAGE_LOG"
printf '\n' >> "$INSTALL_TEST_PACKAGE_LOG"
case " $* " in
	*' docker.io '*) : > "$INSTALL_TEST_DOCKER_STATE" ;;
esac
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "docker"), `#!/bin/sh
if [ "${1:-}" = info ] && [ -f "$INSTALL_TEST_DOCKER_STATE" ]; then
	exit 0
fi
exit 1
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "systemctl"), `#!/bin/sh
printf '%s\n' "$*" >> "$INSTALL_TEST_SERVICE_LOG"
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "usermod"), `#!/bin/sh
printf 'usermod %s\n' "$*" >> "$INSTALL_TEST_SERVICE_LOG"
`)
	writeInstallTestExecutable(t, filepath.Join(fakeBin, "mv"), `#!/bin/sh
if [ "${INSTALL_TEST_FAIL_RUNTIME_MOVE:-0}" -eq 1 ]; then
	case "$1:$2" in
		*/runtime:"$HOME/.local/libexec/hard") exit 1 ;;
	esac
fi
exec "$INSTALL_TEST_REAL_MV" "$@"
`)
	realMove, err := exec.LookPath("mv")
	if err != nil {
		t.Fatalf("find mv: %v", err)
	}

	path := fakeBin + string(os.PathListSeparator) + os.Getenv("PATH")
	if options.includeInstallBinInPath {
		path = fakeBin + string(os.PathListSeparator) +
			filepath.Join(home, ".local", "bin") + string(os.PathListSeparator) +
			os.Getenv("PATH")
	}
	var output strings.Builder
	var installError error
	for run := 0; run < options.repetitions; run++ {
		command := exec.Command("/bin/sh", append(
			[]string{installScriptPath(t)}, options.arguments...,
		)...)
		command.Env = append(os.Environ(),
			"HOME="+home,
			"INSTALL_TEST_ARCHIVE="+archive,
			"INSTALL_TEST_RELEASE_TAG="+options.releaseTag,
			"INSTALL_TEST_CHECKSUM="+checksum,
			"INSTALL_TEST_DOCKER_STATE="+dockerState,
			"INSTALL_TEST_FAIL_RUNTIME_MOVE="+fmt.Sprintf("%d", boolInstallTestValue(options.failRuntimeMove)),
			"INSTALL_TEST_PACKAGE_LOG="+packageLog,
			"INSTALL_TEST_REAL_MV="+realMove,
			"INSTALL_TEST_SERVICE_LOG="+serviceLog,
			"PATH="+path,
			"SHELL="+options.shell,
			"TMPDIR="+temporaryDirectory,
		)
		runOutput, err := command.CombinedOutput()
		output.Write(runOutput)
		if err != nil {
			installError = err
			break
		}
	}

	return installScriptResult{
		home:       home,
		output:     output.String(),
		packageLog: readInstallTestFile(t, packageLog),
		serviceLog: readInstallTestFile(t, serviceLog),
		err:        installError,
	}
}

func installTestShellConfig(home, shell string) string {
	switch filepath.Base(shell) {
	case "bash":
		return filepath.Join(home, ".bashrc")
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return filepath.Join(home, ".profile")
	}
}

func boolInstallTestValue(value bool) int {
	if value {
		return 1
	}
	return 0
}

func createInstallArchive(
	t *testing.T,
	directory string,
	badChecksum bool,
	releaseTag string,
) (string, string) {
	t.Helper()
	archiveRoot := filepath.Join(directory, "fixture", "hard-linux-amd64")
	for _, path := range []string{
		filepath.Join(archiveRoot, "bin"),
		filepath.Join(archiveRoot, "libexec", "hard", "bin"),
		filepath.Join(archiveRoot, "libexec", "hard", "format"),
		filepath.Join(archiveRoot, "libexec", "hard", "lib"),
		filepath.Join(archiveRoot, "share", "bash-completion", "completions"),
		filepath.Join(archiveRoot, "share", "zsh", "site-functions"),
		filepath.Join(archiveRoot, "share", "fish", "vendor_completions.d"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create archive directory: %v", err)
		}
	}
	for _, path := range []string{
		filepath.Join(archiveRoot, "bin", "hard"),
		filepath.Join(archiveRoot, "libexec", "hard", "hard"),
		filepath.Join(archiveRoot, "libexec", "hard", "bin", "clang-format"),
	} {
		writeInstallTestExecutable(t, path, "#!/bin/sh\nexit 0\n")
	}
	for path, contents := range map[string]string{
		filepath.Join(archiveRoot, "libexec", "hard", "hard.h"):                          "#pragma once\n",
		filepath.Join(archiveRoot, "libexec", "hard", "format", "format.v1"):             "BasedOnStyle: LLVM\n",
		filepath.Join(archiveRoot, "libexec", "hard", "lib", "libclang.so.18"):           "fixture\n",
		filepath.Join(archiveRoot, "share", "bash-completion", "completions", "hard"):    "# bash completion for hard\n",
		filepath.Join(archiveRoot, "share", "zsh", "site-functions", "_hard"):            "#compdef hard\n# zsh completion for hard\n",
		filepath.Join(archiveRoot, "share", "fish", "vendor_completions.d", "hard.fish"): "# fish completion for hard\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write archive file: %v", err)
		}
	}
	if err := os.Symlink(
		"libclang.so.18",
		filepath.Join(archiveRoot, "libexec", "hard", "lib", "libclang.so"),
	); err != nil {
		t.Fatalf("create libclang symlink: %v", err)
	}

	archive := filepath.Join(directory, "fixture.tar.gz")
	command := exec.Command(
		"tar",
		"-czf",
		archive,
		"-C",
		filepath.Dir(archiveRoot),
		filepath.Base(archiveRoot),
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create archive: %v, output = %q", err, output)
	}
	contents, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(contents))
	if badChecksum {
		digest = strings.Repeat("0", sha256.Size*2)
	}
	checksum := filepath.Join(directory, "fixture.sha256")
	if err := os.WriteFile(
		checksum,
		[]byte(digest+"  hard-"+releaseTag+".tar.gz\n"),
		0o644,
	); err != nil {
		t.Fatalf("write checksum: %v", err)
	}
	return archive, checksum
}

func writeInstallTestExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func readInstallTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func assertInstallFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %s = %v, want %v", path, got, want)
	}
}

func installScriptPath(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Join(filepath.Dir(workingDirectory), "install.sh")
}
