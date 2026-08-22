# hard project memory

Last updated: 2026-08-23.

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
- one source-context forward-declaration file per compiled translation unit;
- object compilation, dependency-object resolution, ordinary executable
  linking, and atomic delivery;
- content-addressed object, link, delivery, and successful-test result caching
  with `--no-cache` rebuild and rerun support;
- persistent semantic libclang result caching for build/test source
  analysis, including `(CACHED)` preparation output and `--no-cache` refresh;
- hard-owned GoogleTest listing and repeated exact or `*`/`?` selector
  syntax with validation and internal GoogleTest-filter conversion;
- GoogleTest compilation, linking, parallel execution, output grouping, and
  failure aggregation;
- the standalone `fetch` command;
- a POSIX command wrapper and Make-based user-local build and installation;
- exclusion of environment support `hard.h` declarations from source forwards
  while retaining its configured force include.

Not implemented:

- automatic refresh of external snapshots;
- stale generated-artifact cleanup.

Build and successful-test cache hits are validated by input and artifact
content. Existing external repository directories remain persistent snapshot
cache entries and are not refreshed automatically.

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
| `unittest/Makefile` | Passes Make variables and an optional scenario name to the declarative Python runner |
| `unittest/run.py` | Discovers, validates, and sequentially executes strict `test.yaml` scenarios |
| `unittest/requirements.txt` | Pins the PyYAML major version used by the integration runner |
| `unittest/README.md` | Documents the YAML schema, commands, variables, and new-scenario workflow |
| `unittest/001.*` through `unittest/010.*` | Self-contained C and C++ source scenarios whose local `test.yaml` files own every command argument and expectation |
| `hard/go.mod`, `hard/go.sum` | Module identity, Go version, dependencies, and checksums |
| `hard/main.go` | Process entry, dispatch, configuration loading, and shared search progress |
| `hard/main_test.go` | Search-progress behavior and discovery integration for all commands |
| `hard/cli.go` | Cobra command tree, flags, positional paths, test selectors, and job normalization |
| `hard/cli_test.go` | CLI defaults, validation, help, interspersed flags, test selection, and job forms |
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
| `hard/forward.go` | Source-context forward extraction, validation, rendering, path mapping, and atomic writes |
| `hard/forward_test.go` | Namespace/template output, translation-unit context, filtering, paths, and preservation |
| `hard/entry.go` | Configured global entry-function definition detection |
| `hard/entry_test.go` | Definitions, declarations, namespaces, macros, ambiguity, and empty config |
| `hard/build.go` | Dependency closure, support-header exclusion, compilation, link graph, and delivery |
| `hard/cache.go` | Content fingerprints, atomic artifact and parse-result records, semantic-result integrity, and file comparison |
| `hard/cache_test.go` | Artifact and parse cache keys, invalidation, no-cache, integrity, forward restoration, selector separation, and successful-test reuse |
| `hard/build_test.go` | Dependency, forwards, objects, entry binaries, output, and integrations |
| `hard/fetch.go` | Dependency-only source closure and external snapshot fetching |
| `hard/fetch_test.go` | Fetch progress, recursive repositories, caching, and absence of build artifacts |
| `hard/test.go` | GoogleTest listing, selector validation, plans, shared compilation, linking, caching, and execution |
| `hard/test_test.go` | Tool and test discovery, wildcard selectors, parallel phases, output modes, failures, and artifacts |

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

The integration runner below `unittest/` additionally requires Python 3.10 or
later and `PyYAML>=6.0,<7`. These are test-infrastructure dependencies, not
dependencies of the installed `hard` command. The runner uses PyYAML's safe
loader with an added duplicate-key rejection rule.

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
    hard build  [--no-cache] [-s|--silent] [-o <path>] [path...]
    hard fetch  [-s|--silent] [path...]
    hard test   [--list-tests] [--test=<selector>]...
                [--no-cache] [-s|--silent] [path...]

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
- `build` and `test`: `--no-cache`;
- `test`: `--list-tests` or repeatable `--test=<selector>`, which are
  mutually exclusive;
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
`noColor`, `noCache`, `listTests`, `testSelectors`, `jobs`, `format`,
and `output`. Only `build` populates `output`; only `build` and `test` can
set `noCache`; only `test` populates listing or selectors. The raw output
spelling preserves a trailing path separator because that separator declares
directory intent.

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
- defines the immutable toolchain/cache boundary for compiler, libclang
  resource headers, standard library, libc, sysroot, ABI, target, container,
  and user-provided `-isystem` trees;
