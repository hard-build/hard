package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
)

const (
	hardRootEnvironment        = "HARD_ROOT"
	hardEnvEnvironment         = "HARD_ENV"
	hardCCEnvironment          = "HARD_CC"
	hardCFlagsEnvironment      = "HARD_CFLAGS"
	hardLDFlagsEnvironment     = "HARD_LDFLAGS"
	hardEntryPointsEnvironment = "HARD_ENTRYPOINTS"
	hardExecutableSuffix       = "HARD_EXECUTABLE_SUFFIX"
	hardExecutableRunner       = "HARD_EXECUTABLE_RUNNER"

	defaultHardEnv = "host"
	defaultHardCC  = "c++"
)

type configuration struct {
	root             string
	runtimeRoot      string
	env              string
	cc               string
	cflags           []string
	ldflags          []string
	entrypoints      []string
	executableSuffix string
	executableRunner string
}

func loadConfiguration(runtimeRoot string) (configuration, error) {
	return loadConfigurationFrom(runtimeRoot, os.LookupEnv, os.UserHomeDir)
}

func loadConfigurationFrom(
	runtimeRoot string,
	lookupEnv func(string) (string, bool),
	userHomeDir func() (string, error),
) (configuration, error) {
	root, ok := lookupEnv(hardRootEnvironment)
	if !ok || root == "" {
		home, err := userHomeDir()
		if err != nil {
			return configuration{}, fmt.Errorf("%s: determine default: %w", hardRootEnvironment, err)
		}
		root = filepath.Join(home, ".local", "share", "hard")
	}

	environment, ok := lookupEnv(hardEnvEnvironment)
	if !ok || environment == "" {
		environment = defaultHardEnv
	}

	cc, ok := lookupEnv(hardCCEnvironment)
	if !ok || cc == "" {
		cc = defaultHardCC
	}

	cflags, err := flagsFromEnvironment(
		lookupEnv,
		hardCFlagsEnvironment,
		defaultCFlags(),
	)
	if err != nil {
		return configuration{}, err
	}

	ldflags, err := flagsFromEnvironment(
		lookupEnv,
		hardLDFlagsEnvironment,
		defaultLDFlags(),
	)
	if err != nil {
		return configuration{}, err
	}
	entrypoints, err := entryPointsFromEnvironment(lookupEnv)
	if err != nil {
		return configuration{}, err
	}
	executableSuffix, _ := lookupEnv(hardExecutableSuffix)
	if err := validateExecutableSuffix(executableSuffix); err != nil {
		return configuration{}, err
	}
	executableRunner, _ := lookupEnv(hardExecutableRunner)

	return configuration{
		root:             root,
		runtimeRoot:      runtimeRoot,
		env:              environment,
		cc:               cc,
		cflags:           cflags,
		ldflags:          ldflags,
		entrypoints:      entrypoints,
		executableSuffix: executableSuffix,
		executableRunner: executableRunner,
	}, nil
}

func validateExecutableSuffix(suffix string) error {
	if suffix == "" {
		return nil
	}
	if !strings.HasPrefix(suffix, ".") {
		return fmt.Errorf("%s must be empty or start with '.': %s", hardExecutableSuffix, suffix)
	}
	if strings.ContainsAny(suffix, `/\\`) {
		return fmt.Errorf("%s must not contain a path separator: %s", hardExecutableSuffix, suffix)
	}
	return nil
}

func entryPointsFromEnvironment(lookupEnv func(string) (string, bool)) ([]string, error) {
	value, ok := lookupEnv(hardEntryPointsEnvironment)
	if !ok {
		return []string{"main", "_start"}, nil
	}
	if value == "" {
		return []string{}, nil
	}

	parser := shellwords.NewParser()
	parser.ParseEnv = false
	parser.ParseBacktick = false
	entrypoints, err := parser.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("%s: parse entry points: %w", hardEntryPointsEnvironment, err)
	}
	if len(entrypoints) == 0 {
		return []string{}, nil
	}
	return entrypoints, nil
}

func flagsFromEnvironment(
	lookupEnv func(string) (string, bool),
	name string,
	defaults []string,
) ([]string, error) {
	value, ok := lookupEnv(name)
	if !ok {
		return defaults, nil
	}
	if value == "" {
		return []string{}, nil
	}

	parser := shellwords.NewParser()
	parser.ParseEnv = false
	parser.ParseBacktick = false
	flags, err := parser.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("%s: parse flags: %w", name, err)
	}
	if len(flags) == 0 {
		return []string{}, nil
	}
	return flags, nil
}

func defaultCFlags() []string {
	return []string{
		"-std=c++20",
		"-O3",
		"-flto=auto",
		"-Wall",
		"-Wextra",
	}
}

func effectiveCFlags(cflags []string, root, runtimeRoot string) []string {
	flags := append([]string(nil), cflags...)
	return append(
		flags,
		"-I"+filepath.Join(root, "source"),
		"-include",
		filepath.Join(runtimeRoot, "hard.h"),
	)
}

func defaultLDFlags() []string {
	return []string{
		"-std=c++20",
		"-O3",
		"-flto=auto",
		"-Wall",
		"-Wextra",
		"-static-libgcc",
		"-static-libstdc++",
	}
}
