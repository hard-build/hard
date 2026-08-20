# hard project memory

Last updated: 2026-08-21.

This document is a self-contained memory snapshot for the current Go
implementation of `hard`. It records the product intent, confirmed
requirements, implemented behavior, architecture, operational constraints,
tests, known gaps, and the history needed to continue development without the
original conversation.

The repository root is `/home/taitov/projects/hard-build/hard`. The Go module
and implementation live in its `hard/` directory. Root-level documentation,
formatting assets, the environment support header, and container assets belong
to the same current project. The former implementation was deleted; there is
no legacy generation in this repository to extend or preserve.

## How to use and maintain this memory

- Read this file and [AGENTS.md](AGENTS.md) completely before changing the
  project.
- Keep this file synchronized whenever a requirement, accepted design choice,
  implementation state, dependency, test, known limitation, or verification
  procedure changes.
- `AGENTS.md` is the concise repository rule document. `MEMORY.md` is the
  broader self-contained project-memory snapshot. They must not contradict one
  another.
- [README.md](README.md) is English user-facing documentation for the current
  system. It explicitly identifies known future work; use tests and code to
  confirm implementation status.
- When sources disagree, use this precedence:
  1. the newest explicit user instruction;
  2. repository rules in `AGENTS.md`;
  3. current tests and implementation in `hard/`;
  4. this memory snapshot;
  5. user-facing behavior in `README.md`.
- Build verification executables under a unique `/tmp` path. A default
  `go build` from the module directory would create `hard/hard`; do not create
  or overwrite that path during routine verification.

## User-requested working protocol

The project has an explicit collaboration protocol. Preserve it in future
work:

- Read every file completely before editing it. Do not patch from assumptions
  about unseen content.
- If a task needs more than one edit, first provide a plan naming the files,
  changes, and checks, then wait for confirmation. Do not start coding in the
  plan response.
- Ask when a requirement has two materially different interpretations. For a
  cosmetic difference, choose one and state the choice.
- Verify the existence and current version of files, functions, flags, tools,
  and dependencies before relying on them.
- Make only the requested change. Do not refactor, rename, reformat unrelated
  lines, or add speculative abstractions.
- Report an unrelated defect but do not fix it without a request.
- External or irreversible actions such as push, deploy, delete, overwrite,
  branch mutation, or sending data elsewhere require explicit approval each
  time.
- Inspect a target before deleting or overwriting it.
- Completion requires proportionate tests, static checks, and a build. State
  clearly when something could not be checked.
- Preserve actual failing output rather than replacing it with a paraphrase.
- Before reporting completion, reread the complete diff skeptically and check
  compilation, edge cases, and the accuracy of documentation claims.
- Final reports contain the result, checks, and remaining work. Do not inflate
  confidence beyond the evidence.
- Follow the naming, comment density, and idioms of the surrounding code.

## Project identity

`hard` is a convention-based build tool for C and C++ source trees. The goal is
to derive formatting, dependencies, compilation, linking, and tests from the
selected sources and their include graph rather than from a hand-written
project build file.

The public operations are deliberately limited to:

    hard format
    hard fetch
    hard build
    hard test

The repository contains one current generation. Root-level files provide
documentation and shared assets. The `hard/` subdirectory contains the Go
module and all implementation and test sources. The previous version was
deleted after the Go rewrite became the current implementation.

The license is MIT. [LICENSE](LICENSE) contains:

    Copyright (c) 2026 Timur Aitov <timonbl4@gmail.com>

All Go sources currently use package `main`; the module is named `hard`.

## Current implementation status

Implemented:

- Cobra-based argument parsing;
- environment-backed configuration;
- command-specific source discovery;
- recursive traversal through directory symlinks with cycle prevention;
- relative source display paths and canonical deduplication;
- common colored, verbose, normal, and silent progress output;
- parallel formatting with internal unified diffs;
- unified libclang 18 dependency, declaration, and entry-point analysis;
- recursive GitHub snapshot fetching and persistent caching;
- the well-known `hard/` repository mapping;
- forward-declaration generation for project and managed external headers;
- object compilation, dependency-object resolution, ordinary executable
  linking, and atomic delivery;
- GoogleTest discovery, compilation, linking, parallel execution, output
  grouping, and failure aggregation;
- the standalone `fetch` command;
- a POSIX command wrapper and Make-based user-local build and installation;
- exclusion of the environment support `hard.h` from standalone forward
  generation while retaining its configured force include.

Not implemented:

- incremental timestamp or content-based rebuild decisions;
- cache validation or automatic refresh of external snapshots;
- stale generated-artifact cleanup.

Selected objects are rebuilt on every invocation. Existing external repository
directories are treated as persistent cache entries and are not refreshed.

## Current file map