- must change when that system state changes because system headers are not
  content-hashed by parse or object caches;
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

The same vector is used by libclang dependency, source-forward, and entry
analyses and by `HARD_CC` object compilation. Dependency and entry analysis add
the working directory and default to C++ mode unless `-x` is already present.

Build and test canonicalize `HARD_ROOT/env/HARD_ENV/hard.h` through symlinks
after dependency closure discovery and exclude declarations physically owned
by that canonical target from source forwards. The original
`HARD_CFLAGS -include` remains.
Consequences:

- libclang and the compiler still see the environment support header;
- it receives no standalone `Parsing` step or per-header forward output;
- each source still receives its own generated forward include;
- unrelated project or external headers also named `hard.h` remain managed;
- old header-specific forward artifacts are not deleted automatically;
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
  source preparation and object compilation;
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
- `build`: search, all source parsing, and all repository downloads
  reuse preparation step one; total becomes `1 + sources + 2 * root entry
  binaries`; compilation begins at `[2/M]`.
- `fetch`: search, parsing, and downloads form its single preparation step;
  live output stays `[1/?]` and final total is one.
- `test`: search, parsing, and downloads reuse preparation step one. Ordinary
  and list-only runs use `1 + unique compilations + 2 * prepared test
  executables`; filtered runs use one additional listing step per prepared
  executable. Compilation begins at `[2/M]`.

Progress labels:

- format: the selected file;
- build: `Searching source files`, `Parsing ...`, `Downloading ...`,
  `Compiling ...`, `Linking ...`, `Copying ...`;
- fetch: `Searching source files`, `Parsing ...`, `Downloading ...`;
- test: `Searching source files`, `Parsing ...`, `Downloading ...`,
  `Compiling ...`, `Linking ...`, `Listing ...`, `Testing ...`, with listing
  present only when requested or required for selector validation.

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
after considering using libclang only for forward declarations. Build and test
reuse the final dependency-analysis AST for source-context forward extraction.

Dependency translation units use detailed preprocessing records, keep-going,
and skipped function bodies. They receive the source absolute path,
`HARD_CFLAGS`, `-working-directory`, and C++ mode unless `-x` already exists.

Inclusion records provide:

- the physical including file;
- a resolved target when available;
- the preprocessor-expanded include spelling;
- libclang's system-header classification.

The active dependency graph therefore covers direct, transitive, conditional,
macro-expanded, and force-included headers. The translation unit itself is
removed. One canonical, deduplicated, sorted list excludes resolved system
headers and drives implementation discovery, source-forward filtering,
parse-result fingerprints, and compilation fingerprints. `HARD_ENV` represents
immutable system and toolchain state instead of hashing each system header.
Paths marked system by libclang, including user-provided `-isystem`
directories, are absent
from persistent dependency snapshots.

Unresolved includes are preserved with diagnostics. Only actionable GitHub or
well-known paths trigger a snapshot request. Other missing includes retain the
original libclang error and never cause arbitrary network access.

### Persistent parse-result cache

Successful source analysis in `build` and `test` writes a versioned
`<source>.hard-parse-cache.json` record below the mirrored
`HARD_ROOT/env/HARD_ENV/build` path. Each record contains the managed dependency
list, complete active non-system dependency snapshot, detected entry point, and
final validated source-forward text. A separate result digest protects these
semantic fields from valid-JSON corruption.

The action fingerprint contains the current `hard` executable digest,
`clang_getClangVersion()` value, working directory, ordered compiler flags,
configured entry names where relevant, and content digests for the input plus
every non-system dependency recorded by the prior successful analysis. Paths
are canonicalized, deduplicated, and sorted by the common action-key code.
Missing, malformed, version-mismatched, changed, or result-digest-mismatched
records are misses. A missing dependency is also a miss and the real parser
remains authoritative. System state is intentionally represented only by the
environment-specific artifact tree.

On a source hit, dependency and entry analysis are both skipped and their
stored semantic values continue graph discovery. The stored complete
dependency list is passed directly to compilation caching, eliminating the
former second dependency-only libclang parse immediately before every compile.
The stored forward is atomically restored when its output is missing.

