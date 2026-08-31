# hard project memory

Last updated: 2026-08-31.

This document is a self-contained memory snapshot for the current Go
implementation of `hard`. It records the product intent, confirmed
requirements, implemented behavior, architecture, operational constraints,
tests, known gaps, and the history needed to continue development without the
original conversation.

The repository root is `/home/taitov/projects/hard-build/hard`. The Go module
and implementation live in its `hard/` directory. Root-level documentation,
formatting assets, the runtime support header, and container assets belong
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
- [README.md](README.md) is the English newcomer overview and quick start.
  [docs/reference.md](docs/reference.md) is the complete public behavior
  reference. Use tests and code to confirm implementation status.
- When sources disagree, use this precedence:
  1. the newest explicit user instruction;
  2. repository rules in `AGENTS.md`;
  3. current tests and implementation in `hard/`;
  4. this memory snapshot;
  5. public behavior in `docs/reference.md`;
  6. the newcomer overview in `README.md`.
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
to derive formatting, dependencies, compilation, linking, execution, and
tests from the selected sources and their include graph rather than from a
hand-written project build file.

The public operations are deliberately limited to:

    hard version
    hard environment
    hard format
    hard fetch
    hard build
    hard run
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
- an embedded `v4.0-development` version assembled from the `4.0` version
  number and `development` prerelease identifier, with a source-independent
  `version` command and release-time prerelease removal;
- a source-independent `environment` report covering the runtime, operating
  system, CPU, libc, compiler, target triple, configured flags, executable
  naming/execution settings, and libclang, with aligned colored sections, a
  title enclosed between horizontal rules, internal compiler include mechanics
  omitted, and a plain `--no-color` form;
- command-specific source discovery;
- recursive traversal through directory symlinks with cycle prevention;
- relative source display paths and canonical deduplication;
- common colored, verbose, normal, and silent progress output;
- parallel formatting with internal unified diffs;
- unified libclang 18 dependency, declaration, and entry-point analysis;
- recursive GitHub snapshot fetching and persistent caching;
- strict embedded `hard.recipe.v1` YAML recipes for content-addressed CMake
  builds of reachable static third-party libraries;
- the well-known `hard/` library and `recipe/` recipe repository mappings;
- one source-context forward-declaration file per compiled translation unit;
- object compilation, dependency-object resolution, ordinary executable
  linking, atomic build delivery, and direct execution of internal binaries;
- content-addressed object, link, delivery, and successful-test result caching
  with `--no-cache` rebuild and rerun support;
- persistent semantic libclang result caching for build/run/test source
  analysis, including `(CACHED)` preparation output and `--no-cache` refresh;
- hard-owned GoogleTest listing and repeated exact or `*`/`?` selector
  syntax with validation and internal GoogleTest-filter conversion;
- GoogleTest compilation, linking, parallel execution, output grouping, and
  failure aggregation;
- the standalone `fetch` command;
- a `run` command that builds exactly one root entry target without delivery,
  forwards program arguments and live streams, and propagates its exit status;
- a POSIX command wrapper and Make-based user-local build, installation,
  one-command repository verification, and root integration-suite delegation;
- a no-argument `curl | sh` installer for the latest checksum-verified portable
  `linux/amd64` release, with user-local staged replacement and idempotent Bash,
  Zsh, Fish, or POSIX-shell `PATH` startup configuration, eight named progress
  stages, and informational hello-world toolchain recommendations;
- generated Bash, Zsh, and Fish completion files, with target values owned by
  the wrapper and every other dynamic request dispatched to the host backend;
- wrapper-owned `linux64` and `windows64` Docker targets that select their
  always-refreshed GHCR `latest` tags or any syntactically valid explicit
  Docker tag while bind-mounting the persistent source/cache root from the
  host;
- arbitrary `docker://image` wrapper targets with pull-if-missing behavior and
  the same mounts, identity, workdir, and image-owned entrypoint contract;
- generic `HARD_EXECUTABLE_SUFFIX` and `HARD_EXECUTABLE_RUNNER` behavior for
  inferred artifact names and run/test process vectors, independent of
  `HARD_ENV` spelling;
- separate executable-relative runtime bundles for the host and container,
  with shared source snapshots and environment-specific caches below
  `HARD_ROOT`;
- exclusion of runtime support `hard.h` declarations from source forwards
  while retaining its backend-managed force include;
- a GitHub Actions check workflow that runs the canonical `make check` target
  and all declarative host integration scenarios in separate Ubuntu 24.04 jobs
  for pull requests and pushes to `main`, while cancelling superseded runs for
  the same ref;
- a GitHub Actions workflow that automatically publishes an image version only
  when a previously unseen `target/<image>/<version>.Dockerfile` is added to
  `main`, offers guarded manual recovery for a failed first publication, keeps
  version tags immutable, advances `linux64:latest` only for the newest glibc
  version, and advances `windows64:latest` only for the newest
  LLVM-MinGW UCRT version;
- a release-tag GitHub workflow that builds the host backend against
  the Ubuntu 18.04 glibc baseline and publishes a portable archive and SHA-256
  file.

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
| `README.md` | English newcomer overview, quick start, and concise command guide |
| `docs/reference.md` | Complete English public command and behavior reference |
| `assets/hard-build.svg` | Hard Build logo used by the README onboarding |
| `LICENSE` | MIT license |
| `Makefile` | Builds and checks the Go backend, delegates the integration suite, and installs the host wrapper, runtime bundle, and shell completions |
| `install.sh` | Latest portable-release installer and shell `PATH` and completion startup configuration |
| `hard.sh` | Installed public-command wrapper; selects the installed default, explicit host backend, a known container target, or an arbitrary `docker://image`, and keeps completion dispatch on the host |
| `hard.h` | Source runtime support header; host and image installations place it beside their backend |
| `format/format.v1` | Default clang-format style |
| `target/linux64/v2.0-ubuntu.22.04.Dockerfile` | Ubuntu 22.04 `linux/amd64` image installing the pinned hard v2.0 portable runtime |
| `target/linux64/v3.0-ubuntu.22.04.Dockerfile` | Ubuntu 22.04 `linux/amd64` image installing the pinned hard v3.0 portable runtime |
| `target/linux64/v3.0-alpine.3.22-static.Dockerfile` | Alpine 3.22 `linux/amd64` image building hard v3.0 against musl and producing fully static programs |
| `target/linux64/v4.0-glibc.2.35.Dockerfile` | Ubuntu 22.04 `linux/amd64` image building hard v4.0 against glibc 2.35 |
| `target/linux64/v4.0-musl.1.2.5-static.Dockerfile` | Alpine 3.22 `linux/amd64` image building hard v4.0 against musl 1.2.5 and producing fully static programs |
| `target/windows64/v4.0-llvm-mingw.20260616-ucrt.Dockerfile` | Ubuntu 22.04 `linux/amd64` image building hard v4.0 and producing Windows x86-64 UCRT executables with LLVM-MinGW 20260616 and Wine |
| `.github/workflows/check.yml` | Per-push and pull-request `make check` workflow |
| `.github/workflows/container.yml` | New-target GHCR publication workflow and immutable target tag policy |
| `.github/workflows/release.yml` | Release-tag portable host archive build, compatibility checks, and GitHub release publication |
| `unittest/Makefile` | Passes Make variables and an optional scenario name to the declarative Python runner |
| `unittest/run.py` | Discovers, validates, and sequentially executes strict `test.yaml` scenarios, with optional application suffix and runner settings for cross-target CI |
| `unittest/requirements.txt` | Pins the PyYAML major version used by the integration runner |
| `unittest/README.md` | Documents the YAML schema, commands, variables, and new-scenario workflow |
| Twelve `unittest/001.*` through `unittest/012.*` directories | Self-contained C and C++ source scenarios whose local `test.yaml` files own every command argument and expectation |
| `hard/go.mod`, `hard/go.sum` | Module identity, Go version, dependencies, and checksums |
| `hard/main.go` | Process entry, dispatch, configuration loading, and shared search progress |
| `hard/main_test.go` | Search-progress behavior and discovery integration for all commands |
| `hard/cli.go` | Cobra command tree, flags, positional paths, run arguments, test selectors, job normalization, and hidden completion generation |
| `hard/cli_test.go` | CLI defaults, validation, help, completion, interspersed flags, test selection, and job forms |
| `hard/version.go` | Embedded version components, formatting, and output |
| `hard/version_test.go` | Development and release version rendering and output failures |
| `hard/install_test.go` | Isolated portable installation, shell startup and completion files, idempotence, rollback, and failures |
| `hard/config.go` | `HARD_*` configuration and default compiler/linker flag vectors |
| `hard/config_test.go` | Configuration defaults, overrides, parsing, and failures |
| `hard/wrapper_test.go` | Host forwarding, target parsing, Docker arguments, mounts, and errors |
| `hard/source.go` | File classification, recursive discovery, symlink traversal, and deduplication |
| `hard/source_test.go` | Extensions, ordering, explicit paths, symlinks, cycles, and failures |
| `hard/progress.go` | Thread-safe normal, verbose, silent, color, and live-step output |
| `hard/progress_test.go` | Progress rendering, details, colors, padding, and unknown totals |
| `hard/format.go` | Style validation, parallel clang-format execution, and unified diffs |
| `hard/format_test.go` | Style containment, formatting, diff, output, and parallelism tests |
| `hard/clang.go` | Go-facing libclang analysis, arguments, normalization, diagnostics, and retry logic |
| `hard/clang_bridge.h`, `hard/clang_bridge.cc` | C ABI and C++ libclang 18 bridge |
| `hard/clang_test.go` | Includes, macros, conditional behavior, declarations, templates, and functions |
| `hard/environment.go` | Detailed source-independent build-environment diagnostics |
| `hard/environment_test.go` | Environment report, host-file parsing, unavailable probes, and output failures |
| `hard/github.go` | Repository mapping, synchronized downloads, extraction, aliases, and cache |
| `hard/github_test.go` | HTTP, extraction safety, aliases, caching, retries, and concurrency |
| `hard/library.go` | Strict recipe parsing, vendor source/build preparation, package fingerprinting, manifests, and reachable static-library metadata |
| `hard/library_test.go` | Recipe validation, CMake authority, fetch-only behavior, package reuse, and link-closure coverage |
| `hard/forward.go` | Source-context forward extraction, validation, rendering, path mapping, and atomic writes |
| `hard/forward_test.go` | Namespace/template output, translation-unit context, filtering, paths, and preservation |
| `hard/entry.go` | Configured global entry-function definition detection |
| `hard/entry_test.go` | Definitions, declarations, namespaces, macros, ambiguity, and empty config |
| `hard/build.go` | Dependency closure, support-header exclusion, compilation, link graph, and delivery |
| `hard/cache.go` | Content fingerprints, atomic artifact and parse-result records, semantic-result integrity, and file comparison |
| `hard/cache_test.go` | Artifact and parse cache keys, invalidation, no-cache, integrity, forward restoration, selector separation, and successful-test reuse |
| `hard/build_test.go` | Dependency, forwards, objects, entry binaries, output, and integrations |
| `hard/executable.go` | Generic executable suffix application and optional runner process vectors |
| `hard/executable_test.go` | Generic artifact naming, exact output behavior, and runner-backed run/test execution |
| `hard/run.go` | Build-compatible preparation, single-entry internal linking, live program execution, and exit propagation |
| `hard/run_test.go` | Run streams, arguments, exit status, internal-only artifacts, cache behavior, and entry validation |
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
- `github.com/spf13/cobra v1.10.2`;
- `go.yaml.in/yaml/v3 v3.0.5`.

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

The portable host release is `linux/amd64` and has a glibc 2.27 minimum. The
release workflow runs the build directly inside the pinned Ubuntu 18.04 image
`ubuntu:18.04@sha256:152dc042452c496007f07ca9127571cb9c29697f42acbfad72324b2bb2e43c98`;
there is intentionally no `target/host.Dockerfile`. It checksum-verifies and
uses the official LLVM 18.1.8 Ubuntu 18.04 archive, builds the CGO backend with
that Clang toolchain, bundles libclang, Clang resource headers, clang-format,
and Ubuntu 18's `libtinfo.so.5`, and gives the backend an executable-relative
RUNPATH. CI rejects GLIBC symbol requirements above 2.27 and smoke-tests the
result on Ubuntu 18.04, 22.04, and 24.04.

The glibc floor applies to launching the bundled backend and clang-format. It
does not make an old distribution's native compiler or C++ standard-library
headers satisfy hard's configured build flags. In particular, Ubuntu 18.04's
default GCC 7 does not implement the default C++20 contract; Docker remains the
recommended execution target there unless the user supplies a suitable host
toolchain.

The Ubuntu `linux64:v3.0-ubuntu.22.04` target installs hard from the official
`hard-v3.0.tar.gz` portable release rather than building the Go backend from
the current source tree. The Dockerfile pins SHA-256
`4a5d0227e80148684559d148be815cd6169f311fd0abe5b43ad2940b301e9fc1`,
requires the archive `VERSION` to equal `v3.0`, and records Git revision
`3826020ccc617f189521e5628e2ce5f8ecf82e00`, to which tag `v3.0` points.
There is no Go builder stage or apt.llvm.org repository in this image. The
older immutable `linux64:v2.0-ubuntu.22.04` target remains available by its
exact version.

The portable runtime supplies the backend, hard.h, format.v1, clang-format,
libclang 18.1.8, Clang resource headers, libtinfo compatibility library, and
licenses below `/usr/local/libexec/hard`. Ubuntu packages supply GCC 11,
glibc 2.35, GoogleTest 1.11.0, CMake 3.22.1, GNU Make, Meson, Ninja,
pkg-config, Autoconf, Automake, and Libtool. libgcc and libstdc++ are linked
statically into generated programs by the fixed linker flags, but glibc
remains dynamic.

The Alpine `linux64:v3.0-alpine.3.22-static` target checksum-verifies the
official v3.0 GitHub source archive, builds the Go backend natively against
musl and Alpine's system libclang 18, and copies that backend into an Alpine
3.22 runtime image. Its final package set includes the same compiler, formatter,
recipe-build, and test classes of tools as the Ubuntu target. Because Alpine's
packaged GoogleTest libraries are shared-only for linker purposes, the image
builds `libgtest.a` and `libgtest_main.a` from the packaged sources.
`HARD_LDFLAGS` adds `-static`, so generated C and C++ executables are fully
static; the hard backend itself remains a dynamically linked musl program.

The current `linux64:v4.0-glibc.2.35` target uses Ubuntu 22.04, downloads the
exact revision of Git tag `v4.0`, builds the backend against the apt.llvm.org
Jammy libclang 18 packages, and verifies glibc 2.35 in the final image. Its
compiler and recipe/test toolchain remain GCC 11, GoogleTest, CMake, GNU Make,
Meson/Ninja, pkg-config, Autoconf, Automake, and Libtool. Generated programs
link libgcc and libstdc++ statically while keeping glibc dynamic.

