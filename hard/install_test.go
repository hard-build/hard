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

func TestInstallScriptModes(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		wantDefault     string
		wantPackages    []string
		unwantedPackage []string
		wantDocker      bool
	}{
		{
			name:            "host",
			mode:            "host",
			wantDefault:     "host\n",
			wantPackages:    []string{"g++", "libgtest-dev", "pkg-config"},
			unwantedPackage: []string{"docker.io"},
		},
		{
			name:            "docker",
			mode:            "docker",
			wantDefault:     "linux.v1\n",
			wantPackages:    []string{"docker.io"},
			unwantedPackage: []string{"g++", "libgtest-dev", "pkg-config"},
			wantDocker:      true,
		},
		{
			name:         "both",
			mode:         "both",
			wantDefault:  "linux.v1\n",
			wantPackages: []string{"g++", "libgtest-dev", "pkg-config", "docker.io"},
			wantDocker:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runInstallScript(t, tt.mode, false, false)
			if result.err != nil {
				t.Fatalf("install error = %v, output = %q", result.err, result.output)
			}

			defaultTarget, err := os.ReadFile(filepath.Join(
				result.home,
				".local",
				"libexec",
				"hard",
				"default-target",
			))
			if err != nil {
				t.Fatalf("read default target: %v", err)
			}
			if got := string(defaultTarget); got != tt.wantDefault {
				t.Fatalf("default target = %q, want %q", got, tt.wantDefault)
			}

			assertInstallFileMode(t, filepath.Join(result.home, ".local", "bin", "hard"), 0o755)
			assertInstallFileMode(
				t,
				filepath.Join(result.home, ".local", "libexec", "hard", "hard"),
				0o755,
			)
			assertInstallFileMode(
				t,
				filepath.Join(result.home, ".local", "libexec", "hard", "default-target"),
				0o644,
			)
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
			if info, err := os.Stat(filepath.Join(result.home, ".local", "share", "hard")); err != nil {
				t.Fatalf("stat data root: %v", err)
			} else if !info.IsDir() {
				t.Fatal("data root is not a directory")
			}

			packageFields := strings.Fields(result.packageLog)
			for _, want := range tt.wantPackages {
				if !containsInstallField(packageFields, want) {
					t.Errorf("package log = %q, want package %q", result.packageLog, want)
				}
			}
			for _, unwanted := range tt.unwantedPackage {
				if containsInstallField(packageFields, unwanted) {
					t.Errorf("package log = %q, unwanted package %q", result.packageLog, unwanted)
				}
			}
			for _, optional := range []string{
				"make",
				"cmake",
				"meson",
				"ninja",
				"autoconf",
				"automake",
				"libtool",
			} {
				if containsInstallField(packageFields, optional) {
					t.Errorf("package log = %q, optional package %q must not be installed", result.packageLog, optional)
				}
			}
			if tt.wantDocker && !strings.Contains(result.serviceLog, "enable --now docker") {
				t.Errorf("service log = %q, want Docker service start", result.serviceLog)
			}
			if !tt.wantDocker && result.serviceLog != "" {
				t.Errorf("service log = %q, want no Docker service changes", result.serviceLog)
			}
		})
	}
}

func TestInstallScriptStopsOnChecksumMismatch(t *testing.T) {
	result := runInstallScript(t, "host", true, false)
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

func TestInstallScriptRestoresPreviousRuntime(t *testing.T) {
	result := runInstallScript(t, "host", false, true)
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

type installScriptResult struct {
	home       string
	output     string
	packageLog string
	serviceLog string
	err        error
}

func runInstallScript(t *testing.T, mode string, badChecksum, failRuntimeMove bool) installScriptResult {
	t.Helper()
	temporaryDirectory := t.TempDir()
	home := filepath.Join(temporaryDirectory, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatalf("create home: %v", err)
	}
	if failRuntimeMove {
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
	archive, checksum := createInstallArchive(t, temporaryDirectory, badChecksum)
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
		https://*)
			url=$1
			shift
			;;
		*) shift ;;
	esac
done
case "$url" in
	*.sha256) cp "$INSTALL_TEST_CHECKSUM" "$output" ;;
	*) cp "$INSTALL_TEST_ARCHIVE" "$output" ;;
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

	command := exec.Command("/bin/sh", installScriptPath(t), mode)
	command.Env = append(os.Environ(),
		"HOME="+home,
		"INSTALL_TEST_ARCHIVE="+archive,
		"INSTALL_TEST_CHECKSUM="+checksum,
		"INSTALL_TEST_DOCKER_STATE="+dockerState,
		"INSTALL_TEST_FAIL_RUNTIME_MOVE="+fmt.Sprintf("%d", boolInstallTestValue(failRuntimeMove)),
		"INSTALL_TEST_PACKAGE_LOG="+packageLog,
		"INSTALL_TEST_REAL_MV="+realMove,
		"INSTALL_TEST_SERVICE_LOG="+serviceLog,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+temporaryDirectory,
	)
	output, err := command.CombinedOutput()

	return installScriptResult{
		home:       home,
		output:     string(output),
		packageLog: readInstallTestFile(t, packageLog),
		serviceLog: readInstallTestFile(t, serviceLog),
		err:        err,
	}
}

func boolInstallTestValue(value bool) int {
	if value {
		return 1
	}
	return 0
}

func createInstallArchive(t *testing.T, directory string, badChecksum bool) (string, string) {
	t.Helper()
	archiveRoot := filepath.Join(directory, "fixture", "hard-linux-amd64")
	for _, path := range []string{
		filepath.Join(archiveRoot, "bin"),
		filepath.Join(archiveRoot, "libexec", "hard", "bin"),
		filepath.Join(archiveRoot, "libexec", "hard", "format"),
		filepath.Join(archiveRoot, "libexec", "hard", "lib"),
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
		filepath.Join(archiveRoot, "libexec", "hard", "hard.h"):                "#pragma once\n",
		filepath.Join(archiveRoot, "libexec", "hard", "format", "format.v1"):   "BasedOnStyle: LLVM\n",
		filepath.Join(archiveRoot, "libexec", "hard", "lib", "libclang.so.18"): "fixture\n",
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
		[]byte(digest+"  hard-linux-amd64.tar.gz\n"),
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

func containsInstallField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func installScriptPath(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Join(filepath.Dir(workingDirectory), "install.sh")
}