`--no-cache` bypasses reads, invalidates each old parse record before real
analysis, and refreshes it after success. Failed analysis never writes a
record. `fetch` deliberately receives no parse cache and therefore still
creates no environment build tree.

No record is stored when the literal token `__has_include` occurs in the
input itself or the analysis argument vector. Dependencies containing the
token remain content-hashed when they are non-system without disabling the
input's cache, which keeps ordinary sources using standard-library headers
cacheable. The accepted depfile-style limitation is that changing
optional-header availability tested by `__has_include` inside a dependency, or
creating a new higher-priority header which shadows an already resolved
include, is invisible when the input and every previously known non-system
dependency remain byte-identical; use `--no-cache` after such include-path
topology changes.

A successful hit updates preparation progress to
`Parsing <display-path> (CACHED)`. No compiler or parser command detail is
attached because no child parse process exists.

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
Their active non-system headers can contribute declarations to each dependent
source forward. Same-stem implementation sources are recursively discovered,
compiled, and linked when reachable.

## Forward declarations

The former `hard.py` forward-generation implementation was not copied. The
Go version extracts declarations from the final dependency-analysis AST that
was already produced for each source translation unit. It does not parse each
managed header as a separate forward-generation input.

Each compiled source receives exactly one forward file. It contains eligible
declarations physically owned by that source context's active managed
non-system dependencies. Declarations physically owned by the translation unit
itself or a system header are excluded. Conditional preprocessing and macros
therefore remain source-specific even when two translation units include the
same physical header differently.

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
offset order after sorting by canonical physical file path and source offset.
After each candidate, the cumulative source forward is reparsed with libclang.
A candidate that introduces an error is skipped while earlier valid candidates
remain. This safe filter is important for macro-heavy amalgamated headers such
as nlohmann/json.

Forward outputs begin with `#pragma once`. Their path mirrors the lexical
absolute source path below the selected environment build root and appends
`.fwd.h` to the complete source filename:

    /home/user/project/src/first.cpp
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/first.cpp.fwd.h

The complete source extension is retained, so `first.c`, `first.cc`,
`first.cpp`, and `first.c++` have distinct outputs. No `value_fwd.h` or other
per-header forward file is created.

Generation occurs during source preparation after the final dependency AST is
parsed. Candidate validation uses the same source-context compiler flags.
Parent directories are created, writes are atomic, and a byte-identical regular
output is retained without replacement.

The source analysis cache stores the final validated text. A hit restores a
missing `source.cpp.fwd.h`, reports `Parsing <source> (CACHED)`, and performs no
libclang source analysis.

The environment support header reached through
`HARD_ROOT/env/HARD_ENV/hard.h` remains force-included, but declarations
physically owned by its canonical target are excluded from every source
forward. Other project or external headers named `hard.h` are treated normally.

## `hard build`

Build root selection excludes `*_test.*`. For a non-empty selection, the
implemented pipeline is:

1. discover dependencies and configured entry definitions;
2. recursively add same-stem production sources for managed headers;
3. generate one source-context forward for each translation unit while
   filtering declarations from the canonical environment support header;
4. compile all root and automatically discovered sources;
5. resolve reachable dependency object sets for root entry sources;
6. link root entry binaries;
7. atomically copy successful binaries to delivery destinations.

If source preparation reports an error, compilation does not begin.

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

For each source, exactly one source-context forward contains eligible
declarations from that source's own direct, transitive, conditional,
macro-expanded, and force-included active managed dependencies.

The command shape is:

    HARD_CC <HARD_CFLAGS...> \
      -include <source.cpp.fwd.h> \
      -c <source> -o <object>

The generated forward `-include` pair follows all `HARD_CFLAGS` and precedes
`-c`. A source with no eligible declarations still gets an output containing
`#pragma once`. The original environment support-header include remains inside
`HARD_CFLAGS`.

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

Successful compilation writes `<object>.hard-cache.json` atomically. Its input
fingerprint contains the `hard` executable digest, resolved compiler path and
digest, working directory, compiler arguments, source, every active resolved
non-system include, and generated source forward. `HARD_ENV` represents the
system headers and immutable toolchain state. Cache lookup verifies the sidecar
and current regular object digest. Missing, malformed, changed, or non-regular
records and artifacts are misses. `--no-cache` disables reads, invalidates the
old record before compilation, and stores a fresh record only after success.

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