The current `linux64:v4.0-musl.1.2.5-static` target downloads the same exact
revision, builds the backend natively on Alpine 3.22 against packaged libclang
18, and verifies musl 1.2.5. It builds static GoogleTest archives from Alpine's
packaged sources and adds `-static` to project linker flags, so generated
programs have no ELF interpreter or shared-library dependencies.

The `windows64:v4.0-llvm-mingw.20260616-ucrt` target uses Ubuntu 22.04 and
builds the hard backend from the exact revision of Git tag `v4.0` against the
apt.llvm.org Jammy libclang 22.1.8 packages. It checksum-verifies the official
LLVM-MinGW 20260616 UCRT Ubuntu 22.04 archive with SHA-256
`534b92e067b22a6b4441f48ae9240a3341b17825d04d577eab0cf85c44b4deda`.
LLVM-MinGW supplies Clang/LLD/libc++ 22.1.8 and the Windows SDK. The image
cross-builds static GoogleTest archives, provides
`/opt/windows64/toolchain.cmake` to CMake recipes, installs the existing
`PyYAML>=6,<7` integration-test dependency, and includes Wine 64-bit. Project
compiler and linker flags include `-march=x86-64-v3 -mtune=generic`; linker
flags also include `-static`, which embeds the LLVM-MinGW C++ runtime but does
not remove Windows system and UCRT API-set DLL imports. Outputs are therefore
not described as fully static.

Runtime tools by command:

- `clang-format` for `format`;
- libclang 18 for dependency analysis in `fetch`, `build`, `run`, and `test`,
  forward extraction in `build`, `run`, and `test`, and entry detection in
  `build` and `run`;
- `HARD_CC` for object compilation and compiler-driver linking in `build`,
  `run`, and `test`, and as the authoritative CMake C++ compiler for an active
  compiled-library recipe;
- CMake for active compiled-library recipes in `build`, `run`, and `test`;
- `pkg-config` and `gtest_main` for `test`;
- HTTPS access to GitHub when a required repository is not cached.

## Build and installation

The repository-root Makefile has exactly these public targets:

- `all`, the default target, depends on `build`;
- `build` compiles the Go module in `hard/` to `BUILD_DIR/hard`;
- `check` enforces Go formatting, runs ordinary and race tests, vet, an
  isolated temporary build, module verification, both POSIX shell syntax
  checks, and staged and unstaged Git whitespace checks;
- `unittest` delegates to `unittest/Makefile` without depending on `build` or
  `install`, and therefore uses the existing `hard` command from `PATH` unless
  `HARD` is overridden;
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
├── libexec/hard/
│   ├── default-target               mode 0644, `host` for make install
│   ├── hard                         mode 0755, Go backend
│   ├── hard.h                       mode 0644
│   └── format/format.v1             mode 0644
└── share/
    ├── bash-completion/completions/hard
    │                                  mode 0644
    ├── zsh/site-functions/_hard       mode 0644
    ├── fish/vendor_completions.d/hard.fish
    │                                  mode 0644
    └── hard/                          persistent state root
```

`hard.sh` is a POSIX `sh` wrapper. It accepts explicit `host`, `linux64`,
`windows64`, `linux64:<tag>`, `windows64:<tag>`, and `docker://image` targets.
Known-repository tags use Docker's simple 128-character ASCII tag syntax; the
wrapper does not interpret version, libc, distribution, or toolchain
components. It resolves the installation prefix from
its own path only when the target is absent or explicit host execution is
selected. Without `--target`, it reads
`<prefix>/libexec/hard/default-target`, falling back to `host` when the file is
absent. Host mode prefixes the sibling runtime's `bin` directory to `PATH`,
uses `exec` on `<prefix>/libexec/hard/hard`, and passes the remaining argument
vector with `"$@"`. Container target mode removes the wrapper option and uses
`exec docker run`; the container image entrypoint runs its backend, and the
wrapper does not resolve a host runtime. Empty, duplicate, legacy, malformed,
and unknown targets are errors.
Values after `--` remain untouched.

For the private Cobra `__complete` and `__completeNoDesc` protocols, `hard.sh`
answers a final partial `--target` value itself. It routes every other
completion request to the sibling host backend before normal target parsing.
Completion therefore never starts or pulls Docker even when `default-target`
selects a container target.

The host `HARD_ROOT`, or `$HOME/.local/share/hard` when it is empty, is the
source of a bind mount targeting `/hard`. The physical current working
directory is mounted at the same absolute container path and becomes the
container workdir. Docker uses `--rm`, stdin forwarding, and the current
numeric UID:GID. Mutable `linux64` and `windows64` targets use `--pull=always`;
explicit tagged and arbitrary-image targets use `--pull=missing`. No host
`HARD_*` value is forwarded into the container. A relative host `HARD_ROOT` is
made absolute below the current working directory. Only the working directory
and persistent root are mounted.

Any prefix with sibling `bin/hard` and `libexec/hard` paths is a supported
wrapper layout. Changing `PREFIX` or staging through `DESTDIR` preserves the
relative lookup without rewriting the wrapper. `make install` remains
host-only and neither invokes Docker nor installs image assets.

`make install` generates Bash, Zsh, and Fish completion files from the built
backend and installs them into their standard `PREFIX/share` directories
shown above.

`DESTDIR` prepends a staging root to every installed path but does not become
part of the logical prefix. This permits packaging tests without writing into
the user's home or a system directory. There are intentionally no `clean` or
`uninstall` targets in the current scope.

### Portable host release and installer

A release Git tag `vX.Y` runs `.github/workflows/release.yml`. It creates
versioned release assets named `hard-vX.Y.tar.gz` and
`hard-vX.Y.tar.gz.sha256`, smoke-tests them, uploads them as workflow artifacts,
and creates or updates the matching GitHub release. The installer resolves the
latest redirect to its strict `vX.Y` tag and downloads assets from that exact
release. The archive has one top-level `hard-linux-amd64` directory and a
relocatable `bin/`, `libexec/hard/`, and `share/` layout. The top-level
`bin/hard` runs that sibling runtime directly after extraction without
installation. Its runtime includes the Go backend, `hard.h`, `format.v1`,
clang-format, libclang, LLVM resource headers, the required libtinfo
compatibility library and licenses. The version is embedded in the backend;
the `share/` tree contains the generated Bash, Zsh, and Fish completion
files.

`install.sh` is POSIX `sh`, accepts no arguments, and supports Linux x86-64
only. It downloads the latest stable archive and checksum from GitHub Releases,
verifies the checksum before changing the installation, and stages replacement
of the wrapper and runtime below `$HOME/.local`. A failed runtime replacement
restores the previous runtime. The installer creates `$HOME/.local/share/hard`
but keeps every runtime asset under `$HOME/.local/libexec/hard`. It leaves
`default-target` absent, so wrapper fallback selects `host`.

Its user-facing output consists of eight named stages covering compatibility,
release resolution, both downloads, checksum verification, bundle extraction,
installation, and shell configuration. Download stages show their URL and use
curl's progress bar. Styling is limited to terminal stdout when `NO_COLOR` is
unset; noninteractive output contains no ANSI escapes. A successful run ends
with an informational `Next steps` section listing minimal C++20 compiler
commands for Ubuntu/Debian, Arch/CachyOS, Fedora/RHEL/Rocky, openSUSE, and
the Docker target for Alpine. The native compatibility floors shown are Ubuntu
22.04, Debian 12, RHEL 9, and Rocky Linux 9; the openSUSE command is for
Tumbleweed. Alpine is called out as musl-based and incompatible with the
portable glibc host runtime. The section then shows `c++ --version`,
`hard build example.cpp`, and the Docker alternative
`hard --target=linux64 build example.cpp`. These commands are printed only
and are never executed by the installer.

The installer does not inspect the distribution, invoke `sudo`, install system
packages, start Docker, or change group membership. Native compiler, testing,
recipe-build, and Docker prerequisites remain the user's responsibility.
Explicit `--target=linux64` and versioned `linux64` targets remain available
when Docker is installed.

After installation, the script prepends `$HOME/.local/bin` to its own `PATH`
when absent. According to the basename of `$SHELL`, it idempotently records the
same path in `$HOME/.bashrc`, `$HOME/.zshrc`,
`$HOME/.config/fish/config.fish`, or the fallback `$HOME/.profile`. Bash, Zsh,
and POSIX entries use `export PATH="$HOME/.local/bin:$PATH"`; Fish uses
`fish_add_path "$HOME/.local/bin"`. Existing literal, tilde-based, or expanded
entries are not duplicated. A piped shell cannot change its parent process, so
the installer prints the appropriate command when the invoking environment did
not already contain the path.

The archive and installer place completions at the standard
`share/bash-completion/completions/hard`,
`share/zsh/site-functions/_hard`, and
`share/fish/vendor_completions.d/hard.fish` paths. For the shell named by
`$SHELL`, the installer idempotently sources the Bash script from `.bashrc`
or initializes Zsh completion and sources `_hard` from `.zshrc`. Fish
discovers its vendor file automatically. The POSIX fallback receives only the
`PATH` configuration; PowerShell is outside the current scope.

### Container image and publication

The wrapper maps its container target forms as follows:

    --target=linux64                         ghcr.io/hard-build/linux64:latest
    --target=linux64:v4.0-glibc.2.35         ghcr.io/hard-build/linux64:v4.0-glibc.2.35
    --target=linux64:v4.0-musl.1.2.5-static  ghcr.io/hard-build/linux64:v4.0-musl.1.2.5-static
    --target=linux64:v3.0-ubuntu.22.04       ghcr.io/hard-build/linux64:v3.0-ubuntu.22.04
    --target=linux64:v3.0-alpine.3.22-static ghcr.io/hard-build/linux64:v3.0-alpine.3.22-static
    --target=windows64                       ghcr.io/hard-build/windows64:latest
    --target=windows64:v4.0-llvm-mingw.20260616-ucrt
                                             ghcr.io/hard-build/windows64:v4.0-llvm-mingw.20260616-ucrt
    --target=docker://registry/name:tag       registry/name:tag

The older exact `linux64:v2.0-ubuntu.22.04` target remains valid and maps to
the same tag below `ghcr.io/hard-build/linux64`.

Mutable `linux64` and `windows64` aliases are pulled before every run and
remain their newest glibc and LLVM-MinGW UCRT images respectively. Explicit
tag targets are pulled only when missing locally. Published version tags remain
available for reproducible builds and persistent caches.

The wrapper accepts any `linux64:<tag>` or `windows64:<tag>` whose Docker tag
is non-empty, no longer than 128 characters, begins with an ASCII alphanumeric
or underscore, and otherwise contains only ASCII alphanumerics, underscores,
dots, and hyphens. This is syntax validation rather than a published-image
whitelist; registry availability remains Docker's responsibility.

The older v3.0 Ubuntu image installs the official `hard-v3.0.tar.gz` portable
release,
whose backend corresponds to Git tag `v3.0` at
`3826020ccc617f189521e5628e2ce5f8ecf82e00`. The archive checksum is pinned to
`4a5d0227e80148684559d148be815cd6169f311fd0abe5b43ad2940b301e9fc1`.
Its runtime bundle is:

    /usr/local/libexec/hard/hard
    /usr/local/libexec/hard/hard.h
    /usr/local/libexec/hard/format/format.v1
    /usr/local/libexec/hard/bin/clang-format
    /usr/local/libexec/hard/lib/
    /usr/local/libexec/hard/VERSION

The Docker build verifies both the release checksum and the bundled `VERSION`.
It does not use `curl ... | sh`: the interactive latest-release installer cannot
pin `v3.0`, mutates shell configuration, and installs below `$HOME/.local`.
Installing the versioned archive directly gives the image an immutable input and
fails the build if either the archive or expected version changes.

The current glibc image's fixed target environment is:

    HARD_ROOT=/hard
    HARD_ENV=linux64:v4.0-glibc.2.35
    HARD_CC=c++
    HARD_CFLAGS=-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra
    HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra -static-libgcc -static-libstdc++
    HARD_ENTRYPOINTS=main _start

The current musl image uses the same root, compiler, entrypoints, and compiler
flags, with these target-specific values:

    HARD_ENV=linux64:v4.0-musl.1.2.5-static
    HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra -static -static-libgcc -static-libstdc++

It builds hard v4.0 from the exact revision supplied by CI, uses Alpine's
system libclang resource directory, and verifies musl 1.2.5.

The older Ubuntu image's fixed target environment is:

    HARD_ROOT=/hard
    HARD_ENV=linux64:v3.0-ubuntu.22.04
    HARD_CC=c++
    HARD_CFLAGS=-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra
    HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra -static-libgcc -static-libstdc++
    HARD_ENTRYPOINTS=main _start

The older Alpine image uses the same root, compiler, entrypoints, and compiler
flags, with these target-specific values:

    HARD_ENV=linux64:v3.0-alpine.3.22-static
    HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra -static -static-libgcc -static-libstdc++

It checksum-verifies the official v3.0 source archive at
`ee24cbeec82087f31a0c07d7a346f85c0f3d5b36fd25199a90ebb69c1e1bee35`,
builds hard natively against musl, and uses Alpine's system libclang resource
directory.

The Windows image builds the exact hard v4.0 Git revision supplied by the
workflow, installs LLVM-MinGW 20260616 UCRT and Wine on Ubuntu 22.04, and owns
this fixed environment:

    HARD_ROOT=/hard
    HARD_ENV=windows64:v4.0-llvm-mingw.20260616-ucrt
    HARD_CC=x86_64-w64-mingw32-clang++
    HARD_CFLAGS=-std=c++20 --target=x86_64-w64-mingw32 --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++ -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra
    HARD_LDFLAGS=-std=c++20 --target=x86_64-w64-mingw32 --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++ -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra -static
    HARD_ENTRYPOINTS=main _start
    HARD_EXECUTABLE_SUFFIX=.exe
    HARD_EXECUTABLE_RUNNER=wine
    CMAKE_TOOLCHAIN_FILE=/opt/windows64/toolchain.cmake
    PKG_CONFIG_LIBDIR=/opt/windows64/lib/pkgconfig

The generic executable variables make inferred application and test binaries
use `.exe` and make `hard run` and `hard test` start them through Wine. The
backend does not infer either behavior from `HARD_ENV`. Vendor builds continue to
receive the authoritative `HARD_CC` through CMake but not project
`HARD_CFLAGS` or `HARD_LDFLAGS`; the image-owned `CMAKE_TOOLCHAIN_FILE` carries
the cross-compilation configuration.

The backend scans immediate version directories below
`<runtime-root>/lib/clang` and appends the single matching `include` directory
as `-idirafter` only to libclang argument vectors. The configured and
compiler-effective `HARD_CFLAGS` do not contain this internal parser argument.
No matching directory is normal for native libclang; multiple matches are an
error.