| File | Responsibility |
| --- | --- |
| `AGENTS.md` | Concise repository working rules and required checks |
| `MEMORY.md` | This self-contained project-memory snapshot |
| `README.md` | English user-facing description of the current system |
| `LICENSE` | MIT license |
| `Makefile` | Builds the Go backend and installs the wrapper, backend, support header, and default format |
| `hard.sh` | Installed public-command wrapper; replaces itself with the relative backend and forwards arguments unchanged |
| `hard.h` | Source environment support header; installations place or link it at `HARD_ROOT/env/HARD_ENV/hard.h` |
| `format/format.v1` | Default clang-format style |
| `environment/ubuntu2204.v1.dockerfile` | Ubuntu 22.04 environment image definition |
| `hard/go.mod`, `hard/go.sum` | Module identity, Go version, dependencies, and checksums |
| `hard/main.go` | Process entry, dispatch, configuration loading, and shared search progress |
| `hard/main_test.go` | Search-progress behavior and discovery integration for all commands |
| `hard/cli.go` | Cobra command tree, flags, positional paths, and job normalization |
| `hard/cli_test.go` | CLI defaults, validation, help, interspersed flags, and job forms |
| `hard/config.go` | `HARD_*` configuration and default compiler/linker flag vectors |
| `hard/config_test.go` | Configuration defaults, overrides, parsing, and failures |
| `hard/source.go` | File classification, recursive discovery, symlink traversal, and deduplication |
| `hard/source_test.go` | Extensions, ordering, explicit paths, symlinks, cycles, and failures |
| `hard/progress.go` | Thread-safe normal, verbose, silent, color, and live-step output |
| `hard/progress_test.go` | Progress rendering, details, colors, padding, and unknown totals |
| `hard/format.go` | Style validation, parallel clang-format execution, and unified diffs |
| `hard/format_test.go` | Style containment, formatting, diff, output, and parallelism tests |
| `hard/clang.go` | Go-facing libclang analysis, arguments, normalization, diagnostics, and retry logic |
| `hard/clang_bridge.h`, `hard/clang_bridge.cc` | C ABI and C++ libclang 18 bridge |
| `hard/clang_test.go` | Includes, macros, conditional behavior, declarations, templates, and functions |
| `hard/github.go` | Repository mapping, synchronized downloads, extraction, aliases, and cache |
| `hard/github_test.go` | HTTP, extraction safety, aliases, caching, retries, and concurrency |
| `hard/forward.go` | Forward extraction, validation, rendering, path mapping, and atomic writes |
| `hard/forward_test.go` | Namespace/template output, filtering, paths, collisions, and preservation |
| `hard/entry.go` | Configured global entry-function definition detection |
| `hard/entry_test.go` | Definitions, declarations, namespaces, macros, ambiguity, and empty config |
| `hard/build.go` | Dependency closure, support-header exclusion, compilation, link graph, and delivery |
| `hard/build_test.go` | Dependency, forwards, objects, entry binaries, output, and integrations |
| `hard/fetch.go` | Dependency-only source closure and external snapshot fetching |
| `hard/fetch_test.go` | Fetch progress, recursive repositories, caching, and absence of build artifacts |
| `hard/test.go` | GoogleTest flags, plans, forward generation, compilation, linking, and execution |
| `hard/test_test.go` | Tool discovery, parallel phases, output modes, failures, and artifacts |

## Toolchain and dependencies

`hard/go.mod` currently declares:

    module hard
    go 1.23

Direct Go dependencies:

- `github.com/mattn/go-shellwords v1.0.14`;
- `github.com/pmezard/go-difflib v1.0.0`;
- `github.com/spf13/cobra v1.10.2`.

Indirect Go dependencies:

- `github.com/inconshreveable/mousetrap v1.1.0`;
- `github.com/spf13/pflag v1.0.9`.

Do not change these versions or Go 1.23 without a requirement that needs the
change.

The parser is a CGO integration with a C++ bridge pinned on Linux to:

    /usr/lib/llvm-18/include
    libclang-18

Building `hard` requires Go 1.23, CGO, a C++20 host toolchain, and the LLVM 18
development headers and library. Running the resulting binary needs the
libclang 18 shared library.

Runtime tools by command:

- `clang-format` for `format`;
- libclang 18 for dependency analysis in `fetch`, `build`, and `test`, forward
  extraction in `build` and `test`, and entry detection in `build`;
- `HARD_CC` for object compilation and compiler-driver linking in `build` and
  `test`;
- `pkg-config` and `gtest_main` for `test`;
- HTTPS access to GitHub when a required repository is not cached.

## Build and installation

The repository-root Makefile has exactly these public targets:

- `all`, the default target, depends on `build`;
- `build` compiles the Go module in `hard/` to `BUILD_DIR/hard`;
- `install` builds and installs the runtime files.

The installation variables and defaults are:

    GO=go
    INSTALL=install
    PREFIX=$HOME/.local
    DESTDIR=
    BUILD_DIR=build

The default logical installation is:

```text
$HOME/.local/
├── bin/hard                         mode 0755, installed from hard.sh
├── libexec/hard/hard                mode 0755, Go backend
└── share/hard/
    ├── env/host/hard.h              mode 0644
    └── format/format.v1             mode 0644
```

`hard.sh` is a POSIX `sh` wrapper. It resolves the backend relative to its own
installed location, uses `exec`, and passes the original argument vector with
`"$@"`. It does not set or rewrite `HARD_ROOT` or any other environment
variable. Consequently, the default `PREFIX` agrees with the backend's default
`HARD_ROOT=$HOME/.local/share/hard`; a custom `PREFIX` requires the caller to
set `HARD_ROOT=<PREFIX>/share/hard`.

`DESTDIR` prepends a staging root to every installed path but does not become
part of the logical prefix. This permits packaging tests without writing into
the user's home or a system directory. There are intentionally no `clean` or
`uninstall` targets in the current scope.

## Exact public CLI

The public command forms are:

    hard format [--format=<name>] [-s|--silent] [path...]
    hard build  [-s|--silent] [-o <path>] [path...]
    hard fetch  [-s|--silent] [path...]
    hard test   [-s|--silent] [path...]

Persistent flags accepted by every command:

- `-v`, `--verbose`: permanent detailed progress and command-specific details;
- `--no-color`: disable hard's ANSI colors;
- `-jN`, `-j N`, `--jobs=N`: use exactly `N` jobs;
- bare `-j`, bare `--jobs`, `-j0`, or `--jobs=0`: use `runtime.NumCPU()` jobs.

The default job count is one. Negative values are errors. Bare job flags are
normalized to `--jobs=0` before Cobra parsing. Flags may appear before the
command, after the command, or between positional paths because interspersed
flag handling is enabled.

Command-local flags:

- `format`: `--format=<name>`, default `format.v1`;
- every public command: `-s`, `--silent`;
- `build`: `-o <path>`, `--output=<path>`;
- `fetch` and `test` do not accept `--format` or `--output`.

Other CLI decisions:

- each command accepts zero or more paths;
- no path becomes `.`;
- invoking no command is an error: `a command is required`;
- unknown commands and flags are errors;
- Cobra completion is disabled;
- the normal `help` command is not public; `help` and `_help` are rejected;
- root and command `--help` succeed without configuration loading or source
  discovery;
- root help exposes only `format`, `fetch`, `build`, and `test`.

The parsed `arguments` value contains `command`, `paths`, `verbose`, `silent`,
`noColor`, `jobs`, `format`, and `output`. Only `build` populates `output`. Its
raw spelling preserves a trailing path separator because that separator
declares directory intent.