Successful links use the same sidecar suffix beside the internal binary. Their
fingerprint contains the `hard` and compiler fingerprints, working directory,
link arguments, and every object digest. The binary digest is verified on hit.
A failed or forced link cannot retain an eligible older record because the
sidecar is invalidated before the compiler is started.

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

When cache reads are enabled, delivery compares internal and destination
regular-file content and permissions and skips an already identical copy.
`--no-cache` forces delivery. No delivery sidecar is needed.

### Build progress and output

Search, source analysis, and repository downloads share step one. After
successful source preparation, total is:

    1 + number of compiled sources + 2 * number of root entry binaries

Compilation therefore starts at `[2/M]`. Linking and copying are part of the
same counter. Normal mode is one terminal line. Verbose mode has permanent
preparation and completion lines, compiler commands after compilation, linker
commands after linking, and no separate copy command. Silent mode hides
successful progress and commands while preserving errors.

Build no longer prints a preliminary list of discovered header files.
Cache hits consume normal progress steps with `(CACHED)` after the label.
Cached source analyses append `(CACHED)` to their `Parsing` label.
Cached compile and link entries have no verbose command because no child
process was started; an identical delivery is `Copying <binary> (CACHED)`.

## `hard fetch`

Fetch never reads or writes the environment-backed persistent parse-result
cache; its parsing behavior and absence of `HARD_ROOT/env` artifacts remain
unchanged.
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

The command owns its selection syntax:

    hard test --list-tests [path...]
    hard test --test=<selector> [--test=<selector>...] [path...]

Without either flag, every test runs. `--list-tests` and `--test` are
mutually exclusive. Listing builds and links each selected test executable,
runs GoogleTest discovery without the successful-test cache, parses its output,
and prints normalized full test names. One source produces one name per line.
Multiple sources produce lexical source headings with indented names.
`--silent` hides progress but not this requested list output.

`--test` is repeatable. Each value is one positive full-name selector:
`*` matches zero or more bytes and `?` matches exactly one byte. A selector
without either wildcard is exact. Empty values, `:`, and `-` are rejected;
there is no public negative-filter or raw GoogleTest-argument passthrough.
Shell callers quote wildcard selectors. Validated selectors are joined
internally as one GoogleTest positive filter, followed by hard's authoritative
GoogleTest color argument.

Filtered execution first runs uncached GoogleTest discovery on every
successfully linked binary. Every selector must match at least one listed test
across the complete invocation. It may match no test in one binary when it
matches another, but any globally unmatched selector is an error and prevents
the filtered execution phase. Discovery understands ordinary, typed, and
parameterized GoogleTest list names by combining each suite heading and
indented test name while removing GoogleTest's trailing explanatory comments.

The converted selector argument participates in the successful-test cache
fingerprint, so exact, wildcard, and repeated-selector combinations have
independent records. A repeated filtered execution may report
`Testing <binary> (CACHED)`, but discovery still runs first to validate the
current public selector.

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
4. generate one source-context forward per translation unit while excluding
   declarations from the canonical environment support header;
5. globally deduplicate object jobs by output path and require identical
   dependency lists for a shared object;
6. compile each unique source once;
7. link each test with its reachable production objects and gtest_main flags;
8. list tests when listing or selectors require discovery;
9. validate selectors and run each successfully linked test unless the
   invocation is list-only.

Separate test closures are prepared concurrently, but each worker uses one
sequential closure walk. There is no nested `N x N` worker multiplication.
Source preparation, global compilation, test linking, listing, and test
execution are separate invocation-wide phases, each with at most `-j`
workers.

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

The normal and list-only shared total is:

    1 + unique compiled sources + 2 * prepared test executables

Normal mode assigns the two per-test steps to linking and testing. List-only
mode assigns them to linking and listing. A filtered invocation uses:

    1 + unique compiled sources + 3 * prepared test executables

for linking, listing, and testing. The counter never resets. Phase entries
occur in completion order. Skipped work, including testing prevented by an
unmatched selector, can leave the final displayed counter below the planned
total.

Output behavior:

- normal captures combined test stdout/stderr, discards successful output, and
  writes failed output after the progress line finishes;
- verbose attaches the test command and complete captured output atomically to
  its completed Listing or Testing entry; parallel blocks cannot interleave;
- silent hides progress, verbose commands, successful tool diagnostics, and
  successful tests, but writes failed test output and build errors to stderr;
