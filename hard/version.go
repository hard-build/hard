package main

import (
	"fmt"
	"io"
)

var versionNumber = "4.0"
var versionPrerelease = "development"

func hardVersion() string {
	version := "v" + versionNumber
	if versionPrerelease != "" {
		version += "-" + versionPrerelease
	}
	return version
}

func writeVersion(output io.Writer) error {
	if _, err := fmt.Fprintln(output, hardVersion()); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	return nil
}