## Configuration contract

Configuration is loaded after successful CLI parsing and before source
discovery. It is held in an unexported configuration value rather than mutable
package globals.

Environment variable names are ASCII:

- `HARD_ROOT`;
- `HARD_ENV`;
- `HARD_CC`;
- `HARD_CFLAGS`;
- `HARD_LDFLAGS`;
- `HARD_ENTRYPOINTS`.

`HARD_CC` contains two ASCII `C` characters. Avoid visually similar Cyrillic
characters.

### `HARD_ROOT`

- unset or empty: `<user-home>/.local/share/hard`;
- non-empty: retained exactly;
- failure to determine the home directory is reported as a `HARD_ROOT` default
  failure.

### `HARD_ENV`

- unset or empty: `host`;
- non-empty: retained exactly;
- artifact path construction rejects values escaping `HARD_ROOT/env`, such as
  `../outside`.

### `HARD_CC`

- unset or empty: `c++`;
- non-empty: one executable name or path;
- used only for object compilation and linking, not dependency discovery or
  `fetch`.

### `HARD_CFLAGS`

The default vector is:

    -std=c++20
    -O3
    -flto=auto
    -Wall
    -Wextra
    -I<HARD_ROOT>/source
    -include
    <HARD_ROOT>/env/<HARD_ENV>/hard.h

If `HARD_CFLAGS` is present but empty, no compiler flags are supplied. A
non-empty value is parsed with `go-shellwords`. Quoting works, but environment
expansion and backtick execution are disabled. Malformed quoting names
`HARD_CFLAGS` in the error.

An explicit value replaces the complete default vector. It must provide
`-I<HARD_ROOT>/source` when external cached and well-known headers should remain
reachable, and it must provide the support-header force include if desired.

The same vector is used by libclang dependency, forward, and entry analyses
and by `HARD_CC` object compilation. Dependency and entry analysis add the
working directory and default to C++ mode unless `-x` is already present.
Header analysis uses `c++-header`.

Build and test canonicalize `HARD_ROOT/env/HARD_ENV/hard.h` through symlinks
after dependency closure discovery and remove only that canonical target from
forward-managed dependency lists. The original `HARD_CFLAGS -include` remains.
Consequences:

- libclang and the compiler still see the environment support header;
- it has no later standalone `Parsing` step;
- no support `_fwd` file is generated;
- no generated support forward is appended with another `-include`;
- unrelated project or external headers also named `hard.h` remain managed;
- an old support forward artifact is not deleted automatically;
- failure to resolve the environment support path adds no new filter-specific
  error; the normal parser/compiler handling of the flags remains authoritative.

### `HARD_LDFLAGS`

The default vector is:

    -std=c++20
    -O3
    -flto=auto
    -Wall
    -Wextra
    -static-libgcc
    -static-libstdc++

Present-empty and shell-word rules match `HARD_CFLAGS`. Linker flags follow
object paths in every compiler-driver link command.

### `HARD_ENTRYPOINTS`

- unset: `main _start`;
- present but empty: no entry names, disabling build linking while retaining
  dependency analysis, forward generation, and object compilation;
- non-empty: parsed with the same shell-word and disabled-expansion rules;
- only matching global function definitions make an entry source;
- `test` ignores this variable because `gtest_main` supplies its entry point.

## Source selection

Recognized files:

| Command | Files |
| --- | --- |
| `build` | `.c`, `.cc`, `.cpp`, `.c++`, excluding test sources |
| `fetch` | `.c`, `.cc`, `.cpp`, `.c++`, including test sources |
| `format` | build extensions plus `.h`, `.hh`, `.hpp`, `.h++` |
| `test` | test sources using `.c`, `.cc`, `.cpp`, or `.c++` |

Extensions are case-insensitive. A test source has a stem ending in `_test`,
also case-insensitive. `source_TeSt.CPP` is therefore a test, excluded by
`build` and included by `fetch`, `format`, and `test`.

Not recognized unless a future requirement changes the set:

    .cxx .hxx .cu .cuh .inl .ipp .tpp

Path rules:

- with no path, recursively scan `.`;
- with paths, root selection includes only explicit eligible files and
  eligible files recursively found under explicit directories;
- build, fetch, and test may subsequently add same-stem production sources
  required by dependencies; fetch only analyzes them;
- unsupported explicit files are silently skipped;
- missing or inaccessible inputs are errors;
- no matches is a successful no-op;
- displayed selected paths retain the first lexical spelling and are relative
  to the process working directory;
- input roots keep argument order, directory entries use `os.ReadDir` lexical
  order, and traversal is depth-first;
- files are deduplicated by `EvalSymlinks` plus `Abs`;
- symlinks to eligible regular files are accepted according to the link name's
  extension;
- directory symlinks are followed recursively as normal directories, even if
  they leave the lexical input subtree;
- each resolved directory is visited once, preventing cycles;
- the first route to a resolved directory determines displayed spellings.

## Shared progress and output

Every public command creates one thread-safe progress object before source
selection and starts with `Searching source files`.

Modes:

- normal: repeatedly rewrite one terminal line as `\r[N/M] description`, pad
  shorter replacements, and finish with one newline;
- verbose: write permanent `[N/M] description` lines and attach details
  atomically after the relevant completed entry;
- silent: write no successful progress; errors remain available, and silent
  overrides verbose;
- color: counters are green and paths cyan unless `--no-color` is set.

Parallel completion entries are serialized, so a progress entry and its diff,
compiler command, linker command, or test-output block never interleave with
another entry. Work completion order may differ from discovery order.

`updateStep` creates step one on its first call and later updates its text
without advancing the counter. A negative total is displayed as `?`.

- `format`: search is step one; total is `1 + selected files`; formatting
  begins at `[2/M]`.
- `build`: search, all source/header parsing, and all repository downloads
  reuse preparation step one; total becomes `1 + sources + 2 * root entry
  binaries`; compilation begins at `[2/M]`.
- `fetch`: search, parsing, and downloads form its single preparation step;
  live output stays `[1/?]` and final total is one.