- normalized `--list-tests` names are always stdout command output, including
  in silent mode; raw successful GoogleTest listing output follows only a
  verbose Listing entry;
- without `--no-color`, every captured GoogleTest receives
  `--gtest_color=yes` so its output remains colored;
- with `--no-color`, every test receives `--gtest_color=no`.

Source analysis uses the persistent semantic parse-result cache described
above. Parse hits report `Parsing ... (CACHED)`.
Compilation and linking share the build content cache. A successful test run
writes `<internal-test-binary>.hard-test-cache.json`; its key contains the
binary path and digest, test arguments, working directory, and `hard` digest.
A valid hit skips process execution and reports `Testing <binary> (CACHED)`.
Converted selectors are test arguments and therefore separate cache keys.
Listing and selector-validation discovery deliberately receive no
successful-test cache.
The record is invalidated before every actual run and stored only after exit
status zero, so failures are never cached. Runtime state outside the binary,
arguments, and working directory is not inferred; use `hard test --no-cache`
for undeclared files, services, network data, time, or similar inputs.

`--no-cache` disables parse, compile, link, delivery, and test-result reads
for the invocation, forces all those actions, and refreshes successful records.
It does not clear or refresh the GitHub snapshot cache.

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
                    ├── application.hard-cache.json
                    ├── application.hard-test-cache.json
                    ├── file.cpp.hard-parse-cache.json
                    ├── file.cpp.fwd.h
                    ├── file.cpp.o
                    └── file.cpp.o.hard-cache.json

External repository snapshots are shared by all environments below one
`HARD_ROOT`. Environment build artifacts are isolated by `HARD_ENV`.

`hard.h` is an installation/environment input, not generated by the Go
program. Its repository source is
`/home/taitov/projects/hard-build/hard/hard.h`.
`make install` copies it to `PREFIX/share/hard/env/host/hard.h`. The working
tree itself does not contain an installed environment tree. Other environment
configurations must supply their own `env/<HARD_ENV>/hard.h`, copy or link the
support header, or use explicit compiler flags that omit the support include.

Forward, object, and binary paths use lexical absolute source paths. They all
reject an environment name that escapes `HARD_ROOT/env`.

## Integration fixtures and external examples

The repository contains a self-contained positive integration suite below
`unittest/`. It has ten scenarios, thirteen application entry points, eight
GoogleTest translation units, and fifteen GoogleTest cases. The scenarios cover
a minimal application, multiple entries sharing an object, transitive
implementation discovery, cyclic headers and implementation graphs, a
header-only template, ordinary GoogleTest production dependencies, an object
shared by two test plans, all supported source extensions, equal binary
basenames in different directories, and the force-included environment support
header.

The self-contained unit is one numbered scenario, not each individual
GoogleTest `TEST` declaration. Every scenario directory owns one readable
`test.yaml` containing a description and an ordered `steps` list. Each step
contains exactly one restricted action:

- `build` runs verbose `hard build --no-cache` for explicit source arguments
  and may require exact source labels to compile once;
- `run` requires the latest build to have copied one regular executable,
  runs it, and compares the configured exit code and exact stdout/stderr;
- `test` runs verbose `hard test --no-cache`, may require exact source labels to
  compile once, and requires the configured GoogleTest binaries and passing
  counts.

The runner deliberately disables cache reads for `build` and `test` actions so
`compiled_once`, copied-binary, and executed-test checks describe the current
scenario step rather than artifacts or successful results from an earlier run.

`unittest/run.py` automatically discovers immediate subdirectories containing
`test.yaml`, validates the complete YAML schema, and executes steps in file
order. It rejects unknown actions and fields, duplicate YAML keys, wrong value
types, absolute or escaping paths, and `run` before `build`. Scenario YAML
cannot invoke arbitrary shell commands. The runner stops the current scenario
on its first failure, continues with independent scenarios, and returns one
aggregate status.

`unittest/Makefile` only passes configuration variables and an optional
scenario name to the runner. It contains no scenario directory list,
application names, expected stdout, GoogleTest binary names, or
source-compilation expectations. The default command is:

    make -C unittest