The images are intentionally `linux/amd64` only, and generated programs require
an x86-64-v3 CPU. Docker and Wine do not add CPU emulation. Toolchain, ABI,
base system, bundled hard version, or minimum-CPU changes require a new target
version.

The GHCR workflow is independent from release publication. On an automatic
push to `main`, it considers only newly added
`target/<image>/<version>.Dockerfile` paths. It validates the directory and
filename, requires the `vX.Y` prefix to name an existing Git tag, and records
that tag's revision in the image. A path that occurred earlier in Git history is
skipped, even if deleted and re-added. A newly added Dockerfile publishes an
immutable version tag. A newly added glibc Dockerfile can advance
`linux64:latest` only when it is newest across the glibc and legacy Ubuntu
lineage. A newly added
LLVM-MinGW UCRT Dockerfile can similarly advance `windows64:latest`. Legacy
Alpine and current musl static images never change an unversioned target.
Changing or deleting an existing Dockerfile, and ordinary pushes without a new
Dockerfile, publish no image.
An explicit `workflow_dispatch` is available only for recovering a failed
first publication. Its required `dockerfile` input must name a tracked regular
target Dockerfile on `main`. Manual recovery bypasses only the prior-history
skip and retains the same name, version, and release-tag validation. After
logging in, it refuses to continue when the immutable version tag already
exists. Only the exact `manifest unknown` response is treated as an unpublished
tag; authentication, network, and other registry failures remain fatal.
Commit, edge, and release-only tags are not published. Before publication, the
workflow loads the new image on the runner, builds and executes a C++20 smoke
program, and checks static targets for both an ELF interpreter and `NEEDED`
entries. Windows smoke requires AMD64 PE and UCRT contract imports, executes
the program through `hard run` and Wine, and runs all 12 declarative integration
scenarios inside the image. Only a successful image is pushed. New Dockerfiles
do not carry that smoke program as an image layer.

The first externally published GitHub package defaults private and must be made
public once by a maintainer before anonymous pulls work. Package visibility
remains an external GitHub organization setting and is not managed by the
workflow.

## Exact public CLI

The public command forms are:

    hard version
    hard environment
    hard format [--format=<name>] [-s|--silent] [path...]
    hard build  [--no-cache] [-s|--silent] [-o <path>] [path...]
    hard fetch  [-s|--silent] [path...]
    hard run    [--no-cache] [-s|--silent] [path...]
                [-- program-argument...]
    hard test   [--list-tests] [--test=<selector>]...
                [--no-cache] [-s|--silent] [path...]

The installed wrapper additionally accepts `--target=host`,
`--target=linux64`, `--target=linux64:<tag>`, `--target=windows64`,
`--target=windows64:<tag>`,
`--target=docker://image`, or their separate-value forms anywhere before `--`.
This selects how one of the same seven public commands is executed; it does not
add another public command. The
Go help template documents the wrapper option, while the wrapper owns its
parsing, installed default, and Docker behavior.

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
- every source-processing command: `-s`, `--silent`;
- `build`: `-o <path>`, `--output=<path>`;
- `build`, `run`, and `test`: `--no-cache`;
- `test`: `--list-tests` or repeatable `--test=<selector>`, which are
  mutually exclusive;
- `fetch`, `run`, and `test` do not accept `--format` or `--output`.

Other CLI decisions:

- each source-processing command accepts zero or more paths;
- no path for a source-processing command becomes `.`;
- `version` accepts no paths, prints the embedded version, and performs no
  runtime-root resolution, configuration loading, toolchain probing, or source
  discovery;
- `environment` accepts no paths, performs no source discovery, and prints its
  detailed report directly;
- for `run` only, `--` ends hard arguments; later values populate the program
  argument vector unchanged, and no preceding path still defaults to `.`;
- invoking no command is an error: `a command is required`;
- unknown commands and flags are errors;
- Cobra's `completion` generator is hidden from help but can generate Bash,
  Zsh, and Fish scripts for installation and release packaging;
- dynamic completion lists only the seven public commands and filters Cobra's
  private `_help` suggestion;
- the wrapper supplies fixed `host`, `linux64`, `windows64`,
  `linux64:v4.0-glibc.2.35`, `linux64:v4.0-musl.1.2.5-static`,
  `linux64:v3.0-ubuntu.22.04`, `linux64:v3.0-alpine.3.22-static`, and
  `windows64:v4.0-llvm-mingw.20260616-ucrt`, and `docker://` values for `--target`; the backend
  supplies `format.v1`, flag names, commands, and default filesystem-path
  completion;
- the normal `help` command is not public; `help` and `_help` are rejected;
- root and command `--help` succeed without configuration loading or source
  discovery;
- root help exposes only `version`, `environment`, `format`, `fetch`,
  `build`, `run`, and `test`.

The parsed `arguments` value contains `command`, `paths`, `programArguments`,
`verbose`, `silent`, `noColor`, `noCache`, `listTests`, `testSelectors`,
`jobs`, `format`, and `output`. Only `build` populates `output`; only `run`
populates `programArguments`; only `build`, `run`, and `test` can set
`noCache`; only `test` populates listing or selectors. The raw output
spelling preserves a trailing path separator because that separator declares
directory intent.

## Configuration contract

Configuration is loaded after successful CLI parsing and before source
discovery. It is held in an unexported configuration value rather than mutable
package globals. Before configuration loading, the Go backend resolves its own
physical executable path through symlinks; the directory containing that
executable is the runtime root. This is an internal path, not another
environment variable.

Environment variable names are ASCII:

- `HARD_ROOT`;
- `HARD_ENV`;
- `HARD_CC`;
- `HARD_CFLAGS`;
- `HARD_LDFLAGS`;
- `HARD_ENTRYPOINTS`;
- `HARD_EXECUTABLE_SUFFIX`;
- `HARD_EXECUTABLE_RUNNER`.

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
  and user-provided system-include trees such as `-isystem` or `-idirafter`;
- must change when that system state changes because system headers are not
  content-hashed by parse or object caches;
- does not select an executable suffix or runner;
- artifact path construction rejects values escaping `HARD_ROOT/env`, such as
  `../outside`.

### `HARD_CC`

- unset or empty: `c++`;
- non-empty: one executable name or path;
- used for object compilation and compiler-driver linking in `build`, `run`,
  and `test`;
- resolved and passed authoritatively as `CMAKE_CXX_COMPILER` when those
  commands prepare an active compiled-library package;
- not used for dependency discovery or by `fetch`.

### `HARD_CFLAGS`

The configured default vector is toolchain-only:

    -std=c++20
    -O3
    -flto=auto
    -Wall
    -Wextra

A non-empty explicit value is parsed with `go-shellwords` and replaces that
complete configured vector. Quoting works, but environment expansion and
backtick execution are disabled. Malformed quoting names `HARD_CFLAGS` in the
error. A present but empty value supplies no configured compiler flags.

The backend always appends these hard-managed arguments afterward:

    -I<HARD_ROOT>/source
    -include
    <runtime-root>/hard.h

They remain present even when configured `HARD_CFLAGS` is empty. Active
compiled-library include directories are additionally appended per translation
unit after recipe discovery. The resulting effective vector is used by
libclang dependency, source-forward, and entry analyses and by `HARD_CC` object
compilation. Dependency and entry analysis add the working directory and
default to C++ mode unless `-x` is already present. Vendor CMake builds do not
receive `HARD_CFLAGS`.

For libclang only, hard scans immediate version directories below
`<runtime-root>/lib/clang` for `include`. When exactly one exists, it appends
`-idirafter` and that directory to the analysis argument vector. The
compiler-effective vector is unchanged, so verbose compiler commands never
contain this runtime-only resource path. No matching directory is normal for
native libclang installations. A present non-directory or inaccessible path
and multiple matching resource directories are errors.

Build, run, and test canonicalize `<runtime-root>/hard.h` through symlinks and
exclude declarations physically owned by that canonical target from source
forwards. The backend-managed force include remains. Consequences:

- libclang and the compiler still see the runtime support header;
- it receives no standalone `Parsing` step or per-header forward output;
- each source still receives its own generated forward include;
- unrelated project or external headers also named `hard.h` remain managed;
- old header-specific forward artifacts are not deleted automatically;
- failure to resolve the runtime support path adds no new filter-specific
  error; normal parser/compiler handling remains authoritative.

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
  source preparation and object compilation and making run fail with no entry
  source;
- non-empty: parsed with the same shell-word and disabled-expansion rules;
- only matching global function definitions make an entry source;
- `test` ignores this variable because `gtest_main` supplies its entry point.

### `HARD_EXECUTABLE_SUFFIX`

- unset or empty: inferred executable paths are extensionless;
- non-empty: must start with `.` and contain neither `/` nor `\\`;
- appended to inferred internal application and test binaries, default build
  delivery paths, and paths inferred below a directory `-o`;
- not duplicated when the inferred path already has the exact suffix;
- does not rewrite an exact file destination supplied through `-o`;
- retained as one literal value without shell-word parsing.

### `HARD_EXECUTABLE_RUNNER`

- unset or empty: run and test binaries execute directly;
- non-empty: one executable name or path, without shell-word parsing;
- prefixes the binary and arguments for `hard run`, GoogleTest listing, and
  GoogleTest execution;
- participates in successful-test cache fingerprints;
- is independent of `HARD_ENV` and of the executable filename.

## Source selection

Recognized files:

| Command | Files |
| --- | --- |
| `build` | `.c`, `.cc`, `.cpp`, `.c++`, excluding test sources |
| `run` | Same as `build` |
| `fetch` | `.c`, `.cc`, `.cpp`, `.c++`, including test sources |
| `format` | build extensions plus `.h`, `.hh`, `.hpp`, `.h++` |
| `test` | test sources using `.c`, `.cc`, `.cpp`, or `.c++` |

Extensions are case-insensitive. A test source has a stem ending in `.test`
or legacy `_test`, also case-insensitive. Both `source.TeSt.CPP` and
`source_TeSt.CPP` are therefore tests, excluded by `build` and `run` and
included by `fetch`, `format`, and `test`.

Not recognized unless a future requirement changes the set:

    .cxx .hxx .cu .cuh .inl .ipp .tpp

Path rules:

- with no path, recursively scan `.`;
- with paths, root selection includes only explicit eligible files and
  eligible files recursively found under explicit directories;
- build, fetch, run, and test may subsequently add same-stem production
  sources required by dependencies; fetch only analyzes them;
- unsupported explicit files are silently skipped;
- missing or inaccessible inputs are errors;
- no matches is a successful no-op except for `run`, which requires exactly
  one root entry source;
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

Every source-processing command creates one thread-safe progress object before
source selection and starts with `Searching source files`. `hard version` and
`hard environment` do not create a progress object or search for sources.

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
- `run`: search, parsing, and downloads reuse preparation step one; total
  becomes `1 + sources + 1 link`; execution begins only after the progress
  stream is finished and is not a progress or cache step;
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
- run: `Searching source files`, `Parsing ...`, `Downloading ...`,
  `Compiling ...`, `Linking ...`; there is no `Copying` label;
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

and cause status 1. A normally started program that exits nonzero under
`hard run` instead propagates its exit status without adding a top-level
`hard:` diagnostic.

## `hard version`

`version` prints one line assembled from two values embedded in the Go binary:

    versionNumber = 4.0
    versionPrerelease = development

The default output is `v4.0-development`. A non-empty prerelease identifier is
separated from the version number by one hyphen. Release packaging clears only
the prerelease value through `-X main.versionPrerelease=`, producing `v4.0`,
and rejects a binary whose reported version differs from the release tag.

The command does not resolve the runtime root, read a runtime version file,
load `HARD_*` configuration, probe the toolchain, create progress, discover
sources, or create artifacts. New portable releases no longer contain a
runtime `VERSION` file. Older immutable release archives and container images
retain their historical files because their tagged backends predate this
command.

## `hard environment`

`environment` is a detailed human-readable diagnostic command. It accepts no
paths and does not discover sources, initialize caches, or create artifacts.
After ordinary configuration and runtime-root loading it reports:

- the embedded hard version;
- the resolved backend executable, runtime root, `HARD_ENV`, and `HARD_ROOT`;
- `/etc/os-release` identity, `uname` kernel and architecture, the first CPU
  model in `/proc/cpuinfo`, logical CPU count, and libc from `getconf` with
  `ldd --version` fallback;
- configured `HARD_CC`, resolved compiler path, first `--version` line, and
  `-dumpmachine` target;
- `HARD_EXECUTABLE_SUFFIX` and `HARD_EXECUTABLE_RUNNER`, displaying empty
  values as `none` and `direct`;
- shell-rendered configured CFLAGS, LDFLAGS, and entry points, with one argument
  per line; CFLAGS omits the backend-managed source-root include and runtime
  support-header force include;
- `clang_getClangVersion()` and the shared portable resource-directory
  discovery result, with no runtime directory reported as `system default`.

Individual unavailable host/compiler probes render `unavailable` and do not
abort the remaining report. Invalid configuration and output-write failures
remain fatal. The report encloses its bold cyan title between dim cyan rules
and uses bold green section headings, cyan labels, and yellow special values by
default. `--no-color` preserves the aligned layout without ANSI sequences. The
command intentionally has no `--silent` option.

## `hard format`

Format selects ordinary sources, test sources, and supported headers. It does
not print a preliminary source list and performs no libclang `Parsing` stage.

`--format` defaults to `format.v1`. The value is relative to:

    <runtime-root>/format

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
after considering using libclang only for forward declarations. Build, run,
and test reuse the final dependency-analysis AST for source-context forward
extraction.

Dependency translation units use detailed preprocessing records, keep-going,
and skipped function bodies. They receive the source absolute path, the
effective compiler flags, `-working-directory`, and C++ mode unless the
configured flags already contain `-x`. Active package include directories
participate after recipe discovery.

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
Paths marked system by libclang, including user-provided `-isystem` and
`-idirafter` directories, are absent from persistent dependency snapshots.

Unresolved includes are preserved with diagnostics. Only actionable GitHub or
well-known paths trigger a snapshot request. Other missing includes retain the
original libclang error and never cause arbitrary network access.

### Persistent parse-result cache

Successful source analysis in `build`, `run`, and `test` writes a versioned
`<source>.hard-parse-cache.json` record below the mirrored
`HARD_ROOT/env/HARD_ENV/build` path. Each record contains the managed
dependency list, complete active non-system dependency snapshot, active library
recipe header paths, detected entry point, and final validated source-forward
text. A separate result digest protects these semantic fields from valid-JSON
corruption. After that checksum is validated, a prospective hit restores the
packages and per-source package include flags before its action fingerprint is
validated against current inputs.