- `test`: search, parsing, and downloads reuse preparation step one; total is
  `1 + unique compilations + 2 * prepared test executables`; compilation
  begins at `[2/M]`.

Progress labels:

- format: the selected file;
- build: `Searching source files`, `Parsing ...`, `Downloading ...`,
  `Compiling ...`, `Linking ...`, `Copying ...`;
- fetch: `Searching source files`, `Parsing ...`, `Downloading ...`;
- test: `Searching source files`, `Parsing ...`, `Downloading ...`,
  `Compiling ...`, `Linking ...`, `Testing ...`.

Paths below canonical `HARD_ROOT/source/github.com` are displayed relative to
`HARD_ROOT/source`, for example
`github.com/hard-build/library/application/application.cpp`. Well-known aliases
use the same canonical display. Only the progress label changes; actual source
arguments, diagnostics, errors, object paths, and verbose commands keep their
real paths.

Errors and diagnostics use stderr. Top-level errors are rendered as:

    hard: <error>

and cause status 1.

## `hard format`

Format selects ordinary sources, test sources, and supported headers. It does
not print a preliminary source list and performs no libclang `Parsing` stage.

`--format` defaults to `format.v1`. The value is relative to:

    HARD_ROOT/format

Validation rules:

- reject empty values;
- reject absolute paths;
- reject lexical `..` escapes;
- require a regular file;
- reject symlinks whose real target escapes the real format directory;
- permit an internal symlink to a regular style file;
- if source selection is empty, succeed without validating the style or
  finding `clang-format`.

For every selected file, run one process:

    clang-format --style=file:<resolved-style-path> -i <file>

Up to the resolved job count run concurrently. A formatter exit failure is
accumulated while independent files continue. Failure to start clang-format is
fatal and stops new scheduling.

Output:

- normal: one progress line only;
- verbose: a permanent completion line, followed immediately by a unified
  diff when the file changed;
- silent: no successful output, but formatting errors remain on stderr;
- unchanged files have a progress completion and no diff;
- diff headers are bold, hunks cyan, removals red, additions green unless
  `--no-color` is active.

Diff generation is internal Go code using `go-difflib`, with three context
lines. No external `diff` process is used. Earlier designs considered letting
clang-format print a diff and using system `diff -u`; the confirmed final
implementation uses the Go library.

## Unified libclang analysis

Dependency discovery, forward-declaration extraction, and entry-point
detection use one libclang 18 bridge rather than the former parser or several
unrelated textual scanners. The user explicitly selected this unified design
after considering using libclang only for forward declarations.

Dependency translation units use detailed preprocessing records, keep-going,
and skipped function bodies. They receive the source absolute path,
`HARD_CFLAGS`, `-working-directory`, and C++ mode unless `-x` already exists.

Inclusion records provide:

- the physical including file;
- a resolved target when available;
- the preprocessor-expanded include spelling;
- libclang's system-header classification.

The active dependency graph therefore covers direct, transitive, conditional,
macro-expanded, and force-included headers. The translation unit itself and
resolved system headers are removed. Remaining dependency paths are resolved
through symlinks, made absolute, deduplicated, and sorted per source.

Unresolved includes are preserved with diagnostics. Only actionable GitHub or
well-known paths trigger a snapshot request. Other missing includes retain the
original libclang error and never cause arbitrary network access.

## External repositories

An unresolved expanded include beginning with:

    github.com/<owner>/<repository>/...

maps to repository:

    github.com/<owner>/<repository>

The resolver requests GitHub's REST tarball endpoint without a ref:

    /repos/<owner>/<repository>/tarball

GitHub therefore selects the current default branch. Only a source snapshot is
installed; there is no Git history. The cache path is:

    HARD_ROOT/source/github.com/<owner>/<repository>

The current well-known mapping is:

    hard/<path> -> github.com/hard-build/library/<path>

It also creates the relative alias:

    HARD_ROOT/source/hard -> github.com/hard-build/library

An existing alias is valid only when it is a symbolic link resolving to the
mapped canonical repository. A conflicting file, directory, or symlink is an
error and is not replaced.

Snapshot installation:

- download into a temporary sibling location;
- require one generated archive root and strip it;
- accept directories, regular files, safe relative symlinks, and standard
  global PAX metadata;
- reject `.git`, absolute or traversing paths, escaping symlinks, hard links,
  and other special entries;
- rename the completed temporary directory into place atomically;
- require an existing cache target to be a real directory, not a symlink or
  regular file.

One synchronized resolver is shared by all analysis workers in one build,
fetch, or test invocation. Concurrent demand for one repository produces one
HTTP request. Each source remembers repositories it already attempted so an
invalid include inside an installed repository cannot cause an infinite retry.
After a snapshot becomes available, libclang analysis repeats; newly exposed
transitive GitHub and well-known includes can trigger later iterations.

Immediately before an actual request, progress changes to:

    Downloading github.com/<owner>/<repository>

All repositories in one invocation share preparation step one. Cached
repositories produce no Downloading entry. Cache directories persist and are
never refreshed automatically. Removing the exact repository directory is the
current way to request a new snapshot. There is no configured private GitHub
authentication flow.

External repositories, including well-known ones, are managed source trees.
Their non-system headers use the same forward generation as project headers,
and their same-stem implementation sources are recursively discovered,
compiled, and linked when reachable.

## Forward declarations

The former `hard.py` forward-generation implementation was not copied. The
Go version uses libclang and includes only declarations physically originating
in the header currently being processed.

Each unique managed header is independently parsed as `c++-header` with
`HARD_CFLAGS`, working-directory information, detailed preprocessing,
keep-going, and skipped function bodies. Included files may be needed to parse
the header, but declarations from those files cannot enter its output.

Eligible declarations:

- named classes and structs directly declared at global scope;
- named classes and structs directly declared in namespaces;
- ordinary class templates and parameter packs;
- ordinary and inline namespace nesting;
- macro-expanded namespace names through semantic parents.

Excluded declarations:

- anonymous-namespace types;
- types local to functions or lambdas;
- types nested inside class bodies;
- class template specializations;
- duplicates.

