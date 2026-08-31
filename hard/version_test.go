package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHardVersion(t *testing.T) {
	originalNumber := versionNumber
	originalPrerelease := versionPrerelease
	t.Cleanup(func() {
		versionNumber = originalNumber
		versionPrerelease = originalPrerelease
	})

	tests := []struct {
		name       string
		number     string
		prerelease string
		want       string
	}{
		{name: "development", number: "4.0", prerelease: "development", want: "v4.0-development"},
		{name: "release", number: "4.0", want: "v4.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionNumber = tt.number
			versionPrerelease = tt.prerelease
			if got := hardVersion(); got != tt.want {
				t.Fatalf("hardVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteVersion(t *testing.T) {
	var output bytes.Buffer
	if err := writeVersion(&output); err != nil {
		t.Fatalf("writeVersion() error = %v", err)
	}
	if want := hardVersion() + "\n"; output.String() != want {
		t.Fatalf("writeVersion() = %q, want %q", output.String(), want)
	}
}

func TestWriteVersionReturnsOutputError(t *testing.T) {
	err := writeVersion(versionErrorWriter{})
	if err == nil || !strings.Contains(err.Error(), "write version") {
		t.Fatalf("writeVersion() error = %v, want output error", err)
	}
}

type versionErrorWriter struct{}

func (versionErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}