The action fingerprint contains the current `hard` executable digest,
`clang_getClangVersion()` value, ordered compiler flags, configured entry names
where relevant, and content digests for the input plus every non-system
dependency recorded by the prior successful analysis. The invocation working
directory is included only when the effective compiler arguments contain a
relative path or an opaque driver argument whose semantics can depend on it.
With cwd-independent arguments, one absolute source can reuse its parse result
when selected first from a parent directory and then from its own directory.
Paths are canonicalized, deduplicated, and sorted by the common action-key
code. Missing, malformed, version-mismatched, changed, or
result-digest-mismatched records are misses. A missing dependency is also a
miss and the real parser remains authoritative. System state is intentionally
represented only by the environment-specific artifact tree.

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

The current well-known mappings are:

    hard/<path> -> github.com/hard-build/library/<path>
    recipe/<path> -> github.com/hard-build/recipe/<path>

They create relative aliases:

    HARD_ROOT/source/hard -> github.com/hard-build/library
    HARD_ROOT/source/recipe -> github.com/hard-build/recipe

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
fetch, run, or test invocation. Concurrent demand for one repository produces one
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

The runtime support header reached through `<runtime-root>/hard.h` remains
force-included, but declarations physically owned by its canonical target are
excluded from every source forward. Other project or external headers named
`hard.h` are treated normally.

## `hard build`

Build root selection excludes `*.test.*` and legacy `*_test.*`. For a
non-empty selection, the implemented pipeline is:

1. discover dependencies, prepare active compiled-library packages, and detect
   configured entry definitions;
2. recursively add same-stem production sources for managed headers;
3. generate one source-context forward for each translation unit while
   filtering declarations from the canonical runtime support header;
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

    HARD_CC <configured-HARD_CFLAGS...> \
      -I<HARD_ROOT>/source \
      -include <runtime-root>/hard.h \
      <active-library-include-flags...> \
      -include <source.cpp.fwd.h> \
      -c <absolute-source> -o <object>

The generated forward `-include` pair follows the complete per-source effective
flags and precedes `-c`. A source with no eligible declarations still gets an
output containing `#pragma once`. The source-root and runtime-header includes
are backend-managed rather than part of configured `HARD_CFLAGS`. The compiler
retains the invocation working directory so relative user flags keep their
existing meaning, but the source argument is always the lexical absolute path.

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
digest, compiler arguments with the absolute source, source, every active
resolved non-system include, and generated source forward. The invocation
working directory participates only when compiler arguments are relative or
opaque and therefore potentially cwd-dependent. For example, `-I.` deliberately
keeps two invocation directories separate, while only absolute/default flags
permit cross-directory reuse. `HARD_ENV` represents the system headers and
immutable toolchain state. Cache lookup verifies the sidecar and current
regular object digest. Missing, malformed, changed, or non-regular records and
artifacts are misses. `--no-cache` disables reads, invalidates the old record
before compilation, and stores a fresh record only after success.

### Entry-point detection

Every root and automatically discovered source is separately parsed as a full
translation unit, without skipped function bodies, using its effective compiler
flags.

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

    HARD_CC <entry-and-dependency-objects...> \
      <reachable-static-library-archives...> <HARD_LDFLAGS...> \
      -o <internal-binary>

Do not add `-nostartfiles`, `-e`, or a custom entry linker option solely because
`HARD_ENTRYPOINTS` contains `_start` or another name. Such entries must be
compatible with ordinary linking and the supplied flags or receive the normal
linker error.

Internal binaries remove the entry source extension while preserving the
mirrored lexical absolute path:

    /home/user/project/src/application.cpp
      -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/application

Hard appends `HARD_EXECUTABLE_SUFFIX` to the inferred internal path. An exact
suffix already present at the end is not duplicated. `HARD_ENV` does not
select this behavior.

Internal collisions, including same-directory `application.c` and
`application.cpp`, are errors. A source with an empty stem cannot create a
binary. Link jobs use the resolved worker count. Compiler exit failures are
aggregated; a start failure stops new scheduling. Verbose mode prints the exact
shell-escaped link command immediately after the relevant `Linking` entry.

Successful links use the same sidecar suffix beside the internal binary. Their
fingerprint contains the `hard` and compiler fingerprints, link arguments, and
every object digest. The invocation working directory is retained only for
relative or opaque cwd-dependent linker flags. The binary digest is verified
on hit. A failed or forced link cannot retain an eligible older record because
the sidecar is invalidated before the compiler is started.

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

No-`-o` delivery and directory output append `HARD_EXECUTABLE_SUFFIX` to the
inferred destination. A file destination supplied with `-o path` remains exact
and is never rewritten.

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

## `hard run`

Run root selection is identical to build and excludes `*.test.*` and legacy
`*_test.*`. It reuses the build analyzer, recursive same-stem source closure,
source forwards, object compilation, dependency-object traversal, content fingerprints, and ordinary
compiler-driver linking.

Exactly one originally selected root must define a configured entry function.
Zero is an error. Multiple roots are an error that lists their lexical source
paths. The check is performed after preparation but before object compilation.
Automatically discovered sources remain dependency-only. The linked output is
the ordinary internal binary below `HARD_ROOT/env/HARD_ENV/build`; run never
copies it beside the source and has no `-o` flag.

The syntax boundary is:

    hard run [hard flags] [path...] -- [program arguments...]

Cobra's `ArgsLenAtDash` divides the positional vector. Bare `--` with no path
retains the ordinary `.` default. Job-argument normalization stops at `--`, so
program values such as `-j` and `--no-cache` are not rewritten or consumed.

The program is started after a successful link with:

- the invocation's current working directory;
- stdin, stdout, and stderr inherited from the hard process;
- every post-`--` argument unchanged.

The internal binary uses `HARD_EXECUTABLE_SUFFIX`. A non-empty
`HARD_EXECUTABLE_RUNNER` produces the process vector
`<runner> <binary> <program arguments...>`; an empty runner executes the binary
directly. Neither choice depends on `HARD_ENV`.

Program output is live rather than captured. Verbose mode prints the
shell-escaped run command after build progress finishes. Silent mode hides only
hard progress and commands; it does not suppress child streams.
`--no-color` has no effect on the child.

Parse, compile, and link caches behave exactly as in build. `--no-cache` forces
those stages and refreshes their successful records. Program execution has no
result cache and always occurs after a successful cached or uncached build.
The progress total is `1 + compiled sources + 1 link`, with no delivery step.

A normal child exit code is propagated by the top-level process without a
duplicate diagnostic. A signal-style negative `exec.ExitError` code maps to
status 1. Process-start failures and all build failures use the ordinary
`hard: <error>` path and status 1.

## `hard fetch`

Fetch never reads or writes the environment-backed persistent parse-result
cache; its parsing behavior and absence of `HARD_ROOT/env` artifacts remain
unchanged.
Fetch selects all supported translation units, including `*.test.*` and
legacy `*_test.*`, then uses the same backend-effective base compiler flags,
libclang analysis, recursive same-stem source closure, GitHub recovery, well-known mapping, cache, and worker
limit as build. Active recipes add their declared source-tree include
directories for analysis only.

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

Test root selection includes case-insensitive `*.test.c`, `*.test.cc`,
`*.test.cpp`, and `*.test.c++`. Legacy `*_test.*` names with the same source
extensions remain supported.

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

GoogleTest compiler flags are appended after the backend-effective base compiler
flags; active per-source package includes follow recipe discovery. GoogleTest
linker flags are appended after `HARD_LDFLAGS`.

For each selected test root:

1. compute the recursive production dependency closure with entry detection
   disabled;
2. exclude other `*.test.*` and legacy `*_test.*` sources from automatic
   implementation discovery;
3. use one shared GitHub resolver across every test plan;
4. generate one source-context forward per translation unit while excluding
   declarations from the canonical runtime support header;
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
      <reachable-static-library-archives...> \
      <HARD_LDFLAGS...> <gtest_main-linker-flags...> \
      -o <internal-test-binary>

`HARD_ENTRYPOINTS` is ignored. A test that defines an incompatible `main`
receives the normal linker failure. Test binaries use the same mirrored
internal binary path rule, remain in the environment build tree, and are never
copied into the project. Test does not accept `-o`.

Test binaries use `HARD_EXECUTABLE_SUFFIX`; listing and testing use
`HARD_EXECUTABLE_RUNNER`. The successful-test cache remains separated by
`HARD_ENV`, includes the resulting binary path and digest, and also fingerprints
the runner.

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

The implemented installed host layout is:

    <host-runtime-root>/
    ├── default-target
    ├── hard
    ├── hard.h
    └── format/
        └── format.v1

    HARD_ROOT/
    ├── source/
    │   ├── hard -> github.com/hard-build/library
    │   ├── recipe -> github.com/hard-build/recipe
    │   └── github.com/
    │       └── <owner>/
    │           └── <repository>/
    └── env/
        └── HARD_ENV/
            ├── build/
            │   └── <absolute path without leading slash>/
            │       ├── application
            │       ├── application.hard-cache.json
            │       ├── application.hard-test-cache.json
            │       ├── file.cpp.hard-parse-cache.json
            │       ├── file.cpp.fwd.h
            │       ├── file.cpp.o
            │       └── file.cpp.o.hard-cache.json
            └── library/
                └── github.com/<owner>/<repository>/<fingerprint>/
                    ├── build/
                    ├── install/
                    └── manifest.json

External repository snapshots are shared by all environments below one
`HARD_ROOT`. Environment build artifacts are isolated by `HARD_ENV`.

`hard.h` and format styles are runtime inputs, not generated by the Go program.
Their repository sources are `/home/taitov/projects/hard-build/hard/hard.h`
and `/home/taitov/projects/hard-build/hard/format/`. `make install` copies the
host backend, header, and format into `PREFIX/libexec/hard/`. The container
image carries its independent runtime bundle below `/usr/local/libexec/hard/`.
Neither installation stores runtime files below `HARD_ROOT`.

The portable host bundle additionally owns `bin/clang-format`, `lib/` with
libclang, LLVM resource headers, and libtinfo, and license records. The version
is embedded in the backend. These files move as one runtime and are not
supplied by `make install`.
The container runtime has no `default-target` because target selection belongs
to the host wrapper.

Forward, object, and binary paths use lexical absolute source paths. They all
reject an environment name that escapes `HARD_ROOT/env`.

## Integration fixtures and external examples

The repository contains a self-contained positive integration suite below
`unittest/`. It has twelve scenarios, fifteen application entry points, eight
GoogleTest translation units, and fifteen GoogleTest cases. The scenarios cover
a minimal application, multiple entries sharing an object, transitive
implementation discovery, cyclic headers and implementation graphs, a
header-only template, ordinary GoogleTest production dependencies, a compiled
TinyXML2 package linked statically from an embedded recipe, the same package
resolved through the well-known recipe repository, an object shared by two test
plans, all supported source extensions, equal binary basenames in different
directories, and the force-included runtime support header.

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

The repository-root `unittest` target delegates directly to
`unittest/Makefile`, which only passes configuration variables and an optional
scenario name to the runner. Neither Makefile contains a scenario directory
list, application names, expected stdout, GoogleTest binary names, or
source-compilation expectations. The default command is:

    make unittest

It has no dependency on the root `build` or `install` targets. It therefore
uses the existing `hard` command from `PATH` by default. Running
`make -C unittest` from the repository root is equivalent.

`PYTHON`, `HARD`, `OUTPUT`, `JOBS`, and `SCENARIO` are Make variables.
They default to `python3`, the installed `hard` command,
`/tmp/hard-unittest`, zero, and empty respectively. Zero jobs select all
logical CPUs through the public `hard` semantics. An empty `SCENARIO`
discovers every YAML scenario; a name selects only that scenario. Direct
runner invocation accepts zero or more scenario names. Most fixtures use only
local and system headers. `011.compiled_library_recipe` downloads TinyXML2
from GitHub, while `012.well_known_recipe` first obtains its recipe from
`github.com/hard-build/recipe`; both require CMake. Downloaded snapshots remain
shared below `HARD_ROOT/source`. Generated headers, objects, and test binaries
remain below `HARD_ROOT`;
delivered application binaries remain below `OUTPUT/<scenario>`.

The main example tree is:

    /home/taitov/projects/hard-build/example

Known scenarios:

- `001.helloworld`: one simple C++ application;
- `002.internal_library`: application sources and a shared internal object;
- `003.unittest`: ordinary and `.test` sources plus `random.h`;
- `004.circular_dependency`: mutually dependent component/container headers
  and implementations; used to validate recursive source discovery, forward
  headers, cyclic graph suppression, linking, and execution;
- `005.external_library`: includes
  `github.com/nlohmann/json/single_include/nlohmann/json.hpp`; used to validate
  GitHub snapshots, safe forward filtering, cache reuse, compilation, linking,
  and execution with `example.json`;
- `007.hardlib`: includes `hard/...`; used to validate the well-known mapping,
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
- README is a concise newcomer document rather than the complete technical
  specification. It begins with the Hard Build logo, a plain-language product
  summary, and `Quick Start`, which shows a local hello-world build followed by
  direct execution of the produced binary. A following capability overview
  presents all seven public commands together with include-driven discovery,
  dependencies and recipes, content caching, parallel and verbose operation,
  and host, Linux-container, Windows, and arbitrary-image targets before the
  detailed command sections. It links directly to
  `hard-build/example/tree/main/001.helloworld` for further introduction.
  That example is expected to own its separate README; creating that file in
  the example repository remains separate future work. The complete public
  contract lives in `docs/reference.md`; maintainer and implementation history
  remains in this memory.
- MIT was selected, with the current copyright identity.
- README is English and exposes only `version`, `environment`, `format`,
  `fetch`, `build`, `run`, and `test`.
- Go 1.23 is required.
- Cobra was selected rather than a handwritten parser.
- `HARD_ROOT`, `HARD_ENV`, `HARD_CC`, `HARD_CFLAGS`, `HARD_LDFLAGS`, and
  `HARD_ENTRYPOINTS` are configuration inputs; configuration is not implemented
  as mutable package globals despite the early wording “global variables.”
- Source paths are printed relative to the working directory; managed GitHub
  progress paths use canonical `github.com/...` spelling.
- Directory symlinks are recursively traversed as ordinary directories, with
  canonical visited-directory cycle prevention.
- Build and run exclude case-insensitive `.test` and legacy `_test` sources;
  test selects both; format includes headers; fetch includes ordinary and test
  translation units.
- Format has no preliminary source list, uses `[N/M]` rather than a bar,
  supports silent and verbose output, and uses internal Go unified diffs.
- Verbose format prints each diff immediately after that file completes.
- `-jN` is an explicit count; bare or zero `-j` means every logical CPU.
- Build no longer prints discovered headers.
- Build compilation progress is labeled `Compiling`; linking and copying use
  `Linking` and `Copying`, all under one counter.
- Include discovery must honor the effective compiler flags, including
  configured `HARD_CFLAGS` and backend-managed include mechanics, and exclude
  system headers.
- `HARD_ENV` is the immutability boundary for system headers and toolchain
  state; parse and object caches hash only non-system dependencies.