Template default arguments are removed. Declarations are considered in source
offset order. After each candidate, the cumulative standalone forward header
is reparsed with libclang. A candidate that introduces an error is skipped,
while earlier valid candidates remain. This safe filter is important for
macro-heavy amalgamated headers such as nlohmann/json. A libclang `Parse Issue`
diagnostic belonging to the requested source header is still a generation
error with line and column.

Forward outputs begin with `#pragma once`. Their path mirrors the canonical
absolute input header below the selected environment build root. `_fwd` is
inserted before the original extension:

    file.h    -> file_fwd.h
    file.hh   -> file_fwd.hh
    file.hpp  -> file_fwd.hpp
    file.h++  -> file_fwd.h++
    file      -> file_fwd

Example:

    /home/user/project/include/file.hpp
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/include/file_fwd.hpp

Generation is parallel, canonical-header deduplicated, collision checked, and
atomic through a temporary file plus rename. Parent directories are created.
An existing output survives a parse failure. Each header reports `Parsing
<header>` immediately before analysis.

The environment support header reached through
`HARD_ROOT/env/HARD_ENV/hard.h` is the sole canonical path exception. It is not
standalone-forward-parsed and gets no generated forward. This is identity
based, not a blanket exclusion for the basename `hard.h`.

## `hard build`

Build root selection excludes `*_test.*`. For a non-empty selection, the
implemented pipeline is:

1. discover dependencies and configured entry definitions;
2. recursively add same-stem production sources for managed headers;
3. filter the canonical environment support header from forward-managed
   dependencies;
4. generate all required forward headers;
5. compile all root and automatically discovered sources;
6. resolve reachable dependency object sets for root entry sources;
7. link root entry binaries;
8. atomically copy successful binaries to delivery destinations.

If dependency discovery or forward generation reports an error, compilation
does not begin.

### Recursive implementation discovery

For each managed header, build examines its canonical directory for a non-test
source with the same canonical filename stem and one supported source
extension. Header and source extensions may differ:

    common/object.h -> common/object.cpp
    container.hpp   -> container.cc

Extension matching is case-insensitive. Discovery repeats in parallel batches
until no new canonical sources exist. Cycles are suppressed. No candidate
means header-only. More than one candidate is an ambiguity error listing the
candidates.

The initially selected sources are roots. Automatically discovered sources
are dependency-only and cannot independently create delivered binaries even if
they define a configured entry name.

### Object compilation

For each source, only forwards corresponding to that source's own direct,
transitive, conditional, macro-expanded, and force-included managed
dependencies are appended. Unrelated forwards are not included.

The command shape is:

    HARD_CC <HARD_CFLAGS...> \
      -include <first-forward> \
      -include <second-forward> \
      ... \
      -c <source> -o <object>

Generated forward `-include` pairs follow all `HARD_CFLAGS` and precede `-c`.
A source with no managed dependencies gets no generated includes. The original
environment support-header include remains inside `HARD_CFLAGS`, but no
support forward include is appended.

Compilation uses up to the resolved job count and creates parent directories.
Compiler exit failures are accumulated while independent jobs continue. A
failure to start the compiler is fatal to new scheduling. Verbose output prints
the exact shell-escaped compiler command immediately after the completed
`Compiling <source>` progress entry.

Object paths mirror the lexical absolute source path rather than the canonical
dependency path. The source extension remains before `.o`, preventing a
collision between `file.c` and `file.cpp`:

    /home/user/project/src/file.c
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.c.o

    /home/user/project/src/file.cpp
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.cpp.o

No timestamp or cache check exists; selected objects are rebuilt.

### Entry-point detection

Every root and automatically discovered source is separately parsed as a full
translation unit, without skipped function bodies, using `HARD_CFLAGS`.

A source is an entry source only if it directly defines a global function whose
exact name is in `HARD_ENTRYPOINTS`. Accepted definitions are directly under
the translation unit or an `extern` linkage specification.

Excluded:

- declarations without bodies;
- namespace and qualified namespace functions;
- class methods;
- local functions;
- lambdas and other nested definitions.

Active preprocessing controls detection, and definitions produced completely
by macros are recognized. Repeated definitions of one configured name are
deduplicated. Two different configured entry names in one source are an
ambiguity error.

Only entry sources from the initial root selection create binaries.
Automatically discovered entry definitions are retained only for the rule that
excludes other entry-source objects from one binary.

### Dependency-to-object graph

All objects finish compilation before linking. A dependency header is
associated with an implementation source when canonical directory and stem
match. For one root entry source, graph traversal begins with its object and
recursively follows those associations. Cycles are suppressed. Unrelated
sources and other entry sources are excluded.

This behavior fixed the earlier `004.circular_dependency` linker failure where
building only `example.cpp` omitted the implementation objects for
`container::container`, `container::push`, and `container::dump`.

### Linking

Linking always uses ordinary compiler-driver startup behavior:

    HARD_CC <entry-and-dependency-objects...> <HARD_LDFLAGS...> \
      -o <internal-binary>

Do not add `-nostartfiles`, `-e`, or a custom entry linker option solely because
`HARD_ENTRYPOINTS` contains `_start` or another name. Such entries must be
compatible with ordinary linking and the supplied flags or receive the normal
linker error.

Internal binaries remove the entry source extension while preserving the
mirrored lexical absolute path:

    /home/user/project/src/application.cpp
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/application

Internal collisions, including same-directory `application.c` and
`application.cpp`, are errors. A source with an empty stem cannot create a
binary. Link jobs use the resolved worker count. Compiler exit failures are
aggregated; a start failure stops new scheduling. Verbose mode prints the exact
shell-escaped link command immediately after the relevant `Linking` entry.

### Binary delivery and `-o`

Successful internal binaries are copied atomically through a temporary file
and rename. Permission bits, including executable bits, are retained. The
internal artifact remains.

- no `-o`: deliver beside each lexical entry source with its extension removed;
- `-o path/to/application`: exact file destination and exactly one entry;
- `-o path/to/bin/`: directory intent, create it when necessary, and preserve
  each entry source path relative to the process working directory below it;
- an existing directory is directory output even without a trailing separator;
- an entry outside the working directory is mirrored from its lexical absolute
  path below the output directory instead of escaping through `..`;