`PYTHON`, `HARD`, `OUTPUT`, `JOBS`, and `SCENARIO` are Make variables.
They default to `python3`, the installed `hard` command,
`/tmp/hard-unittest`, zero, and empty respectively. Zero jobs select all
logical CPUs through the public `hard` semantics. An empty `SCENARIO`
discovers every YAML scenario; a name selects only that scenario. Direct
runner invocation accepts zero or more scenario names. The fixtures use only
local and system headers, so they require no external repository download.
Generated headers, objects, and test binaries remain below `HARD_ROOT`;
delivered application binaries remain below `OUTPUT/<scenario>`.

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
  alias, managed external sources, external objects, source-forward generation,
  and canonical progress paths.

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
- `HARD_ENV` is the immutability boundary for system headers and toolchain
  state; parse and object caches hash only non-system dependencies.
- The environment support header path changed from
  `HARD_ROOT/include/hard.h` to `HARD_ROOT/env/HARD_ENV/hard.h`.
- Forward files mirror lexical absolute source paths and append `.fwd.h` to the
  complete source name, such as `first.cpp.fwd.h` and `second.cpp.fwd.h`.
- Every compiled translation unit receives exactly one source-context forward.
- The former `hard.py` forward implementation was intentionally not copied and
  is no longer present in the repository.
- Compiler commands force-include exactly one generated forward per source.
- Entry names come only from `HARD_ENTRYPOINTS`, default `main _start`.
- Linking is always ordinary compiler-driver linking.
- Build binaries are internally retained and delivered beside sources or via
  `-o`; directory outputs preserve source paths to prevent basename collisions.
- Test uses GoogleTest, has one invocation-wide progress total, hides successful
  output in normal mode, preserves failure output, supports silent mode, and
  parallelizes actual test processes.
- Test listing and selection use hard-owned `--list-tests` and repeatable
  `--test` syntax. Selectors are positive full-name patterns with only `*` and
  `?`; hard validates them against real discovery and converts them internally
  to one GoogleTest filter instead of exposing raw GoogleTest arguments.
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
- Declarations owned by the canonical environment support `hard.h` are excluded
  from source forwards, while the original configured force include remains.
- The public installed command is `PREFIX/bin/hard`, a shell wrapper around
  `PREFIX/libexec/hard/hard`; data files are installed in `PREFIX/share/hard`.
- Integration scenarios use a strict, ordered `test.yaml` step list interpreted
  by one Python runner. Scenario files may use only `build`, `run`, and
  `test`; they cannot contain arbitrary commands. Immediate directories are
  discovered automatically, so a new scenario needs only sources and its local
  YAML file.

## Known gaps and deliberately unchanged issues

- System-header and toolchain-state changes inside one `HARD_ENV` intentionally
  do not invalidate parse or object caches. Select a new environment after
  compiler, libclang resource, standard-library, libc, sysroot, ABI, target,
  container, or `-isystem` tree changes; use `--no-cache` for a one-off forced
  rebuild in the current environment.
- Parse-result records use the previously observed include set. A newly created
  higher-priority header can shadow an existing include without invalidating
  that set. A dependency can also test the availability of an optional header
  through `__has_include` without that unavailable header entering the known
  set. Run build or test with `--no-cache` after such topology changes.
- Unreferenced stale generated artifacts are not removed automatically.
- Test-result keys cannot infer undeclared runtime files, services, network
  responses, or time; callers use `hard test --no-cache` when these matter.
- Cached external repositories are not automatically updated or validated.
- Old per-header forward files and header parse records are not removed even
  though new builds no longer generate or include them.
- There is no private GitHub authentication configuration.
- The hard-build library incomplete-type warning remains intentionally
  unresolved in the external repository.

## Test inventory

- `hard/cli_test.go`: command defaults, paths, interspersed and no-cache flags,
  silent options, all job syntaxes, invalid input, help, and hidden commands.
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
  namespaces, templates, defaults, parameter packs, symlink deduplication, and
  system-header cache exclusion.
- `hard/github_test.go`: GitHub/well-known mapping, alias
  creation/reuse/conflicts, exact HTTP requests, safe extraction, PAX metadata,
  `.git` and traversal rejection, persistent cache, concurrency deduplication,
  live progress, transitive retries, and non-GitHub diagnostics.
