package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsWineLauncher(t *testing.T) {
	launcher, err := os.ReadFile("../target/windows64/wine.sh")
	if err != nil {
		t.Fatal(err)
	}
	const wine = "/usr/lib/wine/wine64"
	if strings.Count(string(launcher), wine) != 1 {
		t.Fatal("Wine launcher does not exec the expected image binary")
	}
	for _, mode := range []string{"missing parent", "existing prefix", "empty prefix", "unset prefix", "invalid parent"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			fakeWine := filepath.Join(directory, "fake wine")
			if err := os.WriteFile(fakeWine, []byte("#!/bin/sh\nprintf '<%s>\\n' \"$@\"\nexit 7\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			prefix := filepath.Join(directory, "project", "test environment", "@runtime", "wine")
			if mode == "existing prefix" || mode == "invalid parent" {
				parent := prefix
				if mode == "invalid parent" {
					parent = filepath.Dir(filepath.Dir(prefix))
				}
				if err := os.MkdirAll(parent, 0o755); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(prefix, "marker")
				if mode == "invalid parent" {
					marker = filepath.Dir(prefix)
				}
				if err := os.WriteFile(marker, []byte("preserve"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.Replace(string(launcher), wine, quoteShellArgument(fakeWine), 1)
			command := exec.Command("sh", "-c", script, "wine", "binary with spaces.exe", "", "*")
			command.Dir = directory
			command.Env = []string{"PATH=" + os.Getenv("PATH")}
			if mode == "empty prefix" {
				command.Env = append(command.Env, "WINEPREFIX=")
			} else if mode != "unset prefix" {
				command.Env = append(command.Env, "WINEPREFIX="+prefix)
			}
			output, err := command.CombinedOutput()
			if mode == "invalid parent" {
				if err == nil || strings.Contains(string(output), "<binary with spaces.exe>") {
					t.Fatalf("Wine ran despite parent creation failure: %v\n%s", err, output)
				}
				if readTestFile(t, filepath.Dir(prefix)) != "preserve" {
					t.Fatal("invalid parent was overwritten")
				}
				return
			}
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 7 || string(output) != "<binary with spaces.exe>\n<>\n<*>\n" {
				t.Fatalf("arguments or exit status were changed: %v\n%s", err, output)
			}
			switch mode {
			case "missing parent":
				if info, err := os.Stat(filepath.Dir(prefix)); err != nil || !info.IsDir() {
					t.Fatalf("prefix parent was not created: %v", err)
				}
				if _, err := os.Lstat(prefix); !os.IsNotExist(err) {
					t.Fatalf("launcher initialized the prefix itself: %v", err)
				}
			case "existing prefix":
				if readTestFile(t, filepath.Join(prefix, "marker")) != "preserve" {
					t.Fatal("existing prefix was overwritten")
				}
			default:
				if _, err := os.Lstat(filepath.Join(directory, "project")); !os.IsNotExist(err) {
					t.Fatalf("created state without WINEPREFIX: %v", err)
				}
			}
		})
	}
}