- relative destinations are relative to the process working directory;
- destination collisions are errors;
- an existing regular file may be atomically replaced;
- a final symlink, directory, or other non-regular target is rejected;
- with no entry source, `-o` is not validated and no binary is produced.

Preserving directory structure fixed the former collision where root builds
containing `001.helloworld/example.cpp` and `003.unittest/example.cpp` both
mapped to one project-root `example` output.

### Build progress and output

Search, source analysis, header analysis, and repository downloads share step
one. After successful forward preparation, total is:

    1 + number of compiled sources + 2 * number of root entry binaries

Compilation therefore starts at `[2/M]`. Linking and copying are part of the
same counter. Normal mode is one terminal line. Verbose mode has permanent
preparation and completion lines, compiler commands after compilation, linker
commands after linking, and no separate copy command. Silent mode hides
successful progress and commands while preserving errors.

Build no longer prints a preliminary list of discovered header files.

## `hard fetch`

Fetch selects all supported translation units, including `*_test.*`, then uses
the same `HARD_CFLAGS`, libclang analysis, recursive same-stem source closure,
GitHub recovery, well-known mapping, cache, and worker limit as build.

It stops after dependency closure succeeds. It does not:

- generate forward headers;
- create `HARD_ROOT/env` or the environment build tree;
- invoke `HARD_CC`;
- detect entry points;
- compile, link, or copy;
- call `pkg-config`;
- run tests.

An empty root selection is a successful no-op without job validation or
libclang analysis. Existing repository caches are reused and not refreshed.

Search, parsing, and actual downloads share its single live preparation step:

    [1/?] Searching source files
    [1/?] Parsing <source>
    [1/?] Downloading github.com/<owner>/<repository>

Later transitive repositories reuse the same step. Normal mode rewrites one
line and terminates it; verbose mode emits permanent activity lines; silent
mode emits no successful progress; no-color removes ANSI colors. A cached
fetch still reports search and parsing but no Downloading activity.

## `hard test`

Test root selection includes only case-insensitive `*_test.c`, `*_test.cc`,
`*_test.cpp`, and `*_test.c++`.

For a non-empty selection it obtains flags using:

    pkg-config --cflags gtest_main
    pkg-config --libs gtest_main

Each stdout result is independently parsed with `go-shellwords`, with
environment expansion and backticks disabled. Successful pkg-config stderr is
preserved except in silent mode. A start failure, nonzero status, malformed
output, or diagnostics write error stops before building tests. Empty
selection succeeds without pkg-config.

GoogleTest compiler flags are appended after `HARD_CFLAGS`. GoogleTest linker
flags are appended after `HARD_LDFLAGS`.

For each selected test root:

1. compute the recursive production dependency closure with entry detection
   disabled;
2. exclude other `*_test.*` sources from automatic implementation discovery;
3. use one shared GitHub resolver across every test plan;
4. filter the canonical environment support header;
5. globally deduplicate and generate required forwards;
6. globally deduplicate object jobs by output path and require identical
   dependency lists for a shared object;
7. compile each unique source once;
8. link each test with its reachable production objects and gtest_main flags;
9. run each successfully linked test.

Separate test closures are prepared concurrently, but each worker uses one
sequential closure walk. There is no nested `N x N` worker multiplication.
Forward generation, global compilation, test linking, and test execution are
separate invocation-wide phases, each with at most `-j` workers.

The link shape is:

    HARD_CC <test-and-dependency-objects...> \
      <HARD_LDFLAGS...> <gtest_main-linker-flags...> \
      -o <internal-test-binary>

`HARD_ENTRYPOINTS` is ignored. A test that defines an incompatible `main`
receives the normal linker failure. Test binaries use the same mirrored
internal binary path rule, remain in the environment build tree, and are never
copied into the project. Test does not accept `-o`.

A compilation failure skips every test needing that object. A link failure
skips only that test's execution. A process-start error or nonzero test status
is recorded while other scheduled tests continue. All failures are joined for
the final result.

The shared total is:

    1 + unique compiled sources + 2 * prepared test executables

The counter never resets. Compilation, linking, and testing entries occur in
completion order. Skipped work can leave the final displayed counter below the
planned total.

Output behavior:

- normal captures combined test stdout/stderr, discards successful output, and
  writes failed output after the progress line finishes;
- verbose attaches the test command and complete captured output atomically to
  its completed Testing entry; parallel blocks cannot interleave;
- silent hides progress, verbose commands, successful tool diagnostics, and
  successful tests, but writes failed test output and build errors to stderr;
- without `--no-color`, every captured GoogleTest receives
  `--gtest_color=yes` so its output remains colored;
- with `--no-color`, every test receives `--gtest_color=no`.

The `-j` limit applies to test execution itself, not only compilation and
linking. This was added after timing the multi-file
`/home/taitov/projects/hard-build/library` suite showed no improvement with the
earlier implementation.

## Artifact layout

The implemented layout is:

    HARD_ROOT/
    ├── format/
    │   └── format.v1
    ├── source/
    │   ├── hard -> github.com/hard-build/library
    │   └── github.com/
    │       └── <owner>/
    │           └── <repository>/
    └── env/
        └── HARD_ENV/
            ├── hard.h
            └── build/
                └── <absolute path without leading slash>/
                    ├── application
                    ├── file.cpp.o
                    ├── file_fwd.h
                    └── file_fwd.hpp

External repository snapshots are shared by all environments below one
`HARD_ROOT`. Environment build artifacts are isolated by `HARD_ENV`.

`hard.h` is an installation/environment input, not generated by the Go
program. Its repository source is
`/home/taitov/projects/hard-build/hard/hard.h`.
`make install` copies it to `PREFIX/share/hard/env/host/hard.h`. The working
tree itself does not contain an installed environment tree. Other environment
configurations must supply their own `env/<HARD_ENV>/hard.h`, copy or link the
support header, or use explicit compiler flags that omit the support include.

Forward paths use canonical header paths. Object and binary paths use lexical
absolute source paths. Both reject an environment name that escapes
`HARD_ROOT/env`.

## External examples and verification projects

The main example tree is:

    /home/taitov/projects/hard-build/example