- The environment support header path first changed from
  `HARD_ROOT/include/hard.h` to `HARD_ROOT/env/HARD_ENV/hard.h`. It later moved
  out of persistent data entirely: each backend now loads `<runtime-root>/hard.h`.
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
- Run requires exactly one root entry source, links only the internal binary,
  never delivers it, forwards live streams and post-`--` arguments, never caches
  execution, and propagates the program exit status.
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
- `recipe/...` is the well-known alias for
  `github.com/hard-build/recipe/...`.
- External repositories are managed source trees rather than opaque/system
  headers.
- libclang was selected as the unified mechanism for header discovery,
  dependency graphs, declaration extraction, and entry detection.
- Every source-processing command reports searching; build, fetch, run, and
  test report parsing before potentially long analysis.
- Multiple repositories downloaded in one invocation share one progress step,
  and each Downloading message is displayed before its request starts.
- Declarations owned by the canonical runtime support `hard.h` are excluded
  from source forwards, while the backend-managed force include remains.
- The public installed command is `PREFIX/bin/hard`, a shell wrapper around
  the sibling `PREFIX/libexec/hard/hard`; the wrapper derives `PREFIX` from
  its own location. `hard.h` and formats are installed beside that backend,
  while persistent source and caches remain in `PREFIX/share/hard` for the
  default user-local prefix.
- The wrapper supports mutable `--target=linux64` and `--target=windows64`
  aliases, simple Docker-tag-validated explicit versions for either known
  repository, and
  arbitrary `--target=docker://image` references.
  They run the corresponding `ghcr.io/hard-build/<target>:latest` or exact
  GHCR tag with `--pull=always` and `--pull=missing`, respectively. The wrapper does not
  resolve the host runtime or build images; the image entrypoint starts the
  container backend, persistent state is bind-mounted, and `make install`
  remains host-only.
- `host` is an explicit wrapper target. A missing `default-target` means host;
  `make install` records host, while the portable installer leaves the file
  absent. Host runtime lookup is relative to the wrapper with no
  `$HOME/.local` fallback. Explicit `--target=host` performs direct execution
  without an added diagnostic layer.
- Portable distribution uses one relocatable `linux/amd64` archive instead of
  native DEB/RPM packages. The release contract is glibc 2.27 or newer, built
  directly by GitHub Actions in a pinned Ubuntu 18.04 container and verified
  on Ubuntu 18.04, 22.04, and 24.04. Its wrapper runs the sibling runtime
  immediately after extraction. The archive carries the Go backend and its LLVM
  runtime rather than requiring distro-specific libclang packages.
- `install.sh` accepts no arguments, installs no system dependencies, stages
  the latest verified portable release below `$HOME/.local`, leaves host as the
  wrapper fallback, and configures `$HOME/.local/bin` for the user's shell.
- Bash, Zsh, and Fish completion scripts are generated from the same Cobra
  command tree during installation and release packaging. They are data files
  below standard `share/` paths, not a seventh public command. The Go tree keeps
  a synthetic `--target` declaration only so those scripts understand the
  wrapper flag. Concrete target candidates live in `hard.sh`; target-value
  requests are answered there, while all other dynamic completion requests use
  the installed host backend. Interactive completion cannot start Docker.
- Before its first external publication, the former `linux.v1` image was
  rebaselined from provisional Ubuntu 24.04 to Ubuntu 22.04. The finalized
  target keeps GCC 11 and glibc 2.35 from Jammy while preserving libclang and
  clang-format major 18 through the signed `llvm-toolchain-jammy-18` repository.
- The former `linux.v1` image includes GNU Make, CMake, Meson with Ninja,
  pkg-config, Autoconf, Automake, and Libtool for building compiled third-party
  libraries inside the target environment. `make install` for `hard` remains
  host-only.
- Integration scenarios use a strict, ordered `test.yaml` step list interpreted
  by one Python runner. Scenario files may use only `build`, `run`, and
  `test`; they cannot contain arbitrary commands. Immediate directories are
  discovered automatically, so a new scenario needs only sources and its local
  YAML file.

## Known gaps and deliberately unchanged issues

- System-header and toolchain-state changes inside one `HARD_ENV` intentionally
  do not invalidate parse or object caches. Select a new environment after
  compiler, libclang resource, standard-library, libc, sysroot, ABI, target,
  container, or a system-include tree supplied through `-isystem` or
  `-idirafter` changes; use `--no-cache` for a one-off forced rebuild in the
  current environment.
- Parse-result records use the previously observed include set. A newly created
  higher-priority header can shadow an existing include without invalidating
  that set. A dependency can also test the availability of an optional header
  through `__has_include` without that unavailable header entering the known
  set. Run build, run, or test with `--no-cache` after such topology changes.
- Unreferenced stale generated artifacts are not removed automatically.
- Test-result keys cannot infer undeclared runtime files, services, network
  responses, or time; callers use `hard test --no-cache` when these matter.
- Cached external repositories are not automatically updated or validated.
- Old per-header forward files and header parse records are not removed even
  though new builds no longer generate or include them.
- `hard environment` currently reports libc as `unavailable` on Alpine 3.22,
  although the image and `ldd --version` confirm musl 1.2.5.
- There is no private GitHub authentication configuration.
- Ubuntu 22.04 standard security maintenance ends in May 2027. Continued use
  after that date needs an explicit target-version or security-support decision.
- The legacy `ghcr.io/hard-build/hard` GHCR package returned `unauthorized` for
  an anonymous manifest request on 2026-08-23. Package visibility is not managed
  by the workflow. The new `ghcr.io/hard-build/linux64` package also requires a
  maintainer to make it public once before anonymous pulls work. Previously
  published legacy commit and edge tags remain external state; the current
  workflow neither updates nor deletes them.
- The hard-build library incomplete-type warning remains intentionally
  unresolved in the external repository.

## Test inventory

- `hard/cli_test.go`: command defaults, paths, the source-independent
  version and environment commands, interspersed and no-cache flags, silent
  options, all job syntaxes, invalid input, help, hidden commands,
  backend completion directives without concrete target ownership, and
  Bash/Zsh/Fish script generation.
- `hard/config_test.go`: all defaults and overrides, environment choice,
  executable suffix and runner, default source include and runtime support
  include, present-empty flags, safe
  shell parsing, disabled expansion, malformed values, and home failures.
- `hard/wrapper_test.go`: host passthrough, both target spellings and positions,
  installed target defaults, bundled host tool lookup, mutable and exact
  known and arbitrary Docker image mappings and pull policies, Docker arguments and mounts,
  default persistent root, argument preservation, no host `HARD_*` forwarding,
  wrapper-owned target completion, host-only dispatch for other completion
  requests, and invalid target diagnostics.
- `hard/environment_test.go`: exact rule-framed aligned plain layout, ANSI
  palette and removal, embedded version, configured flags without internal
  include mechanics, detailed sections and configured values, compiler
  diagnostics, OS/CPU parsing, unavailable probes, and output errors.
- `hard/version_test.go`: development and release version composition, exact
  line output, and output errors.
- `hard/executable_test.go`: generic suffix application, inferred versus exact
  output names, `HARD_ENV` independence, and runner-backed run/test processes.
- `hard/install_test.go`: checksum-gated installation, portable runtime file
  modes, symlinks, and completion data files, absent installed `default-target`,
  strict `vX.Y` release resolution and exact versioned asset URLs, Bash, Zsh,
  Fish, and POSIX startup configuration, Bash/Zsh completion activation,
  active and existing path handling, repeated-install idempotence, rejected
  arguments, checksum failure, failed-update runtime restoration, and absence
  of package-manager or service changes; ordered stage descriptions, curl
  progress bars, plain noninteractive output, and informational hello-world
  toolchain recommendations for supported glibc distributions and the Alpine
  Docker-target compatibility note.
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
- `hard/github_test.go`: GitHub and both well-known mappings, alias
  creation/reuse/conflicts, exact HTTP requests, safe extraction, PAX metadata,
  `.git` and traversal rejection, persistent cache, concurrency deduplication,
  live progress, transitive retries, and non-GitHub diagnostics.
- `hard/library_test.go`: strict YAML and marker validation, source/path rules,
  authoritative CMake compiler and install prefix, cleared `CXXFLAGS`, package
  manifest reuse across invocation directories, stable vendor-source CMake
  working directory, fetch-only source includes, and reachable archive selection.
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
  progress, absolute source compiler commands, quoting, errors, parallel links,
  copy behavior, and real executable integrations.
- `hard/run_test.go`: live streams and working directory, argument quoting,
  child exit propagation, single-entry validation before compilation,
  internal-only real C++ artifacts, cached rebuild reuse, unconditional child
  execution, no delivery progress, and forced no-cache rebuilds.
- `hard/cache_test.go`: stable content keys, compiler/input/dependency/source
  invalidation, semantic-result integrity, malformed records, source parse hits,
  input/flag `__has_include` suppression, cacheable dependencies containing the
  token, standard-library parse reuse, `HARD_ENV` handling of `-isystem` header
  changes, source-forward restoration, no-cache refresh,
  compile/link/delivery reuse across invocation directories, separation for
  relative or opaque working-directory-dependent compiler flags, build/run/test
  `Parsing ... (CACHED)` output,
  and successful-only test-result reuse with selector-separated keys and uncached
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
- `unittest/`: twelve declarative source-tree scenarios whose local `test.yaml`
  files require fifteen applications to build and produce exact outputs,
  eight GoogleTest binaries with fifteen cases to run successfully, automatic
  dependency sources to be discovered, one TinyXML2 package to be built and
  linked statically from both an embedded and a well-known recipe, and one
  shared production object to compile once; the Python runner validates and
  executes ordered steps, while the top-level Makefile only passes
  configuration.

## Required verification

For every completed implementation change, run the canonical repository-root
check:

    make check

It performs the following Go commands from `hard/` and uses a unique temporary
directory for the verification binary:

    gofmt -d *.go
    go test ./...
    go test -race ./...
    go vet ./...
    go build -o <unique /tmp path> .
    go mod verify

It additionally checks `hard.sh` and `install.sh` with `sh -n` and runs both
`git diff --check` and `git diff --cached --check` from the repository root.

`.github/workflows/check.yml` runs for every pull request and every push to
`main`, with superseded runs for the same ref cancelled. Its independent
GitHub-hosted `ubuntu-24.04` jobs both use Go 1.23.12 through
`actions/setup-go`, with the cache keyed from `hard/go.sum`. The `check` job
installs `build-essential` and `libclang-18-dev` before invoking `make check`.
The `integration-host` job additionally installs CMake, GoogleTest, pkg-config,
and PyYAML, builds the backend from the checked-out commit into a temporary
runtime beside the current `hard.h`, and runs all declarative scenarios with
isolated `HARD_ROOT` and output directories. The workflow does not build or
publish container images and does not install hard into a persistent prefix.

For check-workflow changes, parse the workflow as YAML, verify the two event
triggers, read-only contents permission, runner, action versions, Go version,
dependency installation, cache path, and exact `make check` command, then run
`make check` locally.

For wrapper or installation changes, `make check` already covers both POSIX
shell syntax checks. Additionally run:

    make BUILD_DIR=<unused /tmp path> build
    make BUILD_DIR=<unused /tmp path> PREFIX=<logical prefix> DESTDIR=<unused /tmp staging root> install

Inspect the staged paths, file modes, and file types, and invoke the staged
public wrapper. Verify that the backend, `hard.h`, and format file work after
staging or relocating the complete runtime bundle. When completions change,
also check all three staged completion paths and modes, validate the Bash script
with `bash -n`, source it in a clean Bash process, and confirm that target
completion through the staged wrapper does not invoke Docker. Do not install
into the real home or system tree during routine verification.

For portable release changes, validate the release workflow syntax, pinned
download checksums and builder image, archive paths and modes, executable
RUNPATHs, the maximum required GLIBC symbol, and Ubuntu 18.04/22.04/24.04 smoke
tests. Release publication remains an external action and is never part of
routine local verification.

For container-target changes, build the image locally for `linux/amd64`, inspect
its architecture, entrypoint, fixed `HARD_*` values, OCI version and revision
labels, pinned release version, runtime bundle, and dynamic libraries. Run the
real wrapper against a temporary C++ project at least twice to exercise
persistent cache reuse:

    docker build --platform linux/amd64 \
      --build-arg HARD_VERSION=v4.0 \
      --build-arg HARD_REVISION=83971184c99f79a2751bf271903ba567ba6fa8d6 \
      --file target/linux64/v4.0-glibc.2.35.Dockerfile \
      --tag hard-build/linux64:v4.0-glibc.2.35 .

    docker build --platform linux/amd64 \
      --build-arg HARD_VERSION=v4.0 \
      --build-arg HARD_REVISION=83971184c99f79a2751bf271903ba567ba6fa8d6 \
      --file target/linux64/v4.0-musl.1.2.5-static.Dockerfile \
      --tag hard-build/linux64:v4.0-musl.1.2.5-static .

Temporarily tag that local image with its exact GHCR reference for the wrapper
smoke test. Use a temporary host `HARD_ROOT`, confirm container artifacts below
the matching `env/linux64:<version>` directory, and confirm host `HARD_*`
variables do not replace the image configuration. For a static target, inspect
all generated programs for both an ELF interpreter and `NEEDED` entries. Verify
the unversioned `linux64` wrapper form selects the `latest` reference with
`--pull=always`, while the exact form uses `--pull=missing`. Validate the
workflow syntax and version/tag rules. Do not push, publish, change package
visibility, or remove a pre-existing local image as part of routine
verification.

For documentation work, reread `README.md`, `AGENTS.md`, and `MEMORY.md`
completely, plus `docs/reference.md` when it is affected; verify English
language, seven-command public scope, local links,
versions, flags, paths, defaults, implemented/target distinctions, and known
gaps.

For build behavior, use an isolated temporary `HARD_ROOT`, a complete temporary
runtime bundle containing the backend and its adjacent `hard.h`, and the system
compiler. Keep outputs under `/tmp`. Inspect artifact names with `find`, types
with `file`, and run the binary.

For libclang dependency, forward, or entry changes, verify Clang major 18,
`/usr/lib/llvm-18/include/clang-c/Index.h`, and `libclang-18`, then build and run
`004.circular_dependency`.

For GitHub changes, use a fresh isolated root with `005.external_library`.
Verify request, snapshot path, absence of `.git`, cache reuse, json forward,
successful build/link/run, and `example.json` behavior.

For well-known or managed-external changes, use a fresh isolated root with
`007.hardlib`. Verify the relative hard alias, external forwards and objects,
library implementation compilation/linking, canonical labels, actual verbose
paths, runtime output, and cached reuse. For the recipe mapping, additionally
run `012.well_known_recipe` with a fresh root, verify the relative recipe alias,
static package link and output, and repeat it with cache reads enabled.

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

Build the current backend into an unused `/tmp` runtime directory, place the
current `hard.h` and `format/format.v1` beside it, and create an isolated
`HARD_ROOT` for source and cache state. Run every numbered scenario
independently by passing its name to:

    python3 unittest/run.py \
      --hard <temporary-backend> \
      --output <temporary-output> \
      <scenario>

