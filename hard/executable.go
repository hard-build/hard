package main

import (
	"strings"
)

func appendExecutableSuffix(path, suffix string) string {
	if suffix == "" || strings.HasSuffix(path, suffix) {
		return path
	}
	return path + suffix
}

func programInvocation(runner, binary string, arguments []string) (string, []string) {
	if runner != "" {
		return runner, append([]string{binary}, arguments...)
	}
	return binary, append([]string(nil), arguments...)
}