- `hard/forward_test.go`: physical-file extraction, namespace and template
  rendering, macro and inline namespaces, safe candidate filtering, exclusions,
  invalid syntax, source-context paths, translation-unit conditional isolation,
  duplicates, atomic preservation, unchanged-output retention, and slice safety.
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
- `hard/cache_test.go`: stable content keys, compiler/input/dependency/source
  invalidation, semantic-result integrity, malformed records, source parse hits,
  input/flag `__has_include` suppression, cacheable dependencies containing the
  token, standard-library parse reuse, `HARD_ENV` handling of `-isystem` header
  changes, source-forward restoration, no-cache refresh,
  compile/link/delivery reuse, build/test `Parsing ... (CACHED)` output, and
  successful-only test-result reuse with selector-separated keys and uncached
  listing.
- `hard/fetch_test.go`: empty no-op, search/parse progress, recursive repository
  downloads, shared progress step, install order, cached reuse, absence of
  environment build artifacts and compiler arguments, and invalid job counts.
- `hard/test_test.go`: empty no-op, pkg-config success/failure and parsing,
  production objects, support-header exception, internal test binaries, common
  progress, shared-object compilation, global worker limits, grouped verbose
  output, successful-output suppression, failure output, silence, GoogleTest
  color, normalized listing, exact and wildcard selectors, unmatched-selector
  rejection, continuation, aggregation, command rendering, and cache-aware
  helper signatures.
- `hard/main_test.go`: normal, verbose, and silent search progress for every
  command while retaining command-specific selection.
- `unittest/`: ten declarative source-tree scenarios whose local `test.yaml`
  files require thirteen applications to build and produce exact outputs,
  eight GoogleTest binaries with fifteen cases to run successfully, automatic
  dependency sources to be discovered, and one shared production object to
  compile once; the Python runner validates and executes ordered steps, while
  the top-level Makefile only passes configuration.

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
is nonzero. For listing or selection changes, additionally verify normalized
single- and multi-source lists, exact and repeated selection, quoted `*` and
`?` patterns, rejection of an unmatched selector before execution, selector-
specific successful-result caching, and uncached discovery on repeated list
and filtered invocations. For parallelism changes, compare `-j1` and bare `-j` on
`/home/taitov/projects/hard-build/library` and confirm shared numbering plus a
material wall-time improvement.

For `unittest/` fixture, runner, YAML schema, or Makefile changes, first
verify the documented Python and PyYAML versions. Load every committed
`test.yaml` through the strict schema and exercise rejection of duplicate
keys, unknown actions and fields, invalid value types, escaping paths, and
`run` before `build` using temporary YAML below `/tmp`.

Build the current backend to an unused `/tmp` path and create an isolated
`HARD_ROOT` containing the current `env/<environment>/hard.h` and format
file. Run every numbered scenario independently by passing its name to:

    python3 unittest/run.py \
      --hard <temporary-backend> \
      --output <temporary-output> \
      <scenario>

Then run automatic discovery through:

    make -C unittest HARD=<temporary-backend> OUTPUT=<temporary-output>

Require every scenario success message and the final aggregate suite success
message. The YAML steps for `003.transitive_dependency` and
`004.circular_dependency` must continue to select only `example.cpp`,
require the production implementation sources to compile exactly once, and
run the copied binaries. This verifies dependency discovery instead of relying
on a full-directory source scan.

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

## Last known verification of content caching

On 2026-08-22, the cache implementation was checked with the required Go
commands from `hard/` and `git diff --check` from the repository root. Unit
coverage included source, compiler, dependency, object, binary, sidecar, and
`--no-cache` invalidation; malformed records; compile, link, and delivery
reuse; successful test-result reuse; and the rule that failing tests are never
cached.

All ten declarative `unittest/` scenarios passed with a newly built backend and
an isolated `HARD_ROOT`. A repeated real GoogleTest invocation reported cached
compilation, linking, and execution, including:

    Testing calculator_test (CACHED)

No GoogleTest process output appeared on that cached run, confirming that the
successful test binary was not executed again.

## Last known verification of persistent parsing cache

On 2026-08-22, the persistent parse-result implementation passed the complete
required Go check set: clean gofmt diff, ordinary and race tests, vet, an
out-of-tree backend build, module verification, and repository diff checking.
The backend linked `libclang-18.so.18`; `clang-18 --version` reported
18.1.3 and the configured LLVM 18 `Index.h` was present.

Unit coverage confirmed source and header hits, source/known-dependency/compiler
flag/entry-name invalidation, malformed and semantically corrupted records,
`--no-cache` refresh, absence of records after failed parsing, suppression
for `__has_include` in the direct input and flags, cache reuse when a
transitive or standard-library dependency contains the token, restoration of
a deleted forward output, and build/test `Parsing ... (CACHED)` progress.