Then run automatic discovery through the repository-root target:

    make unittest HARD=<temporary-backend> OUTPUT=<temporary-output>

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

That snapshot predates the runtime-root split. Current integration checks copy
this root file beside the temporary backend as `<runtime-root>/hard.h`; they do
not place it below `HARD_ROOT`.

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

## Last known verification of `hard run`

On 2026-08-23, the new run command passed clean gofmt, ordinary and race tests,
vet, an out-of-tree backend build, module verification, and repository diff
checking. Unit coverage exercised `--` separation, unchanged `-j` and
`--no-cache` child arguments, run-specific flag rejection and help, build-style
source selection, live stdin/stdout/stderr, working directory, exit-code
extraction, zero/multiple entry validation, real C++ linking, internal-only
artifacts, verbose command quoting, cached rebuild reuse, unconditional program
execution, absence of `Copying`, and forced no-cache rebuilds.

The separately built backend
`/tmp/hard-check-run-command-20260823` was exercised with a real C++20 program,
an isolated `HARD_ROOT`, and a separate `HARD_ENV`. The first verbose invocation
compiled and linked an ELF binary only below the environment build tree,
forwarded a spaced argument and stdin, preserved child stderr, and created no
binary beside `app.cpp`. The second invocation reported:

    Parsing app.cpp (CACHED)
    Compiling app.cpp (CACHED)
    Linking app (CACHED)

and still ran the child with new arguments and input. A following
`run --no-cache` reported no cached stages. A silent invocation emitted only
the child's stdout/stderr and propagated its exit status 7 without a
`hard:` diagnostic.

## Last known verification of the container target

On 2026-08-23, the wrapper, runtime-root split, host installation, and
`linux.v1` image passed the complete required Go check set, wrapper shell
syntax checking, a staged `make build` and `make install`, workflow YAML
parsing, release-tag validation, and repository diff checking.

The staged host layout had modes 0755 for the wrapper and backend and 0644 for
`hard.h` and `format.v1`. The staged wrapper formatted, built, and ran a real
C++20 program. Copying the complete `libexec/hard` bundle to another temporary
directory and rebuilding with verbose output showed the relocated `hard.h` in
the compiler `-include`, while source and build artifacts remained below the
separate temporary `HARD_ROOT`.

A local `linux/amd64` image was built twice while completing verification. The
first real source-analysis smoke test exposed missing Clang resource headers
in the runtime layer. Adding `libclang-common-18-dev` supplied `stddef.h` and
`stdarg.h`; the rebuilt image then reported an amd64 architecture, the expected
backend entrypoint and fixed `HARD_*` environment, clang-format 18.1.3, GCC
13.3.0, GoogleTest 1.14.0, and a backend dynamically linked to libclang 18.

The real wrapper ran a temporary C++ program twice with deliberately invalid
host `HARD_CC`, `HARD_CFLAGS`, `HARD_LDFLAGS`, `HARD_ENV`, and
`HARD_ENTRYPOINTS`. The first invocation compiled with
`-march=x86-64-v3 -mtune=generic`; the second reported cached parsing,
compilation, and linking, and both executed successfully. The mounted host
root contained artifacts only below `env/linux.v1`. A real GoogleTest also
compiled, linked, and passed through the container target. The image was tagged
locally for wrapper verification, but no registry push, package publication,
or visibility change was performed.

On the same date, before external publication, the image was rebaselined to
Ubuntu 22.04 in both stages and built twice under the unique local verification
tag `hard-build/hard:linux.v1-jammy-check-20260823`. The second build reused
all Docker layers. The final image was amd64 with the unchanged backend
entrypoint and fixed `HARD_*` environment. It reported GCC 11.4.0, glibc 2.35,
LLVM and clang-format 18.1.8, GoogleTest 1.11.0, and CMake 3.22.1. The backend
resolved libclang 18 and libLLVM 18 dynamically, and the LLVM 18 `stddef.h`
and `stdarg.h` resource headers were present.

A real wrapper smoke test used a fresh temporary `HARD_ROOT` and temporarily
retagged only the expected local GHCR name; the prior Ubuntu 24.04 image ID was
restored afterward. Deliberately invalid host `HARD_CC`, `HARD_CFLAGS`,
`HARD_LDFLAGS`, `HARD_ENV`, and `HARD_ENTRYPOINTS` did not replace the
image settings. The first run used `-march=x86-64-v3 -mtune=generic`; the
second reported cached parsing, compilation, and linking. Formatting used
clang-format 18.1.8, and a real GoogleTest executable passed five cases. The
temporary persistent root contained only the `env/linux.v1` artifact tree.
The generated smoke binary was dynamically linked and its highest requested
glibc symbol version was `GLIBC_2.34`.

All ten declarative integration scenarios then passed inside a disposable
Ubuntu 22.04 container with the image backend and GCC 11. The complete required
Go check set, wrapper syntax, workflow YAML parsing, and repository diff check
also passed. No image was pushed or published, and no package visibility was
changed.

Later on 2026-08-23, the runtime tool set was extended for compiled third-party
libraries. The unique local image
`hard-build/hard:linux.v1-build-tools-check-20260823` built successfully for
amd64. Runtime commands reported GNU Make 4.3, CMake 3.22.1, Meson 0.61.2,
Ninja 1.10.1, pkg-config 0.29.2, Autoconf 2.71, Automake 1.16.5, and both
`libtool` and `libtoolize` 2.4.6.

The image retained the backend entrypoint, fixed `HARD_*` environment, runtime
bundle, and dynamic libclang 18 and libLLVM 18 resolution. A real wrapper smoke
test used deliberately invalid host `HARD_*` values and an isolated temporary
root. The first run compiled, linked, and executed `001.hello_world` with the
image flags; the second reused cached parsing, compilation, and linking. The
pre-existing local GHCR wrapper tag was restored to its original image ID. The
complete required Go check set also passed. No image was pushed or published.

## Compiled external library recipe decision

On 2026-08-23, compiled third-party library integration was defined around an
active included recipe header. A recipe is a leading block comment marked
`hard.recipe.v1`, followed by strict YAML, and may coexist with arbitrary C++
code and includes in that header. A `.hard.h` suffix is the recommended
collision-free convention. Only marker blocks among the leading comments
before the first C++ token are recognized. Unknown or duplicate YAML fields,
second documents, anchors, aliases, merge keys, custom tags, absolute paths,
and lexical path escapes are rejected. The former `hard.library.v1` spelling
is not recognized.

The initial format supports one `github.com/<owner>/<repository>` source, CMake,
and installed static archives. It records `source_directory`, CMake configure
arguments, source-tree include directories for `fetch`, installed include
directories, and installed archive paths. The source recipe stays with the
source that consumes it and can be reused through the existing GitHub include
namespace. The example is `unittest/011.compiled_library_recipe/tinyxml2.hard.h`;
the reusable repository publishes an equivalent header as
`recipe/tinyxml2.hard.h`, mapped to
`github.com/hard-build/recipe/tinyxml2.hard.h`.

Only the active libclang include graph activates a recipe. `fetch` downloads
both the recipe source and vendor source, uses the declared source include
directories to finish dependency analysis, and does not run CMake or the
compiler or create an environment tree. `build`, `run`, and `test` build and
install the package, append installed includes only to affected translation
units, and append static archives only to reachable binary closures.

`HARD_CC` is resolved and passed authoritatively as `CMAKE_CXX_COMPILER`.
`CMAKE_INSTALL_PREFIX` is also hard-managed. Ambient `CXXFLAGS` are cleared.
Neither `HARD_CFLAGS` nor `HARD_LDFLAGS` is passed to the vendor build. Recipe
configure arguments are the explicit place for vendor-specific options. This
means ABI-affecting project flag changes require a distinct `HARD_ENV` when
vendor compatibility could change. CMake configure, build, and install actions
run from the declared vendor source directory, which is the stable package
fingerprint working directory. Relative configure-argument values therefore do
not depend on the directory from which `hard` was invoked.

Packages use the content-addressed layout
`HARD_ROOT/env/HARD_ENV/library/github.com/<owner>/<repository>/<fingerprint>`
with `build`, `install`, and `manifest.json`. The fingerprint covers the hard
executable, complete recipe header and contents, full downloaded source tree,
CMake tool, resolved compiler tool, and recipe configuration. The manifest
verifies the complete installed regular-file tree. Parse-cache records retain
active recipe header paths. After validating the semantic-result checksum, a
prospective hit restores the package and compiler flags before validating its
action fingerprint against current inputs. If that preliminary restoration
fails, the record is treated as a cache miss and current source analysis remains
authoritative; this permits a removed recipe header to invalidate cleanly.
`--no-cache` rebuilds packages but does not refresh downloaded GitHub snapshots.
The invocation working directory is absent from the package key, so the same
recipe and source snapshot reuse one package when a consuming source is
selected from a parent directory or its own directory.

At the same time, `-I<HARD_ROOT>/source` and
`-include <runtime-root>/hard.h` moved out of configured `HARD_CFLAGS` into the
backend effective compiler arguments. They are appended even when
`HARD_CFLAGS` is explicitly empty. Host defaults are now toolchain-only:
`-std=c++20 -O3 -flto=auto -Wall -Wextra`. The `linux.v1` image retains its
x86-64-v3 and generic tuning flags in `HARD_CFLAGS`, while the backend appends
`/hard/source` and the image runtime `hard.h` internally.

The approved YAML module is `go.yaml.in/yaml/v3 v3.0.5`. The
`011.compiled_library_recipe` example builds TinyXML2 with tests, shared
libraries, and pkg-config installation disabled, installs a static
`libtinyxml2.a`, links it, and prints `answer=42`. The
`012.well_known_recipe` scenario exercises the equivalent published recipe
through the short `recipe/tinyxml2.hard.h` include.

The implementation passed clean gofmt output, ordinary and race tests, vet, an
out-of-tree backend build, module verification, and repository diff checking.
Unit coverage includes strict recipe parsing, CMake compiler authority,
`CXXFLAGS` clearing, package manifest reuse, fetch-only source includes without
an environment tree, internal compiler flags, reachable archive selection, and
fallback from a stale parse record whose former recipe header was removed.

The unique local `linux/amd64` image
`hard-build/hard:linux-v1-vendor-check-20260823` built twice; the second build
reused every layer. Inspection confirmed the backend entrypoint, toolchain-only
image `HARD_CFLAGS`, CMake 3.22.1, GCC 11.4.0, clang-format 18.1.8, and the
runtime support and Clang resource headers.

A real wrapper build with an isolated root downloaded TinyXML2, configured,
built, and installed it, compiled the project with the internal `/hard/source`
and runtime-header flags plus the installed package include, linked the
installed static archive, and produced `answer=42`. The second wrapper build
reported cached package, parsing, compilation, linking, and copying. A
subsequent `hard run` reused those cached build stages and still executed the
program. A separate `fetch` root downloaded TinyXML2, parsed its implementation,
and contained no `env` directory.

All 11 declarative integration scenarios passed in the Ubuntu 22.04 image with
a fresh root, including the new vendor build and all existing real GoogleTest
cases. No Docker image was pushed or published, and no existing GHCR local tag
was replaced.

A final cache audit added semantic-checksum validation before preliminary
package restoration and made an unavailable recipe named by a stale parse
record an ordinary cache miss. The targeted remove-recipe regression passed,
followed by the complete gofmt, ordinary test, race test, vet, out-of-tree
build, and module verification set.

The final local image `hard-build/hard:linux-v1-vendor-final-20260823` rebuilt
from that backend as amd64 with image ID `1dbad816b8e1`, the expected entrypoint,
and toolchain-only `HARD_CFLAGS`. A fresh-root TinyXML2 build configured, built,
and installed the package, linked its static archive, and produced `answer=42`.
The second build reported cached package, parsing, compilation, and linking. No
image was pushed or published.

## GHCR publication policy decision

On 2026-08-23, pushing commit `16ad23b` to GitHub triggered the initial
workflow policy and published `sha-16ad23b-linux.v1` and `edge-linux.v1`. The
user rejected commit and edge image tags and confirmed that the wrapper's
stable `ghcr.io/hard-build/hard:linux.v1` reference is the only tag the
workflow should publish. Ordinary branch pushes must not build or rebuild the
image.

At that time, the replacement workflow was independent from releases and had
no manual dispatch. A push to `main` published only Dockerfiles added below
`target/` whose paths have never occurred in earlier history, mapping each
basename to the same image tag. Existing target Dockerfiles are immutable:
modification, deletion, and re-addition do not publish them. `linux.v1` is
excluded explicitly, so the current workflow cannot advance it. Release-specific,
commit, edge, and `latest` tags are not published. Existing remote tags are not
deleted by this repository change because registry deletion is a separate
external action. At the time of the decision, an anonymous manifest request
for the published commit tag returned `unauthorized`, so public package
visibility still required a maintainer action in GitHub.

On 2026-08-24, release tags were changed from three components to the strict
`vX.Y` format, and container publication was separated from releases. Both
workflow files parsed as YAML. The exact release validation accepted `v1.0`
and `v12.34` and rejected `v1`, `v1.0.0`, `1.0`, and `v1.x`. The container
discovery script was executed in an isolated Git repository: adding
`target/linux.v2.Dockerfile` produced only the `linux.v2` matrix entry;
modifying existing target files produced an empty matrix; deleting and
re-adding `linux.v2` was skipped by the history check; and re-adding
`linux.v1` was skipped by its explicit immutable-target guard. No Docker image
was built or published during this verification.

## Last known verification of the portable release

On 2026-08-24, the portable installer and host release passed clean gofmt,
ordinary and race tests, vet, an out-of-tree backend build, module
verification, both wrapper shell syntax checks, workflow YAML parsing, and
repository diff checking. `go test -count=1 ./...` forced the external
`install.sh` tests to run without Go's test cache. Their fake package manager,
Docker, service manager, and downloads verified all three modes, installed
defaults and file modes, restored a previous runtime after an injected update
failure, omitted host build-system packages, and stopped before package changes
on a bad archive checksum.

A staged `make install` under `/tmp` produced 0755 wrapper/backend files, 0644
`hard.h`, `format.v1`, and `default-target`, recorded `host`, and ran the staged
wrapper's explicit host help. The installer package mappings were separately
queried in current Fedora, Rocky Linux 9, and openSUSE Tumbleweed containers.
This confirmed Fedora's `gtest-devel`, `pkgconf-pkg-config`, and `moby-engine`,
Rocky's EPEL `gtest-devel` and Docker's RHEL repository packages, and
openSUSE's `gtest`, `pkgconf-pkg-config`, and `docker` names.