Known scenarios:

- `001.helloworld`: one simple C++ application;
- `002.internal_library`: application sources and a shared internal object;
- `003.unittest`: ordinary and `_test` sources plus `random.h`;
- `004.circular_dependency`: mutually dependent component/container headers
  and implementations; used to validate recursive source discovery, forward
  headers, cyclic graph suppression, linking, and execution;
- `005.external_library`: includes
  `github.com/nlohmann/json/single_include/nlohmann/json.hpp`; used to validate
  GitHub snapshots, safe forward filtering, cache reuse, compilation, linking,
  and execution with `example.json`;
- `006.hardlib`: includes `hard/...`; used to validate the well-known mapping,
  alias, managed external sources, external objects, forward generation, and
  canonical progress paths.

`/home/taitov/projects/hard-build/library` is both a multi-file GoogleTest
project and the source repository behind `github.com/hard-build/library`. It
has a short English README describing its elements; requirements/build
instructions were intentionally omitted. Its history was squashed and an
English initial commit recreated at the user's request. Do not rewrite that
repository history again without explicit confirmation.

The cached hard-build library previously produced this GCC warning while
compiling `module.cpp`:

    invalid use of incomplete type ... hard::module

The source is `application.h` using `module->broadcast(...)` in a lambda while
only a forward declaration is visible. The preferred eventual library-side fix
is to move the relevant implementation into `application.cpp`. The user
explicitly rejected adding a manual `class application;` workaround and chose
to leave the library unchanged for now.

## Historical decisions that must remain visible

- The Go rewrite was initially developed in `v1.0` while the old version
  remained at the root. The user later deleted the old version and renamed
  `v1.0` to `hard`; the repository now contains only the Go generation.
- MIT was selected, with the current copyright identity.
- README is English and exposes only `format`, `fetch`, `build`, and `test`.
- Go 1.23 is required.
- Cobra was selected rather than a handwritten parser.
- `HARD_ROOT`, `HARD_ENV`, `HARD_CC`, `HARD_CFLAGS`, `HARD_LDFLAGS`, and
  `HARD_ENTRYPOINTS` are configuration inputs; configuration is not implemented
  as mutable package globals despite the early wording “global variables.”
- Source paths are printed relative to the working directory; managed GitHub
  progress paths use canonical `github.com/...` spelling.
- Directory symlinks are recursively traversed as ordinary directories, with
  canonical visited-directory cycle prevention.
- Build excludes case-insensitive `_test`; test selects it; format includes
  headers; fetch includes ordinary and test translation units.
- Format has no preliminary source list, uses `[N/M]` rather than a bar,
  supports silent and verbose output, and uses internal Go unified diffs.
- Verbose format prints each diff immediately after that file completes.
- `-jN` is an explicit count; bare or zero `-j` means every logical CPU.
- Build no longer prints discovered headers.
- Build compilation progress is labeled `Compiling`; linking and copying use
  `Linking` and `Copying`, all under one counter.
- Include discovery must honor `HARD_CFLAGS` and exclude system headers.
- The environment support header path changed from
  `HARD_ROOT/include/hard.h` to `HARD_ROOT/env/HARD_ENV/hard.h`.
- Forward files mirror absolute paths and preserve original header extensions.
- Only declarations physically owned by one header can enter its forward file.
- The former `hard.py` forward implementation was intentionally not copied and
  is no longer present in the repository.
- Compiler commands force-include the forwards required by each source.
- Entry names come only from `HARD_ENTRYPOINTS`, default `main _start`.
- Linking is always ordinary compiler-driver linking.
- Build binaries are internally retained and delivered beside sources or via
  `-o`; directory outputs preserve source paths to prevent basename collisions.
- Test uses GoogleTest, has one invocation-wide progress total, hides successful
  output in normal mode, preserves failure output, supports silent mode, and
  parallelizes actual test processes.
- An include beginning with `github.com/` triggers a default-branch snapshot,
  not a Git clone, below `HARD_ROOT/source`.
- `hard/...` is the well-known alias for
  `github.com/hard-build/library/...`.
- External repositories are managed source trees rather than opaque/system
  headers.
- libclang was selected as the unified mechanism for header discovery,
  dependency graphs, declaration extraction, and entry detection.
- Every command reports searching; build, fetch, and test report parsing before
  potentially long analysis.
- Multiple repositories downloaded in one invocation share one progress step,
  and each Downloading message is displayed before its request starts.
- The canonical target of the environment support `hard.h` is not separately
  parsed for forward generation and gets no support `_fwd`, while the original
  configured force include remains.
- The public installed command is `PREFIX/bin/hard`, a shell wrapper around
  `PREFIX/libexec/hard/hard`; data files are installed in `PREFIX/share/hard`.

## Known gaps and deliberately unchanged issues

- No incremental rebuild or stale-output cleanup exists.
- Cached external repositories are not automatically updated or validated.
- Old support forward files are not removed even though new builds no longer
  generate or include them.
- There is no private GitHub authentication configuration.
- The hard-build library incomplete-type warning remains intentionally
  unresolved in the external repository.

## Test inventory

- `hard/cli_test.go`: command defaults, paths, interspersed flags, silent
  options, all job syntaxes, invalid input, help, and hidden commands.
- `hard/config_test.go`: all defaults and overrides, environment choice, default
  source include and environment support include, present-empty flags, safe
  shell parsing, disabled expansion, malformed values, and home failures.
- `hard/source_test.go`: accepted and rejected extensions, case-insensitive test
  suffix, recursion/order, explicit paths, relative display, canonical
  deduplication, file and directory symlinks, cycles, missing paths, and empty
  selections.
- `hard/progress_test.go`: normal line replacement, verbose permanent entries,
  unknown totals, multiple live messages, exact-total transitions, padding,
  attached details, silence, and ANSI colors.
- `hard/format_test.go`: style containment, symlinks, exact clang-format
  arguments, empty no-op, search continuity, continued/fatal failures,
  output modes, colors, unchanged files, worker limits, diff adjacency, and
  unified diff edge cases.
