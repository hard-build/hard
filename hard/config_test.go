package main

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfigurationDefaults(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "user")
	root := filepath.Join(home, ".local", "share", "hard")

	got, err := loadConfigurationFrom(environment(nil), homeDirectory(home))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := configuration{
		root:        root,
		env:         defaultHardEnv,
		cc:          "c++",
		cflags:      defaultCFlags(root, defaultHardEnv),
		ldflags:     defaultLDFlags(),
		entrypoints: []string{"main", "_start"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadConfigurationFrom() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigurationOverrides(t *testing.T) {
	values := map[string]string{
		hardRootEnvironment:        "/opt/hard",
		hardEnvEnvironment:         "custom",
		hardCCEnvironment:          "clang++",
		hardCFlagsEnvironment:      `-std=c++23 '-DNAME=hello world'`,
		hardLDFlagsEnvironment:     `-fuse-ld=lld "-Wl,-rpath,/path with space"`,
		hardEntryPointsEnvironment: `main service_start`,
	}

	got, err := loadConfigurationFrom(environment(values), homeDirectory("unused"))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := configuration{
		root:        "/opt/hard",
		env:         "custom",
		cc:          "clang++",
		cflags:      []string{"-std=c++23", "-DNAME=hello world"},
		ldflags:     []string{"-fuse-ld=lld", "-Wl,-rpath,/path with space"},
		entrypoints: []string{"main", "service_start"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadConfigurationFrom() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigurationEmptyValues(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "user")
	root := filepath.Join(home, ".local", "share", "hard")
	values := map[string]string{
		hardRootEnvironment:        "",
		hardEnvEnvironment:         "",
		hardCCEnvironment:          "",
		hardCFlagsEnvironment:      "",
		hardLDFlagsEnvironment:     "",
		hardEntryPointsEnvironment: "",
	}

	got, err := loadConfigurationFrom(environment(values), homeDirectory(home))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := configuration{
		root:        root,
		env:         defaultHardEnv,
		cc:          "c++",
		cflags:      []string{},
		ldflags:     []string{},
		entrypoints: []string{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadConfigurationFrom() = %#v, want %#v", got, want)
	}
}

func TestDefaultCFlagsUseConfiguredRootAndEnvironment(t *testing.T) {
	values := map[string]string{
		hardRootEnvironment: "/srv/hard",
		hardEnvEnvironment:  "target",
	}

	got, err := loadConfigurationFrom(environment(values), homeDirectory("unused"))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := defaultCFlags("/srv/hard", "target")
	if !reflect.DeepEqual(got.cflags, want) {
		t.Fatalf("cflags = %#v, want %#v", got.cflags, want)
	}
}

func TestDefaultCFlagsUseSourceDirectory(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "srv", "hard")
	flags := defaultCFlags(root, "host")
	want := "-I" + filepath.Join(root, "source")

	if len(flags) < 6 {
		t.Fatalf("defaultCFlags() = %#v", flags)
	}
	if flags[5] != want {
		t.Fatalf("defaultCFlags()[5] = %q, want %q", flags[5], want)
	}
}

func TestFlagsDoNotExpandShellExpressions(t *testing.T) {
	values := map[string]string{
		hardCFlagsEnvironment: `-DHOME=$HOME -DUSER=` + "`whoami`",
	}

	got, err := loadConfigurationFrom(environment(values), homeDirectory("/home/user"))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := []string{"-DHOME=$HOME", "-DUSER=`whoami`"}
	if !reflect.DeepEqual(got.cflags, want) {
		t.Fatalf("cflags = %#v, want %#v", got.cflags, want)
	}
}

func TestEntryPointsDoNotExpandShellExpressions(t *testing.T) {
	values := map[string]string{
		hardEntryPointsEnvironment: `$ENTRY ` + "`whoami`",
	}

	got, err := loadConfigurationFrom(environment(values), homeDirectory("/home/user"))
	if err != nil {
		t.Fatalf("loadConfigurationFrom() error = %v", err)
	}

	want := []string{"$ENTRY", "`whoami`"}
	if !reflect.DeepEqual(got.entrypoints, want) {
		t.Fatalf("entrypoints = %#v, want %#v", got.entrypoints, want)
	}
}

func TestLoadConfigurationRejectsInvalidShellWords(t *testing.T) {
	tests := []string{hardCFlagsEnvironment, hardLDFlagsEnvironment, hardEntryPointsEnvironment}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			values := map[string]string{name: `"unterminated`}

			_, err := loadConfigurationFrom(environment(values), homeDirectory("/home/user"))
			if err == nil {
				t.Fatal("loadConfigurationFrom() error = nil")
			}
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("loadConfigurationFrom() error = %q, want it to contain %q", err, name)
			}
		})
	}
}

func TestLoadConfigurationReportsHomeDirectoryError(t *testing.T) {
	wantErr := errors.New("home directory unavailable")
	_, err := loadConfigurationFrom(environment(nil), func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("loadConfigurationFrom() error = %v, want %v", err, wantErr)
	}
}

func environment(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func homeDirectory(path string) func() (string, error) {
	return func() (string, error) {
		return path, nil
	}
}