The exact release build was then reproduced locally with test version
`v0.0.0`, the pinned Ubuntu 18.04 image, the checksum-verified official LLVM
18.1.8 archive, and Go 1.23.12. It produced a 59 MiB
`hard-linux-amd64.tar.gz` with SHA-256
`7f1c0a1e80bc486c4a426d70a5850589bb31cff57e8c3f4071269a7e7b5ab624`.
The backend RUNPATH was `$ORIGIN/lib`; backend and clang-format `ldd` output had
no missing libraries; the maximum GLIBC symbol across the bundled backend,
formatter, libclang, and libtinfo was `GLIBC_2.16`, below the promised 2.27
ceiling. Wrapper, backend, and formatter modes were 0755, data files were 0644,
and library aliases remained symbolic links.

The archive's explicit host help and bundled formatting operation succeeded in
clean Ubuntu 18.04, 22.04, and 24.04 containers. An attempted dependency parse
in a package-empty Ubuntu 18.04 base correctly demonstrated why the glibc
launch floor is not a native toolchain promise: that image has no C++20
standard-library headers, and its packaged GCC 7 also rejects `-std=c++20`.
No GitHub release, registry package, or image was published or changed during
this verification.

## Last known verification of cross-directory cache reuse

On 2026-08-24, compiler source arguments were changed to lexical absolute
paths while compiler processes retained the invocation working directory.
Parse, compile, and link fingerprints omit that directory for cwd-independent
flags and retain it for relative or opaque compiler-driver arguments. CMake
recipe actions now run from and fingerprint the stable vendor source directory.

The complete gofmt, ordinary test, uncached test, race test, vet, out-of-tree
build, module verification, and repository diff-check set passed. All 11
declarative integration scenarios passed with an isolated runtime and
`HARD_ROOT`.

The real `example/006.vendor_library` YAML-CPP example was first built from the
parent example repository and then from its own directory. The second run
reported cached package preparation, parsing, compilation, linking, and
delivery, while one package fingerprint directory remained. The binary was an
x86-64 ELF executable, linked the installed static `libyaml-cpp.a`, and printed
`answer=42`.

## Last known verification of the well-known recipe mapping

On 2026-08-24, `recipe/<path>` was mapped to
`github.com/hard-build/recipe/<path>`. Unit coverage passed for dependency
classification, the exact `hard-build/recipe` snapshot request, canonical
installation, relative alias creation, canonical resolution, persistent cache
reuse, and the unchanged `hard/<path>` mapping.

The real `012.well_known_recipe` scenario passed with an isolated runtime and a
fresh `HARD_ROOT`. Its canonical recipe snapshot contained no `.git` directory,
and `HARD_ROOT/source/recipe` resolved through the relative target
`github.com/hard-build/recipe`. TinyXML2 was configured, built, installed as a
static library, linked into the copied application, and produced `answer=42`.
A second cache-enabled build reported cached package preparation, parsing,
compilation, linking, and delivery.

The existing `example/007.hardlib` mapping regression also passed from a fresh
alias with its external implementation objects, expected lifecycle output, and
cached repeat build. All 12 declarative integration scenarios then passed with
the newly built backend.

The complete required verification set passed: clean gofmt output, ordinary
and race Go tests, vet, an out-of-tree backend build, module verification, and
repository diff checking. The new C++ fixture also passed the repository's
Clang 18 format check.

## Test source filename convention

On 2026-08-25, the preferred C and C++ test-source names became
`*.test.c`, `*.test.cc`, `*.test.cpp`, and `*.test.c++`. The legacy
`*_test.*` convention remains fully supported for backward compatibility.
Both forms are case-insensitive, are excluded from `build` and `run`, are
included by `fetch` and `format`, and are selected by `test`. Binary names
continue to follow the source stem naturally, so `name.test.cpp` produces
`name.test` and legacy `name_test.cpp` produces `name_test`.

All current C and C++ test files in the local `hard`, `example`, `library`,
and `recipe` repositories were migrated to the preferred form. Go
`*_test.go` files remain unchanged. The VS Code extension now uses both forms
in its classifier, Test Explorer default glob, current-file menu condition, and
user-facing diagnostic.

The complete required Go verification passed: clean gofmt output, ordinary and
race tests, vet, an out-of-tree backend build, and module verification. All 12
declarative integration scenarios passed individually and again through Make
autodiscovery with a fresh runtime, `HARD_ROOT`, environment, and output tree.
The runner used Python 3.12.3 and PyYAML 6.0.1. A separate real
`calculator_test.cpp` build and five-test execution verified the legacy
convention end to end. The renamed
example test passed; all four library binaries passed 78 tests; and both recipe
tests passed after static TinyXML2 and YAML-CPP builds. The VS Code extension
passed ESLint, TypeScript compilation, and all 19 tests in a temporary Node.js
container. Temporary negative configurations also confirmed rejection of
duplicate YAML keys, unknown actions and fields, invalid field types, escaping
paths, and a run action placed before a build action.

## Last known verification of the relocatable release wrapper

On 2026-08-25, the host wrapper stopped using a fixed
`$HOME/.local/libexec/hard` path. Host execution and an installed default target
now resolve from the wrapper's own installation prefix, with no home-directory
fallback. Explicit Docker execution does not resolve that host runtime and
continues to rely on the container image entrypoint.

Uncached wrapper tests passed for an unpacked prefix whose path contained a
space, explicit host execution with bundled tools, invocation through `PATH`,
an installed Docker default, and an explicit Docker target with no sibling
`libexec/hard` directory. A real `make install` into a temporary non-default
`PREFIX` produced a working host wrapper without a Makefile change.

The saved `v1.0` archive was extracted below a path containing a space and its
wrapper was replaced with the current source. Running it without arguments
reached the sibling backend's `a command is required` diagnostic; explicit host
help and real formatting through the bundled clang-format both succeeded. The
release workflow parsed as YAML. Clean gofmt output, uncached ordinary and race
tests, vet, an out-of-tree backend build, module verification, and wrapper
shell syntax checking all passed.

## Last known verification of shell completions

On 2026-08-25, Bash, Zsh, and Fish completion generation, fixed target and
format values, public-command filtering, filesystem directives, bare-job flag
handling, wrapper host-only dispatch, installer placement, startup activation,
and repeated-install idempotence passed their Go tests. The complete required
Go verification also passed: clean gofmt output, ordinary and race tests, vet,
an out-of-tree build, and module verification.

A real staged `make install` below `/tmp` generated all three completion
files, installed them with mode 0644 in their standard `share/` locations,
preserved the 0755 wrapper/backend and 0644 runtime data modes, and kept
`default-target` at `host`. The staged wrapper served `host` and `linux.v1`
target candidates through the host backend after its temporary
`default-target` was changed to `linux.v1`. The Bash script passed
`bash -n`, loaded in a clean Bash 5.2 process, and registered completion for
`hard`. Zsh and Fish were not installed locally; their generated scripts were
covered by content tests but were not executed by those shell binaries.

Both POSIX shell syntax checks, release-workflow YAML parsing, and
`git diff --check` passed.

## Last known verification of linux64 image versioning

On 2026-08-26, container targets changed from the legacy `linux.v1` name to
the mutable `linux64` alias and exact
`linux64:vX.Y-ubuntu.YY.MM` versions. The wrapper maps them to the
`ghcr.io/hard-build/linux64` repository with `--pull=always` and
`--pull=missing`, respectively.

The new `v2.0-ubuntu.22.04.Dockerfile` installs the checksum-pinned official
`hard-v2.0.tar.gz` release rather than rebuilding the current branch. Its
bundled `VERSION` is verified as `v2.0`, and the OCI revision is the commit
named by Git tag `v2.0`. Workflow discovery passed isolated tests for a new
version, an ordinary modification, an older version, deletion and re-addition,
an invalid filename, and a missing release tag.

The first real C++ smoke exposed that portable libclang did not automatically
find its relocated resource headers. A plain `-isystem` path made GCC consume
Clang-specific headers and was rejected. Adding the runtime include directory
with `-idirafter` preserved GCC's builtin headers and fixed libclang analysis.
The Dockerfile now compiles a minimal C++ source during the image build so this
failure cannot pass publication again.

Clean gofmt output, ordinary and race Go tests, vet, an out-of-tree build,
module verification, POSIX wrapper and installer syntax, workflow YAML parsing,
and staged `make install` checks passed. The local `linux/amd64` image built
successfully, verified the release SHA-256 and `VERSION`, and exposed the
expected entrypoint, OCI labels, runtime files, dynamic libraries, GCC 11,
clang-format 18.1.8, Make, CMake, Meson, Ninja, Autoconf, Automake, and Libtool.
The real wrapper built and ran an iostream program twice with poisoned host
`HARD_*` values; the repeat reported cached parsing, compilation, linking, and
delivery, and artifacts existed only below
`env/linux64:v2.0-ubuntu.22.04`.

## Last known verification of the Alpine static prototype

On 2026-08-26, `linux64:v2.0-alpine.3.22-static` was implemented and verified
locally as an exact-only prototype. Before commit or publication, the user
chose to prepare hard v3.0 so relocated LLVM resource headers could become an
internal parser-only concern. The prototype Dockerfile was removed from the
working tree; the replacement target will be
`linux64:v3.0-alpine.3.22-static` after the v3.0 tag exists.

The final local `linux/amd64` image ID was
`sha256:992af7837911ccd00fd2864db7d64c8a1158066e705e9209b1e85aef84f9ec60`.
Its backend is a dynamic musl executable, its `VERSION` is `v2.0`, its OCI
revision is `100406872f99fd4fcdb23425d21f638d58368237`, and its entrypoint is
`/usr/local/libexec/hard/hard`. The image contains the expected compiler,
format, recipe-build, test, license, and LLVM runtime files.

The first full integration run exposed that Alpine's packaged GoogleTest
libraries were shared-only for linker purposes. The prototype Dockerfile built
`libgtest.a` and `libgtest_main.a` from the official `gtest-src` package. After
that change all 12 declarative integration scenarios passed, including the
GoogleTest, embedded TinyXML2 recipe, and well-known TinyXML2 recipe scenarios.
A GoogleTest executable and an ordinary delivered executable were both
verified with `file` and `readelf` to be statically linked, with neither an ELF
interpreter nor `NEEDED` shared libraries.

The real installed wrapper built and ran an application twice with poisoned
host `HARD_*` values. The repeat reported cached parsing, compilation, linking,
and delivery, and the only environment directory was
`env/linux64:v2.0-alpine.3.22-static`. Isolated workflow checks accepted both
supported filename forms, rejected malformed static forms, extracted `v2.0`,
discovered the new Alpine Dockerfile, and kept `publish_latest=false`.

The required ordinary and race Go tests, vet, out-of-tree build, module
verification, gofmt check, shell and workflow syntax checks, documentation
validation, and diff checks passed for the prototype. These results establish
the Alpine package/toolchain design but do not verify the future v3.0 image.

## v3.0 preparation status

On 2026-08-26, v3.0 preparation moved the portable LLVM 18 resource include
out of image `HARD_CFLAGS`. `hard` now detects
`<runtime-root>/lib/clang/18/include` and appends it only to libclang arguments.
Unit coverage verifies both the portable-directory and native-system cases and
that the caller's compiler flags are not mutated.

The concrete target candidates also moved out of the Go backend. `hard.sh`
answers attached and separate target-value completion directly with `host`,
`linux64`, `linux64:v3.0-ubuntu.22.04`, and
`linux64:v3.0-alpine.3.22-static`. Other completion requests still execute the
host backend and cannot start Docker.

Container publication now builds and loads a newly discovered image locally,
runs a C++20 program using `<cstddef>` and `<iostream>`, verifies static ELF
metadata for `*-static`, and pushes tags only after success. The smoke source
belongs to CI rather than new Dockerfiles. Release smoke additionally builds
that C++20 program on Ubuntu 22.04 and 24.04; Ubuntu 18.04 remains a portable
runtime launch and formatting check because its default compiler is outside
the supported host contract.

Local verification passed the required gofmt, ordinary and race Go tests, vet,
out-of-tree build, module verification, wrapper and installer syntax, staged
installation, all three completion files, generated Bash target completion,
workflow YAML and nested-shell syntax, and diff checks. A portable-shaped
runtime with an adjacent LLVM 18 resource directory built and ran a C++
application while its verbose compiler command contained no parser-only path.
All 12 declarative integration scenarios passed with that backend. The new CI
smoke also passed against the existing local v2.0 Ubuntu image and the removed
Alpine prototype, including the static ELF checks; this validates the workflow
command sequence, not either future v3.0 image.

The preparation commit and annotated `v3.0` tag were pushed. The first release
workflow run built the portable archive successfully, but failed before
starting its first compatibility container. Nested single quotes in the
`sh -c` smoke script ended the runner's outer quoted string, so runner Bash
expanded the inner completion check's `$1` while `set -u` was active. Artifact
upload and GitHub release publication were therefore skipped.

The release workflow is now split into build, smoke, and publish jobs. Build
uploads the archive and checksum as a workflow artifact and exposes its ID.
Smoke uses a non-fail-fast matrix of Ubuntu 18.04, 22.04, and 24.04 job
containers, so the compatibility commands execute directly in each target
environment without a nested `docker run` or quoted container script.
Publication depends on both build and the complete smoke matrix.

Modern JavaScript artifact actions cannot execute in the Ubuntu 18.04 job
container because the runner's Node runtime requires a newer glibc. Each
matrix container therefore installs `curl` and `unzip` and downloads the
same immutable workflow artifact through GitHub's authenticated REST API.
The host-side publish job can use `actions/download-artifact` normally.

Local verification parsed the workflow, checked every run script with
`bash -n`, and asserted the three exact matrix images, non-fail-fast policy,
job dependencies, artifact-ID handoff, and absence of nested Docker in smoke.
All three local base images installed their declared dependencies successfully;
Ubuntu 22.04 and 24.04 also compiled and ran the smoke C++20 source. The actual
GitHub Actions run described below subsequently verified the REST artifact
transfer and complete matrix.

The corrected workflow commit and moved annotated `v3.0` tag were pushed. Its
release run built and uploaded the portable archive, passed the complete smoke
matrix, and downloaded the workflow artifact in the publish job. Publication
then failed because that separate job had no checkout while its `gh release`
commands did not select a repository explicitly.

The follow-up passes `--repo "$GITHUB_REPOSITORY"` to release view, upload, and
create, so publication no longer depends on a local Git checkout. The follow-up
commit and moved annotated `v3.0` tag were pushed, and the subsequent workflow
published the GitHub Release. Its `hard-v3.0.tar.gz` asset has SHA-256
`4a5d0227e80148684559d148be815cd6169f311fd0abe5b43ad2940b301e9fc1`.
Tag `v3.0` resolves to commit
`3826020ccc617f189521e5628e2ce5f8ecf82e00`; the corresponding GitHub source
archive has SHA-256
`ee24cbeec82087f31a0c07d7a346f85c0f3d5b36fd25199a90ebb69c1e1bee35`.

## Last known verification of v3.0 container images