- `hard/clang_test.go`: active direct/transitive/system includes, unresolved
  expanded spellings, flag-controlled conditionals, definitions, macro/inline
  namespaces, templates, defaults, parameter packs, and symlink deduplication.
- `hard/github_test.go`: GitHub/well-known mapping, alias
  creation/reuse/conflicts, exact HTTP requests, safe extraction, PAX metadata,
  `.git` and traversal rejection, persistent cache, concurrency deduplication,
  live progress, transitive retries, and non-GitHub diagnostics.
- `hard/forward_test.go`: physical-file extraction, namespace and template
  rendering, macro and inline namespaces, safe candidate filtering, exclusions,
  invalid syntax, mirrored paths, extension retention, environment escape,
  isolation, duplicates, atomic preservation, activity, cleanup, and slice safety.
- `hard/entry_test.go`: configured global definitions, main/custom names,
  extern C, declarations, namespace and qualified definitions, methods,
  conditional and macro definitions, empty configuration, deduplication, and
  ambiguity.
- `hard/build_test.go`: flags and includes, system exclusion, recursive managed
  sources, cycles and ambiguity, support-header exception, per-source forwards,
  object paths and creation, link graphs, external managed sources, canonical
  display, entry exclusion, binary/output collisions, output destinations,
  progress, commands, quoting, errors, parallel links, copy behavior, and real
  executable integrations.
- `hard/fetch_test.go`: empty no-op, search/parse progress, recursive repository
  downloads, shared progress step, install order, cached reuse, absence of
  environment build artifacts and compiler arguments, and invalid job counts.
- `hard/test_test.go`: empty no-op, pkg-config success/failure and parsing,
  production objects, support-header exception, internal test binaries, common
  progress, shared-object compilation, global worker limits, grouped verbose
  output, successful-output suppression, failure output, silence, GoogleTest
  color, continuation, aggregation, and command rendering.
- `hard/main_test.go`: normal, verbose, and silent search progress for every
  command while retaining command-specific selection.

## Required verification

For every completed Go implementation change, run these commands from the
`hard/` module directory and use an unused `/tmp` binary path:

    gofmt -d *.go
    go test ./...
    go test -race ./...
    go vet ./...
    go build -o /tmp/hard-check .
    go mod verify

For wrapper or installation changes, also run:

    sh -n hard.sh
    make BUILD_DIR=<unused /tmp path> build
    make BUILD_DIR=<unused /tmp path> PREFIX=<logical prefix> DESTDIR=<unused /tmp staging root> install

Inspect the staged paths, file modes, and file types, and invoke the staged
public wrapper. Do not install into the real home or system tree during routine
verification.

From the repository root:

    git diff --check

For documentation work, reread `README.md`, `AGENTS.md`, and `MEMORY.md`
completely; verify English language, four-command public scope, local links,
versions, flags, paths, defaults, implemented/target distinctions, and known
gaps.

For build behavior, use an isolated temporary `HARD_ROOT`, a real
`env/<environment>/hard.h`, and the system compiler. Keep outputs under `/tmp`.
Inspect artifact names with `find`, types with `file`, and run the binary.

For libclang dependency, forward, or entry changes, verify Clang major 18,
`/usr/lib/llvm-18/include/clang-c/Index.h`, and `libclang-18`, then build and run
`004.circular_dependency`.

For GitHub changes, use a fresh isolated root with `005.external_library`.
Verify request, snapshot path, absence of `.git`, cache reuse, json forward,
successful build/link/run, and `example.json` behavior.

For well-known or managed-external changes, use a fresh isolated root with
`006.hardlib`. Verify the relative hard alias, external forwards and objects,
library implementation compilation/linking, canonical labels, actual verbose
paths, runtime output, and cached reuse.

For fetch changes, use a fresh root and an external example. The first run must
show search, parsing, and downloads and must not create an environment build
tree. A second run must show search/parsing, omit Downloading, and preserve the
cache. Use a separate fresh root for subsequent build verification.

For test changes, run a real GoogleTest under an isolated root and a pair with
one passing and one failing executable. Confirm both run and aggregate status
is nonzero. For parallelism changes, compare `-j1` and bare `-j` on
`/home/taitov/projects/hard-build/library` and confirm shared numbering plus a
material wall-time improvement.

## Last known verification of the support-header rule

On 2026-08-20, before the repository layout cleanup, the implementation was
checked with a temporary environment whose `env/host/hard.h` linked to the
support header. That source header now resides at:

    /home/taitov/projects/hard-build/hard/hard.h

Future integration checks should link or copy this current root file into the
temporary `HARD_ROOT/env/<environment>/hard.h`.

Observed:

- build parsing showed the application and ordinary project header, not the
  support header as an independent forward parse;
- the compiler command retained the environment `-include`;
- the ordinary project `model_fwd.h` was generated and included;
- no mirrored `hard_fwd.h` for the support target existed;
- the application compiled, linked, copied, and returned status 0;
- a real GoogleTest compiled, linked, and passed;
- a passing/failing test pair both executed and aggregate status was 1;
- `004.circular_dependency` built and printed `[100, 200, 300]`;
- `go test`, race tests, vet, build, module verification, gofmt, and diff checks
  passed at that snapshot.

## Workspace safety snapshot

At the start of the wrapper and Makefile task on 2026-08-21, the worktree was
clean. The earlier repository-layout update followed the user's deletion of
the old version and rename of `v1.0` to `hard`. Preserve future unrelated
changes, and keep verification binaries outside the repository so the default
module output `hard/hard` is not created or overwritten.

## Resume checklist

When resuming work:

1. read `AGENTS.md` and `MEMORY.md` completely;
2. inspect `git status --short` and preserve unrelated changes;
3. read every file that will be edited completely;
4. verify the relevant current functions, flags, dependencies, and external
   tool versions;
5. for multi-edit work, present a file/change/check plan and wait for approval;
6. implement only the confirmed scope;
7. update `MEMORY.md` with every new requirement or material state change,
   update `README.md` with user-facing contract changes, and update `AGENTS.md`
   when repository working rules change;
8. run the required unit, race, static, build, module, diff, and applicable
   integration checks;
9. reread the complete diff skeptically;
10. report result, exact verification evidence, and remaining work.
