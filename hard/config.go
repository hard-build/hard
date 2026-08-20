package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattn/go-shellwords"
)

const (
	hardRootEnvironment        = "HARD_ROOT"
	hardEnvEnvironment         = "HARD_ENV"
	hardCCEnvironment          = "HARD_CC"
	hardCFlagsEnvironment      = "HARD_CFLAGS"
	hardLDFlagsEnvironment     = "HARD_LDFLAGS"
	hardEntryPointsEnvironment = "HARD_ENTRYPOINTS"

	defaultHardEnv = "host"
	defaultHardCC  = "c++"
)

type configuration struct {
	root        string
	env         string
	cc          string
	cflags      []string
	ldflags     []string
	entrypoints []string
}

func loadConfiguration() (configuration, error) {
	return loadConfigurationFrom(os.LookupEnv, os.UserHomeDir)
}

func loadConfigurationFrom(
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
		defaultCFlags(root, environment),
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

	return configuration{
		root:        root,
		env:         environment,
		cc:          cc,
		cflags:      cflags,
		ldflags:     ldflags,
		entrypoints: entrypoints,
	}, nil
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

func defaultCFlags(root, environment string) []string {
	return []string{
		"-std=c++20",
		"-O3",
		"-flto=auto",
		"-Wall",
		"-Wextra",
		"-I" + filepath.Join(root, "source"),
		"-include",
		filepath.Join(root, "env", environment, "hard.h"),
	}
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