On 2026-08-26, two new immutable repository targets were prepared:
`linux64:v3.0-ubuntu.22.04` and
`linux64:v3.0-alpine.3.22-static`. The Ubuntu Dockerfile installs the exact
portable release asset; the Alpine multi-stage Dockerfile builds the exact tag
source against musl and Alpine's system LLVM 18. Neither Dockerfile contains a
smoke-test layer because `.github/workflows/container.yml` owns publication
smoke testing.

Both `linux/amd64` images built locally with their pinned checksums and version
checks. Their local IDs are
`sha256:5ac994271f12b7a0cabcbee6acfd2cd00aa5b6d86915071885159fd6f9c3712d`
for Ubuntu and
`sha256:9ada3fea3a68a2f92a253c363017594d2094e01267a594092c62db3ea18643fb`
for Alpine. Inspection confirmed the v3.0 backend entrypoint, runtime files,
OCI revision, toolchain and recipe-build tools, target-specific environment,
and the absence of parser-only `-idirafter` from `HARD_CFLAGS`. The Ubuntu
backend resolves the bundled portable libclang; the Alpine backend is a
dynamic musl program resolving Alpine's system libclang.

The real wrapper built and ran the same C++20 program twice through each exact
target with poisoned host `HARD_*` variables. The second run reused parsing,
compilation, linking, and delivery, and the shared root kept separate
`env/linux64:v3.0-ubuntu.22.04` and
`env/linux64:v3.0-alpine.3.22-static` cache trees. All 12 declarative
integration scenarios passed independently in both images, including
GoogleTest, the embedded TinyXML2 recipe, and the well-known TinyXML2 recipe.
All 38 Alpine-generated ELF applications and test binaries inspected across
the output and build trees were statically linked and had neither an ELF
interpreter nor `NEEDED` entries.

An isolated replay of workflow discovery produced exactly two matrix entries,
both tied to tag `v3.0` and its commit. The Alpine entry had
`publish_latest=false`; the newest Ubuntu entry had `publish_latest=true`.
Final repository verification also passed clean gofmt output, ordinary and
race Go tests, vet, an out-of-tree Go build, module verification, POSIX wrapper
and installer syntax, workflow YAML parsing and Bash syntax for every run
block, documentation path and link checks, Dockerfile whitespace checks, and
`git diff --check`.

The exact GHCR references currently exist only as local tags. These repository
changes are not committed, pushed, or published remotely, and no package
visibility setting has been changed.

## Windows cross-compilation target preparation

On 2026-08-26, a new immutable target was prepared as
`windows64:v4.0-llvm-mingw.20260616-ucrt`, with `windows64` as its moving
wrapper alias. Its image reference is
`ghcr.io/hard-build/windows64:v4.0-llvm-mingw.20260616-ucrt`; the exact target
uses pull-if-missing while the short alias always checks GHCR for the newest
published image. The Dockerfile is based on Ubuntu 22.04 and builds the exact
future `v4.0` source revision rather than installing a prebuilt host archive.

The image pins LLVM-MinGW's
`llvm-mingw-20260616-ucrt-ubuntu-22.04-x86_64` archive with SHA-256
`534b92e067b22a6b4441f48ae9240a3341b17825d04d577eab0cf85c44b4deda`.
It uses Clang, LLD, libc++, and the UCRT-flavoured MinGW runtime from that
archive, and Ubuntu's exact LLVM/libclang 22.1.8 packages for parsing and for
building `hard`. Target compiler flags include `-march=x86-64-v3` and
`-mtune=generic`; linker flags include `-static` so the bundled C++ runtime is
embedded, while Windows system and UCRT API-set imports remain. The image also
contains Wine and a cross-built static GoogleTest, and exposes a CMake
toolchain file to recipe builds without passing project `HARD_CFLAGS` or
`HARD_LDFLAGS` into them.

The initial preparation inferred `.exe` and Wine from a `windows64` environment
prefix. Directory and default build outputs preserved that suffix, while an
exact file passed to `-o` remained exactly the caller's path. That target-name
coupling was superseded by the generic configuration recorded below. Libclang resource discovery
now accepts the single version directory found below
`<runtime-root>/lib/clang`, so the same backend supports the portable LLVM 18
runtime and the image's LLVM 22 runtime; an ambiguous multi-version layout is
rejected. The declarative integration runner gained explicit executable suffix
and runner options so container CI can validate Windows output without
changing the Linux scenarios.

A production-equivalent local Dockerfile was verified with the repository
working tree copied into the builder because the required `v4.0` tag does not
exist yet; the committed Dockerfile differs there by downloading the exact
future tag revision. The resulting local image ID is
`sha256:8f2d4b5cb5e3f014707fe3f036824467138bf8649a87693366da823fbe3611a0`.
Inspection confirmed x86-64 PE/COFF output, the AMD64 machine type, UCRT API-set
imports, the absence of separate libc++ runtime DLL dependencies, and PyYAML
6.0.3 in the final image. Build and run also succeeded under an arbitrary
non-root UID/GID while leaving the shared cache owned by that caller.

The locally built image passed `hard run`, `hard test`, and all 12 declarative
integration scenarios through Wine. This includes all supported source and
test extensions, GoogleTest, shared production dependencies, the embedded
TinyXML2 CMake recipe, and the well-known TinyXML2 recipe. Required Go checks
also passed: clean gofmt output, ordinary and race tests, vet, an out-of-tree
build, and module verification. Wrapper and Python syntax, workflow YAML and
every nested Bash run block were checked. The Windows Dockerfile, workflow,
backend, wrapper, documentation, and tests remain uncommitted; no `v4.0` tag,
push, image publication, or external package-visibility change was performed.

## Generic executable configuration, arbitrary images, and environment report

On 2026-08-27, the backend stopped recognizing `linux64` and `windows64`
environment names. `HARD_ENV` remains only the persistent artifact and
immutable-toolchain cache boundary. `HARD_EXECUTABLE_SUFFIX` now controls all
inferred internal, default-delivery, and directory-output executable names;
`HARD_EXECUTABLE_RUNNER` independently controls the process prefix for run,
test listing, and test execution. The Windows Dockerfile explicitly owns
`.exe` and `wine`, while host and Linux images leave both values empty.

The wrapper now accepts `docker://<image>` in addition to known target aliases
and exact tags. It strips only that prefix, uses `--pull=missing`, preserves the
ordinary `/hard` and working-directory mounts and numeric user, and leaves the
entrypoint and all target configuration to the image. Empty images and values
beginning with `-` are rejected before Docker starts. `docker://` is a fixed
shell-completion candidate; other target completion behavior remains on the
host.

`hard environment` became the sixth public command. It loads the same runtime
and `HARD_*` configuration as a build, but does not create progress, discover
sources, or touch build state. Its report combines hard paths, operating-system
and CPU diagnostics, libc, resolved compiler/version/triple, suffix/runner,
effective flags, entry points, libclang version, and portable resource
directory. Missing individual probes render `unavailable`; configuration and
output failures remain fatal.

A production-equivalent verification layer was built on the prior local
Windows image with the current untagged backend. Its image ID is
`sha256:6555902a410fe02b7a2477921ff8d7bb216c774bf014a5d56d084966ea3e18cf`.
Inspection showed amd64, the hard entrypoint, `.exe`, and `wine`; the real
environment report showed Ubuntu 22.04, glibc 2.35, LLVM-MinGW Clang 22.1.8,
`x86_64-w64-windows-gnu`, libclang 22.1.8, and the runtime Clang 22 resource
directory. The source wrapper successfully ran that local image through
`--target=docker://hard-windows64-current:test`.

All 12 declarative scenarios built their expected PE32+ x86-64 application and
GoogleTest outputs and ran them through Wine. A completely fresh Wine prefix
wrote its one-time setup diagnostics to the first scenario's stderr, so that
initial aggregate run returned nonzero even though scenarios 002–012 passed;
rerunning 001 after the prefix initialization passed cleanly. A separate real
`hard run` used the internal `.exe` through the configured Wine runner. The
isolated root contained only the expected
`env/windows64:v4.0-llvm-mingw.20260616-ucrt` directory.

The complete required Go check set, both POSIX shell syntax checks, staged
Make build/install, runtime file modes, host `hard environment`, generated
target completion including `docker://`, Bash syntax, and clean Bash completion
registration passed. No tag, push, image publication, package visibility
change, or removal of an existing image was performed.

Later on 2026-08-27, the environment report gained its final terminal layout:
a bold cyan title and rule, bold green runtime/system/compiler/build/parser
section names, cyan aligned labels, and yellow special states. CFLAGS, LDFLAGS,
and entry points now render as one shell-quoted argument per line. The default
output is colored, while `--no-color` preserves byte-for-byte equivalent text
after ANSI removal. Exact plain-layout, palette-presence, ANSI-removal, and
output-error tests passed together with clean gofmt output, ordinary and race
tests, vet, an out-of-tree backend build, and module verification. A real host
report showed Ubuntu 24.04.4, glibc 2.39, GCC 13.3.0, and libclang 18.1.3; its
plain form contained no escape character.

## Last known verification of v4.0 Linux images and generic target tags

On 2026-08-27, two new immutable Linux definitions were prepared:
`linux64:v4.0-glibc.2.35` and `linux64:v4.0-musl.1.2.5-static`. Both build the
hard backend from commit `83971184c99f79a2751bf271903ba567ba6fa8d6`, the
commit selected by Git tag `v4.0`, and record `v4.0` in the image runtime.
Their locally built image IDs are
`sha256:8a0b31eaf93b63157840c156cfe7be75b57c043ec6d6a81c11e09cae364f14ca`
for glibc and
`sha256:d6f8b81a00821936d4aae49c0f09d3d903356388dc52547b166b046f26545fb8`
for musl.

Inspection confirmed amd64, the hard entrypoint, target-specific OCI metadata,
all fixed `HARD_*` values, the recipe-build tools, GCC 11 with glibc 2.35, and
GCC 14 with musl 1.2.5. A C++20 hello-world built and ran in each image. The
glibc output is dynamically linked and requires at most `GLIBC_2.34`; the musl
output is fully static and has neither an ELF interpreter nor a dynamic
section. The real source wrapper built the program twice through each local
image while host compiler, flags, environment, and entry points were poisoned;
the second run reused parsing, compilation, linking, and delivery from each
target-specific cache. All 12 declarative integration scenarios passed
independently in both images, including GoogleTest, the embedded TinyXML2
recipe, and the well-known TinyXML2 recipe.

An isolated replay of the exact workflow discovery run block produced the two
expected matrix entries. The glibc image had `publish_latest=true`; the musl
static image had `publish_latest=false`; both selected hard release `v4.0`.
Workflow YAML parsing and Bash syntax for all ten run blocks passed. Staged
`make install`, host wrapper execution, target completion, completion syntax,
and installed file modes also passed. The complete required Go check set
passed: clean gofmt output, ordinary and race tests, vet, an out-of-tree build,
and module verification. Both POSIX scripts and `git diff --check` passed.

One existing diagnostic gap was observed but not changed in this image task:
`hard environment --no-color` reports Alpine libc as `unavailable`, although
the Dockerfile check and `ldd --version` both confirm musl 1.2.5. No commit,
tag change, push, registry publication, image removal, or package-visibility
change was performed.

The first GitHub publication attempt for `linux64:v4.0-musl.1.2.5-static`
failed in the smoke test after the image built successfully. Musl's
`ldd --version` prints the expected `Version 1.2.5` line but exits with status
1 when no program is supplied. Under the workflow's `set -e`, the command
substitution therefore stopped the step before its exact version check. The
container command now neutralizes only that expected inner status with
`ldd --version 2>&1 || true`; failure to start Docker still propagates, and the
following exact `grep` still rejects missing or incorrect version output.

Because a rerun stays on the original event commit, the corrected workflow
also gained a manual recovery dispatch. It accepts one tracked target
Dockerfile from `main`, performs the ordinary target and release validation,
and bypasses only the automatic history skip. Before rebuilding, it verifies
that the exact version tag is still absent from GHCR and refuses to overwrite
an existing tag. Registry responses other than the expected
`manifest unknown` remain errors.

Local execution of the exact discovery block selected both new Linux images
for their automatic addition, selected no images for an ordinary push, and
selected only the existing musl Dockerfile for manual recovery. A non-main
ref, a missing file, and an invalid path were rejected. The exact immutable-tag
guard accepted the live missing musl tag, rejected the live published glibc
tag, and rejected a simulated authentication failure. Workflow YAML parsing,
Bash syntax for all four run blocks, the complete required Go check set, both
POSIX shell syntax checks, and `git diff --check` passed.

## Last known verification of installer output

On 2026-08-26, `install.sh` gained an eight-stage user-facing progress flow,
curl progress bars for both release downloads, and post-install instructions
for a minimal C++20 hello-world toolchain. The recommendations cover supported
glibc releases of Ubuntu/Debian, Arch/CachyOS, Fedora/RHEL/Rocky, and openSUSE;
Alpine is explicitly directed to Docker because its musl environment cannot
run the portable glibc host runtime. All recommendations remain informational:
the installer still invokes no package manager, `sudo`, Docker, or service
manager.

Clean gofmt output, ordinary and race Go tests, vet, an out-of-tree Go build,
module verification, both POSIX shell syntax checks, and `git diff --check`
passed. Unit coverage verified the ordered stage text, download URLs, curl
progress-bar option, every recommendation, and absence of ANSI escapes in
noninteractive output. `shellcheck` was not installed locally and was not run.

A staged `make build` and `make install` below `/tmp` produced the expected
0755 wrapper/backend and 0644 runtime, default-target, and completion files.
The staged wrapper reached its sibling backend with `--target=host --help`, and
the Bash completion file passed `bash -n`.

Finally, the real installer resolved and downloaded the official v3.0 archive
and checksum into an isolated temporary `HOME`. Both curl progress bars were
visible in a pseudo-terminal, checksum and bundle validation succeeded, the
expected Bash startup entries were written only below that temporary home, and
the installed wrapper's explicit host help succeeded. No real user or system
installation path was changed.

## Last known verification of the one-command repository check

On 2026-08-27, the repository-root Makefile gained `make check` as the
canonical local verification entry point. It enforces Go formatting, runs the
ordinary and race test suites, vet, an isolated build below a unique temporary
directory, module verification, both POSIX shell syntax checks, and staged and
unstaged Git whitespace checks. The temporary build directory is removed by a
shell trap.

The complete target passed repeatedly. An isolated negative fixture containing
an unformatted Go function printed the expected unified gofmt diff and made
`make check` exit nonzero before tests began. Docker image builds, container
smoke tests, workflow YAML validation, staged installation, and declarative C++
integration scenarios remain separate checks and are intentionally not part of
this initial target.

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
   update `README.md` and `docs/reference.md` with their respective
   user-facing contract changes, and update `AGENTS.md`
   when repository working rules change;
8. run the required unit, race, static, build, module, diff, and applicable
   integration checks;
9. reread the complete diff skeptically;
10. report result, exact verification evidence, and remaining work.