A separately built backend passed all ten declarative integration scenarios
with an isolated `HARD_ROOT`, including the cyclic build and real GoogleTest
cases. A minimal real build was then invoked in separate processes with an
isolated root and explicit empty compiler/linker flag vectors. The second
invocation reported:

    Parsing main.cpp (CACHED)
    Compiling main.cpp (CACHED)
    Linking main (CACHED)
    Copying main (CACHED)

A following `build --no-cache` reported none of those hits and recreated
`main.cpp.hard-parse-cache.json`, confirming forced analysis and refresh.

Later on 2026-08-22, a full build of the main `example/` tree exposed that the
initial guard also scanned every transitive and system dependency. Standard
library headers contain `__has_include`, so no parse records were created in
normal C++ projects. The guard was narrowed to the direct input and analysis
arguments while dependency contents remain in the fingerprint. The complete
required Go checks and all ten declarative integration scenarios passed again.
Two separate full `example/` builds with a fresh isolated root created 23 parse
records; the second build reported 23 cached analyses. The only repeated
analysis was the direct nlohmann `json.hpp` input, which itself contains
`__has_include` and therefore remains intentionally uncached.

Later on 2026-08-22, `HARD_ENV` was accepted as the immutability boundary for
the compiler toolchain and system include trees. Parse-cache format version 2
therefore excludes libclang system-header targets, including headers reached
through `-isystem`, from both parse-result and compilation fingerprints while
retaining the complete active non-system dependency closure. A targeted
`-isystem` test confirmed that changing such a header reuses both parsing and
compilation in the same environment, while `--no-cache` forces both operations.

The required Go checks and all ten declarative integration scenarios passed
with a newly built backend. Two full `example/` builds with another isolated
root produced 23 parse records containing no `/usr` paths. The
`001.helloworld/example.cpp` record contained only the environment `hard.h` as
its dependency. The second build reported 23 cached analyses; nlohmann
`json.hpp` remained the sole direct input parsed again because it contains
`__has_include`. The measured wall times were 30.60 seconds for the first build
and 23.80 seconds for the second build.

## Last known verification of hard test selection

On 2026-08-23, the hard-owned GoogleTest listing and selection interface passed
clean gofmt, ordinary and race tests, vet, an out-of-tree backend build, module
verification, and repository diff checking.

With GoogleTest 1.14.0 and an isolated `HARD_ROOT`, a single-source list
reported five normalized `calculator_test.*` names. Repeating it reused
parsing, compilation, and linking while executing `Listing calculator_test`
again. A repeated exact selector still performed listing and then reported
`Testing calculator_test (CACHED)`.

One filtered run used the repeated selectors
`calculator_test.?dds_values` and `calculator_test.*zero`. The rendered
GoogleTest filter preserved both patterns and exactly the expected two cases
passed. A two-source silent list printed lexical source headings and five
normalized names. Repeated exact selectors matching different binaries ran one
case in each. An unmatched selector returned status 1 after listing both
binaries and before any Testing entry.

A separate real passing/failing pair scheduled both test binaries and returned
status 1 after the expected failure. Finally, all ten declarative integration
scenarios passed with the newly built backend, including eight GoogleTest
binaries and fifteen successful cases.

## Last known verification of source-context forwards

On 2026-08-22, the source-context forward implementation passed clean gofmt,
ordinary and race tests, vet, an out-of-tree backend build, module verification,
and repository diff checking. The backend used Clang 18.1.3, found the LLVM 18
`Index.h`, and linked `libclang-18.so.18`.

An isolated real `004.circular_dependency` build parsed only its three `.cpp`
translation units. It created `example.cpp.fwd.h`,
`component/component.cpp.fwd.h`, and `container/container.cpp.fwd.h`, with no
`*_fwd.*` per-header output. The executable printed `[100 200 300]`.

A second build reported cached parsing for all three sources and cached
compilation, linking, and delivery. This confirmed that source parse records
restore and reuse the matching source forward rather than parsing headers.

All ten declarative integration scenarios passed with the newly built backend.
Their verbose compiler commands contained exactly one source-specific forward
include for every compiled C, CC, CPP, or C++ translation unit. The real
GoogleTest scenarios compiled, linked, and passed all configured tests. A
separate real passing/failing pair executed both binaries and returned status 1
after reporting the expected failed test.

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
