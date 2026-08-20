package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressBarUpdatesOneLineInNormalMode(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, 2, false, false, true)

	progress.complete("first.cpp", nil)
	progress.complete("directory/second.cpp", nil)
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}

	want := "\r[1/2] first.cpp\r[2/2] directory/second.cpp\n"
	if got := output.String(); got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressBarCompletesMultipleMessagesAsOneStep(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, -1, false, false, true)
	first := "Downloading github.com/owner/repository"
	second := "Downloading github.com/owner/other"
	compile := "Compiling source.cpp"

	progress.updateStep(first)
	progress.updateStep(second)
	progress.setTotal(2)
	progress.complete(compile, nil)
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}

	firstLine := "[1/?] " + first
	secondLine := "[1/?] " + second
	compileLine := "[2/2] " + compile
	want := "\r" + firstLine + "\r" + secondLine +
		strings.Repeat(" ", len(firstLine)-len(secondLine)) +
		"\r" + compileLine +
		strings.Repeat(" ", len(secondLine)-len(compileLine)) + "\n"
	if got := output.String(); got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressBarWritesVerboseMessagesForOneStep(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, -1, true, false, true)

	progress.updateStep("Downloading github.com/owner/first")
	progress.updateStep("Downloading github.com/owner/second")
	progress.setTotal(2)
	progress.complete("Compiling source.cpp", nil)
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}

	want := strings.Join([]string{
		"[1/?] Downloading github.com/owner/first",
		"[1/?] Downloading github.com/owner/second",
		"[2/2] Compiling source.cpp",
		"",
	}, "\n")
	if got := output.String(); got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressBarWritesVerboseEntriesAndDiff(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, 2, true, false, true)

	progress.complete("first.cpp", nil)
	progress.complete("second.cpp", []byte("--- second.cpp\n+++ second.cpp\n-old\n+new\n"))
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}

	want := strings.Join([]string{
		"[1/2] first.cpp",
		"[2/2] second.cpp",
		"--- second.cpp",
		"+++ second.cpp",
		"-old",
		"+new",
		"",
	}, "\n")
	if got := output.String(); got != want {
		t.Fatalf("progress output = %q, want %q", got, want)
	}
}

func TestProgressBarSilentModeWritesNothing(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, 1, true, true, false)

	progress.updateStep("Downloading github.com/owner/repository")
	progress.complete("source.cpp", []byte("diff\n"))
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("progress output = %q, want empty output", output.String())
	}
}

func TestProgressBarUsesColor(t *testing.T) {
	var output bytes.Buffer
	progress := newProgressBar(&output, 1, true, false, false)

	progress.complete("source.cpp", nil)
	if err := progress.finish(); err != nil {
		t.Fatalf("progress.finish() error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "\x1b[") {
		t.Fatalf("progress output has no color: %q", got)
	}
}
