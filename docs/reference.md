# Hard Build Reference

This document defines the complete user-facing behavior of `hard`. For an
overview and first build, start with the [project README](../README.md).

The public interface contains only `version`, `environment`, `format`, `build`,
`fetch`, `run`, and `test`.

Recorded and unrecorded projects use the same [persistent cache layout](cache-layout.md).
[Project configuration and pinned dependencies](#project-configuration-and-pinned-dependencies)
defines `hard.yaml`, fixed revisions, defaults, and update policy.

## Installation

On Linux x86-64, run the installer from a terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/hard-build/hard/main/install.sh | sh
```

The installer accepts no arguments. It resolves the latest `vX.Y` GitHub
release, downloads its
`hard-vX.Y.tar.gz` archive and SHA-256 file, verifies the archive, and then
installs its relocatable `bin/`, `libexec/hard/`, and shell-completion files
below `~/.local`. It does not invoke `sudo`, a distribution package manager,
or a Docker service.

The terminal output names each of eight stages: checking compatibility,
resolving the latest release, downloading the archive, downloading its
checksum, verifying it, extracting and validating the bundle, installing the
files, and configuring the current shell. Download entries include the source
URL and use curl's progress bar. ANSI styling is enabled only when stdout is a
terminal and `NO_COLOR` is unset, so piped or captured output remains plain.

The installer adds `~/.local/bin` to its process `PATH` when necessary and
records the path for new shells according to `$SHELL`:

| Shell | Startup file | Added entry |
| --- | --- | --- |
| Bash | `~/.bashrc` | `export PATH="$HOME/.local/bin:$PATH"` |
| Zsh | `~/.zshrc` | `export PATH="$HOME/.local/bin:$PATH"` |
| Fish | `~/.config/fish/config.fish` | `fish_add_path "$HOME/.local/bin"` |
| Other | `~/.profile` | `export PATH="$HOME/.local/bin:$PATH"` |

An existing equivalent path entry is not duplicated. Because a piped `sh`
process cannot modify its parent shell, the installer prints the same command
to run when the current shell did not already contain `~/.local/bin`; opening
a new shell reads the saved startup entry automatically.

Command completion is installed at the shell-standard user-local paths:

| Shell | Completion file | Activation |
| --- | --- | --- |
| Bash | `~/.local/share/bash-completion/completions/hard` | Sourced from `~/.bashrc` |
| Zsh | `~/.local/share/zsh/site-functions/_hard` | `compinit` and the file are loaded from `~/.zshrc` |
| Fish | `~/.local/share/fish/vendor_completions.d/hard.fish` | Discovered automatically |

Existing Bash and Zsh activation entries are not duplicated. Completion offers
the seven public commands, command flags, filesystem paths, `host`, `linux64`,
`windows64`, `docker://`, the published version tags for both known GHCR
repositories, and the default `format.v1` format. The wrapper obtains the
versioned targets from GHCR through an anonymous pull-scoped token and rejects
`latest`, malformed tags, and tags belonging to another repository. The token
is kept only for the request. A successful list is cached for five minutes in
`${XDG_CACHE_HOME:-$HOME/.cache}/hard/target-completion` with mode `0600`.

When the cache expires, completion refreshes both known repositories. A failed
refresh silently retains a stale valid cache; without one, the stable `host`,
`linux64`, `windows64`, and `docker://` values remain available. A missing
`curl`, unset cache location, unwritable cache directory, or malformed cache
does not make completion fail. Other dynamic requests always execute the
installed host backend, even when a container target is the installed default,
so pressing Tab never starts or pulls a Docker container. POSIX shells without
programmable completion and PowerShell do not receive a completion integration.

The release archive can also be unpacked and used without installation:

```bash
tar -xzf hard-vX.Y.tar.gz
./hard-linux-amd64/bin/hard --target=host --help
```

The archive wrapper derives its installation prefix from its own location and
executes the sibling `libexec/hard/hard` backend for `host`. Because an
unpacked archive and the installed runtime have no `default-target`, their
default is `host`.

The installer does not install native compilers, GoogleTest, pkg-config, GNU
Make, CMake, Meson/Ninja, Autoconf, Automake, Libtool, or Docker. Install the
particular host tools required by the commands and reachable recipes you use.
Container images already contain those build tools, but selecting one requires
Docker to have been installed and started separately.

After a successful installation, the script prints an informational-only
`Next steps` section. For a native hello-world build it recommends a C++20
compiler and shows the minimal package command for Ubuntu/Debian, Arch Linux
and CachyOS, Fedora/RHEL/Rocky Linux, and openSUSE. The displayed compatibility
floors are Ubuntu 22.04, Debian 12, RHEL 9, and Rocky Linux 9; the openSUSE
command is for Tumbleweed. Alpine uses musl and cannot run the portable glibc
host runtime, so the installer directs Alpine users to the Docker target
instead. The section then shows how to check `c++ --version` and run
`hard build example.cpp`. As an alternative, it shows
`hard --target=linux64 build example.cpp` for a Docker-provided toolchain.
None of these recommendation commands is executed by the installer.

## Requirements

The portable release requires Linux x86-64 and glibc 2.27 or newer. Its host
backend, libclang 18.1.8, Clang resource headers, clang-format, and the required
`libtinfo` compatibility library are installed together. Release CI builds the
backend in a pinned Ubuntu 18.04 environment, rejects a runtime GLIBC symbol
requirement above 2.27, smoke-tests launching and formatting on Ubuntu 18.04,
22.04, and 24.04, and additionally builds a C++20 program on the supported
Ubuntu 22.04 and 24.04 host toolchains.

That glibc floor covers launching the bundled backend and formatter. Native
dependency analysis, compilation, linking, and tests still require C++20
standard-library headers and a compiler that accepts the configured flags.
Ubuntu 18.04's default GCC 7 does not meet that toolchain contract; use the
`linux64` target there unless a suitable host toolchain is
configured explicitly.

Building the Go module in `hard/` requires:

- Linux;
- Go 1.23 or later;
- CGO and a C++20 host toolchain;
- LLVM/Clang 18 development headers at `/usr/lib/llvm-18/include` and the
  `libclang-18` shared library.

Depending on the command, using `hard` also requires:

- a C/C++ compiler supporting the configured flags (`c++` by default) for
  `build`, `run`, and `test`;
- CMake when an active source include reaches a `*.hard.h` recipe wrapper;
- `clang-format` for source formatting;
- `pkg-config` and GoogleTest's `gtest_main` package for test compilation and
  linking;
- network access to GitHub when a referenced `github.com/<owner>/<repository>/`
  or well-known repository snapshot is not already cached below
  `HARD_ROOT/snapshot`.

`curl` and network access to GHCR are optional for refreshing versioned target
completion. Stable targets and an existing cached registry list remain usable
without them.

Using `--target=linux64`, `--target=windows64`, or `--target=docker://image`
requires Docker. The documented target images contain their own backend and
C/C++ toolchain, so the host does not need the native build requirements
listed above for target-mode execution. An arbitrary image is responsible for
providing the compatible entrypoint and environment itself.

## Command-line interface

```text
hard [--target=<name>] [-v|--verbose] [--no-color]
     [-jN|--jobs=N] <command> [path...]
```

The default job count is one. `-jN` or `--jobs=N` selects `N` workers. Bare
`-j`, bare `--jobs`, `-j0`, and `--jobs=0` use all logical CPUs. Negative job
counts are rejected. A command never creates a nested `N × N` pool. In
`hard fetch`, the selected count is used for dependency analysis. In `hard run`,
it limits source preparation and compilation; exactly one binary is linked and
executed. In `hard test`, the selected count is the invocation-wide maximum for
source preparation, compilation, linking, and test-execution phases.

`-v` writes permanent progress entries and command-specific details.
`--no-color` disables ANSI colors. The source-processing commands accept `-s`
or `--silent`, which suppresses normal output while preserving errors.

Persistent flags may appear before or after the command. Command-local flags,
including `-s`, appear after the command and may be interspersed with paths.

The public interface contains exactly these commands:

```text
hard version
hard environment
hard format [--format=<name>] [-s|--silent] [path...]
hard build  [--locked] [--no-cache] [-s|--silent] [-o <path>] [path...]
hard fetch  [--lock | --locked | --update=<repository>@<ref>...]
            [--no-cache] [-s|--silent] [path...]
hard run    [--locked] [--no-cache] [-s|--silent] [path...]
            [-- program-argument...]
hard test   [--list-tests] [--test=<selector>]...
            [--locked] [--no-cache] [-s|--silent] [path...]
```

### `hard version`

```bash
hard version
```

`version` prints the version embedded in the running backend as one line. The
source defaults are version number `8.0` and prerelease identifier
`development`, producing `v8.0-development`. Release packaging clears the
prerelease identifier through a Go linker value, producing `v8.0`, and rejects
a binary whose output does not match the release tag.

The command accepts no paths and does not determine the runtime root, read a
runtime version file, load `HARD_*` configuration, probe the compiler or
libclang, scan sources, or create artifacts. A wrapper target selects which
backend supplies the embedded version in the same way as for other commands.

### `hard environment`

```bash
hard environment
hard --target=linux64 environment
```

`environment` writes a human-readable report describing where and how the
current invocation would build binaries. It takes no paths, does not search a
source tree, and does not create or read build artifacts. The report contains:

- the embedded hard version;
- the resolved hard executable, runtime root, `HARD_ENV`, and `HARD_ROOT`;
- operating-system identity from `/etc/os-release`, kernel and architecture
  from `uname`, the first CPU model from `/proc/cpuinfo`, the logical CPU
  count, and libc from `getconf GNU_LIBC_VERSION` with `ldd --version` as a
  fallback;
- `HARD_CC`, its resolved executable, the first `--version` line, and its
  `-dumpmachine` target;
- the configured executable suffix and runner;
- configured `HARD_CFLAGS`, `HARD_LDFLAGS`, and entry points, rendered as one
  shell-quoted argument per line; the CFLAGS list omits the hard-managed
  source-root include and runtime-header force include;
- the libclang API version and the single portable Clang resource directory,
  or `system default` when the runtime supplies none.

Failure of an individual system or toolchain probe is printed as
`unavailable`; the remaining diagnostics continue. Invalid `HARD_*`
configuration and failure to write the requested report remain command errors.
The output encloses its title between matching horizontal rules and has aligned
fields with separate runtime, system, compiler, build-configuration, and parser
sections. By default the title and labels use cyan, section headings use green,
and `unavailable`, `none`, `direct`, and `system default` use yellow.
`--no-color` preserves the complete layout while removing all ANSI sequences.
The command has no `--silent` flag because the report itself is its result.

### Container targets

The POSIX wrapper accepts `--target=<name>` and `--target <name>` anywhere
before `--`. `host` executes the private backend from the same installation
prefix. Container execution accepts the latest `linux64` or `windows64` image,
any syntactically valid explicit tag for either known image repository, or any
image reference prefixed with `docker://`:

```bash
hard --target=host build src
hard --target=linux64 build src
hard test --target linux64:v4.0-glibc.2.35 tests
hard run --target=linux64 src/application.cpp -- --mode=check
hard --target=windows64 build src/application.cpp
hard --target=docker://registry.example/toolchain:tag environment
```

Without `--target`, the wrapper uses the choice recorded in the sibling
`libexec/hard/default-target` file. `make install` writes `host` to that file.
The portable installer and an unpacked archive leave the file absent, which
also selects `host`. An explicit target always overrides that default, and no
compatibility diagnostics are added to host execution.

A target-looking value after the `run` separator belongs to the program and is
not interpreted by the wrapper. Empty and repeated targets are errors. A named
explicit image must be `linux64:<tag>` or `windows64:<tag>`, where the tag is
at most 128 ASCII characters, begins with an ASCII letter, digit, or
underscore, and otherwise contains only letters, digits, underscores, dots,
and hyphens. The wrapper validates only this Docker tag syntax; it does not
interpret a version, libc, distribution, toolchain, or other tag component.
Unknown target repositories and the legacy `linux.v1` spelling are rejected.

For a container target, the wrapper only executes `docker run`; it never builds
an image or resolves the host runtime. The image entrypoint runs the container
backend. Unversioned forms check for a newer image on every invocation:

```text
linux64 -> ghcr.io/hard-build/linux64:latest     (--pull=always)
windows64 -> ghcr.io/hard-build/windows64:latest (--pull=always)
```

Documented version forms name immutable images and download them only when
missing:

```text
linux64:v4.0-glibc.2.35
  -> ghcr.io/hard-build/linux64:v4.0-glibc.2.35  (--pull=missing)
linux64:v4.0-musl.1.2.5-static
  -> ghcr.io/hard-build/linux64:v4.0-musl.1.2.5-static  (--pull=missing)
linux64:v3.0-ubuntu.22.04
  -> ghcr.io/hard-build/linux64:v3.0-ubuntu.22.04  (--pull=missing)
linux64:v3.0-alpine.3.22-static
  -> ghcr.io/hard-build/linux64:v3.0-alpine.3.22-static  (--pull=missing)
windows64:v4.0-llvm-mingw.20260616-ucrt
  -> ghcr.io/hard-build/windows64:v4.0-llvm-mingw.20260616-ucrt  (--pull=missing)
```

For `docker://registry.example/toolchain:tag`, the wrapper removes exactly the
`docker://` prefix and passes `registry.example/toolchain:tag` to
`docker run --pull=missing`. It does not prepend GHCR or validate a known tag
shape. An empty image or an image beginning with `-` is rejected before Docker
is started. The image must own a hard-compatible entrypoint and complete
`HARD_*` configuration. It receives the same `/hard` persistent-state mount,
same-absolute-path project mount, working directory, UID:GID, and stdin
contract as the documented images.

The current glibc definition is
`target/linux64/v4.0-glibc.2.35.Dockerfile`. It uses Ubuntu 22.04, downloads
the exact Git revision supplied for tag `v4.0`, and builds the backend against
the apt.llvm.org Jammy libclang 18 packages. Its final runtime uses the same
system libclang and Clang resource headers, verifies `glibc 2.35`, and records
`HARD_ENV=linux64:v4.0-glibc.2.35`.

The current static musl definition is
`target/linux64/v4.0-musl.1.2.5-static.Dockerfile`. It downloads the same exact
Git revision, builds the backend natively on Alpine 3.22 against the packaged
libclang 18, verifies musl 1.2.5, and records
`HARD_ENV=linux64:v4.0-musl.1.2.5-static`. Generated programs are linked
completely statically against musl. The image also builds static GoogleTest
archives from Alpine's packaged sources.

The older `target/linux64/v3.0-ubuntu.22.04.Dockerfile` installs the official
v3.0 portable archive with SHA-256
`4a5d0227e80148684559d148be815cd6169f311fd0abe5b43ad2940b301e9fc1`. The
older `target/linux64/v3.0-alpine.3.22-static.Dockerfile` builds the v3.0
source archive with SHA-256
`ee24cbeec82087f31a0c07d7a346f85c0f3d5b36fd25199a90ebb69c1e1bee35`.
Their exact image tags remain available for compatibility.

The Windows definition is
`target/windows64/v4.0-llvm-mingw.20260616-ucrt.Dockerfile`. It builds the
backend from the exact Git revision supplied for tag `v4.0`, against the
apt.llvm.org Jammy libclang 22.1.8 packages, and installs the resulting runtime
at `/usr/local/libexec/hard`. It checksum-verifies the official LLVM-MinGW
20260616 UCRT archive for Ubuntu 22.04 with SHA-256
`534b92e067b22a6b4441f48ae9240a3341b17825d04d577eab0cf85c44b4deda`.
LLVM-MinGW supplies Clang/LLD/libc++ 22.1.8 and the Windows SDK headers and
libraries. The image also cross-builds static GoogleTest archives, installs
Wine 64-bit, and supplies a CMake toolchain file at
`/opt/windows64/toolchain.cmake`.

The glibc image uses Ubuntu 22.04. Its C++ toolchain is GCC 11 with glibc 2.35.
`-static-libgcc` and `-static-libstdc++` do not make glibc static, so the image
does not promise that generated programs run on systems older than the Ubuntu
22.04 ABI. The runtime also contains GoogleTest 1.11, CMake 3.22, GNU Make,
Meson with Ninja, pkg-config, and the Autoconf, Automake, and Libtool toolchain.
Distribution package revisions are resolved when the image is built. Ubuntu
22.04 standard security maintenance ends in May 2027.

An image version is published only when its previously unseen Dockerfile is
added. The newest glibc version advances `linux64:latest`, with older
Ubuntu-named images included when selecting that lineage. The newest LLVM-MinGW
UCRT version advances `windows64:latest`; adding a musl or older Alpine static
image leaves `linux64:latest` unchanged. Modifying, deleting, or re-adding a
known Dockerfile does not rebuild or republish it. CI loads each new image
locally, builds and executes a C++20 smoke program, and only then pushes its
tags. For a `*-static` image, the same pre-publication step rejects an ELF
interpreter or `NEEDED` shared library. For `windows64`, it requires an AMD64
PE file with UCRT contract imports, executes it through `hard run` and Wine,
and runs all declarative integration scenarios inside the image.

The wrapper bind-mounts the host `${HARD_ROOT:-$HOME/.local/share/hard}` at
`/hard` and the current working directory at the same absolute path inside the
container. The v4.0 images use
`/hard/env/linux64:v4.0-glibc.2.35/build` and
`/hard/env/linux64:v4.0-musl.1.2.5-static/build`. Every image preserves the
shared source snapshots while isolating its build artifacts from other target
versions and `env/host`.

The Windows image uses
`/hard/env/windows64:v4.0-llvm-mingw.20260616-ucrt/build` and keeps its Wine
prefix below the same environment directory.

The container runs with the current numeric UID and GID, preventing root-owned
build outputs, and forwards stdin without allocating a TTY. Only the working
directory and `HARD_ROOT` are mounted. A parent `hard.yaml` does not change the
mount; run from the project root when its configuration and sibling sources
must be available inside the container. Explicit inputs or resolved symlinks
outside both trees are therefore unavailable in the container. A non-empty
host `HARD_ROOT` selects the bind-mount source but is not copied into the
container environment. No host `HARD_*` values are forwarded, including
`HARD_PROXY` even when explicitly empty. The wrapper neither inspects nor mounts
an external `HARD_CONFIG` file. Configure any required proxy, configuration path,
and credentials within the container environment. The glibc image fixes its
complete target configuration as follows:

```text
HARD_ROOT=/hard
HARD_ENV=linux64:v4.0-glibc.2.35
HARD_CC=c++
HARD_CFLAGS=-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto
            -Wall -Wextra
HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra
             -static-libgcc -static-libstdc++
HARD_ENTRYPOINTS=main _start
```

The musl image uses the same fixed root, compiler, compiler flags, and
entrypoints; only the environment name and linker flags differ:

```text
HARD_ENV=linux64:v4.0-musl.1.2.5-static
HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra
             -static -static-libgcc -static-libstdc++
```

The Windows image fixes the cross-toolchain configuration as follows:

```text
HARD_ROOT=/hard
HARD_ENV=windows64:v4.0-llvm-mingw.20260616-ucrt
HARD_CC=x86_64-w64-mingw32-clang++
HARD_CFLAGS=-std=c++20 --target=x86_64-w64-mingw32
            --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++
            -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra
HARD_LDFLAGS=-std=c++20 --target=x86_64-w64-mingw32
             --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++
             -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra
             -static
HARD_ENTRYPOINTS=main _start
HARD_EXECUTABLE_SUFFIX=.exe
HARD_EXECUTABLE_RUNNER=wine
CMAKE_TOOLCHAIN_FILE=/opt/windows64/toolchain.cmake
PKG_CONFIG_LIBDIR=/opt/windows64/lib/pkgconfig
```

`-static` links the LLVM-MinGW C++ runtime into generated programs, but Windows
system and UCRT API-set DLL imports remain. The image is therefore not named or
documented as a fully static target. The generic suffix and runner variables
make `hard run` and `hard test` execute the resulting `.exe` files through
Wine; the backend does not recognize the `windows64` environment name.

Those historical v4.0 backends add `-I/hard/source` and
`-include /usr/local/libexec/hard/hard.h` internally. These are hard-managed
include mechanics rather than part of the image `HARD_CFLAGS` value.
The current backend instead adds its project's environment-specific `include/`
view, as described in the [cache layout](cache-layout.md).
The version-independent Windows Dockerfile also moves its environment-wide
Wine prefix to `/hard/project/windows64:${IMAGE_VERSION}/@runtime/wine`. Its
Wine launcher creates missing parent directories on first use; the backend
and host wrapper do not initialize Wine state. Historical versioned Dockerfiles
and already-published images retain their original paths.

Hard scans `<runtime-root>/lib/clang/*/include` and adds the directory only to
libclang arguments when exactly one exists. It is not part of `HARD_CFLAGS`
and is never passed to the compiler. The after-system position lets the
configured standard-library discovery keep using compiler headers during
analysis. No matching directory is normal for a native libclang installation;
multiple matching resource directories are an error.

Container images use the `linux/amd64` platform. Programs built by the current
images require an x86-64-v3 processor; Docker and Wine do not emulate missing
CPU instructions. Static Alpine outputs use musl rather than glibc. Windows
outputs are x86-64 PE executables targeting UCRT.

If no path is supplied, `.` in the invocation directory is used.
Directories are scanned recursively subject to project `exclude`. If
paths are supplied, only explicitly named matching files and matching files
below explicitly named directories are selected as roots. During `build`,
`fetch`, `run`, and `test`, implementation sources associated with project
headers may additionally be discovered as dependencies of those roots. `build`,
`run`, and `test` compile those implementations; `fetch` only inspects them.

| Command | Selected files |
| --- | --- |
| `build` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, excluding `*.test.*` and legacy `*_test.*` |
| `run` | Same as `build` |
| `fetch` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, including `*.test.*` and legacy `*_test.*` |
| `format` | Build extensions plus `*.h`, `*.hh`, `*.hpp`, `*.h++` |
| `test` | `*.test.c`, `*.test.cc`, `*.test.cpp`, `*.test.c++`; legacy `*_test.*` is also supported |

Extensions and the `.test` and `_test` suffixes are matched without regard to
case.
Unsupported explicitly named files are ignored. Missing or inaccessible paths
are errors. Finding no matching files is a successful no-op except for `run`,
which requires exactly one root entry source.

Directory symlinks are followed recursively and treated like ordinary
directories. Resolved directories are visited only once, preventing symlink
cycles. Files are deduplicated by their resolved absolute paths, while the
first selected spelling is retained for display and compilation.

### `hard format`

```bash
hard format [--format=<name>] [-s|--silent] [path...]
```

Formatting is implemented. Every selected source or header is formatted in
place by a separate process:

```text
clang-format --style=file:<runtime-root>/format/<name> -i <file>
```

`--format` overrides project `format`, which defaults to `format.v1`.
Its value is resolved relative to
the `format` directory installed beside the running backend. The runtime root
is derived from the physical backend executable path, including through a
symlink; it is normally `~/.local/libexec/hard` on the host and
`/usr/local/libexec/hard` in container images. Empty values, absolute paths,
lexical escapes through `..`, non-regular files, and symlinks resolving outside
the real format directory are rejected. An internal symlink to a regular style
file is allowed.

Formatting uses the selected job count. A formatter exit failure is reported
after independent files have been attempted. Failure to start `clang-format`
stops new scheduling. An empty selection does not require a style file or a
formatter executable.

Source search is preparation step one. After selection, the exact total becomes
one plus the number of files, so formatting completions begin at `[2/M]`.

Output modes:

- Normal mode updates one line from `[1/?] Searching source files` to
  `[N/M] file`.
- `-v` writes one line per completed file and immediately follows a changed
  file with a unified diff.
- `-s` writes no normal output; formatter errors still go to stderr.
- Progress and diffs are colored unless `--no-color` is set.

Unified diffs are produced by the Go implementation; no external diff program
is required.

### `hard build`

```bash
hard build [--locked] [--no-cache] [-s|--silent] [-o <path>] [path...]
```

The build pipeline prepares dependencies, analyzes each source, generates one
source-context forward per translation unit, compiles objects, links reachable
objects for configured entry functions, and delivers binaries.

Build, run, and test share one full libclang AST per source analysis attempt:
it supplies dependencies, declarations, and entry functions. A cache hit makes
zero libclang calls. A miss with prepared dependencies and package flags makes
one call; neither entry detection nor forward generation reparses the source
or the generated header. Package flags recovered from the previous parse
record are used on the first attempt even when source contents changed.
The new active graph still determines the final packages and flags, so removing
a recipe removes its flags and can require another analysis.

Every root or automatically discovered translation unit is analyzed through
libclang 18. `hard` passes the effective compiler flags: configured
`HARD_CFLAGS`, the hard-managed source and runtime-header includes, and any
active package includes. A portable runtime's LLVM 18 resource directory is
added only for analysis. `hard` also passes the project working directory and
C++ language mode unless the configured flags already select a language. The
detailed preprocessing record supplies one active, preprocessor-aware include
graph for direct, transitive, macro-expanded, conditional, and force-included
headers.

libclang classifies system headers, so only resolved non-system dependencies
participate in implementation discovery, source-context forward extraction,
and cache fingerprints. Their canonical absolute paths are deduplicated and
sorted per source. `HARD_ENV` is the immutability boundary for the compiler
toolchain, standard library, libc, sysroot, Clang resource headers, and every
path marked as system by libclang, including user-provided `-isystem` and
`-idirafter` directories. System headers are excluded from parse and
compilation cache
fingerprints.
No preliminary header list is printed; preparation progress identifies only
the translation unit currently being parsed. The same analysis reports
unresolved include spellings. A missing include whose expanded path begins
with:

```text
github.com/<owner>/<repository>/
```

causes `hard` to select the project's pin, an applicable inherited pin, or the
cached default. On first default use it resolves the branch to a full commit.
The archive contains source files only, not Git history, and is installed at:

```text
HARD_ROOT/snapshot/github.com/<owner>/<repository>/@<commit>
```

Well-known include prefixes map shorter public paths to canonical repositories.
The current mapping is:

```text
hard/<path>   -> github.com/hard-build/library/<path>
recipe/<path> -> github.com/hard-build/recipe/<path>
```

Each project's locked include view exposes relative aliases:

```text
<project-owner>/include/github.com/<owner>/<repository> -> <selected-snapshot>
<project-owner>/include/hard   -> github.com/hard-build/library
<project-owner>/include/recipe -> github.com/hard-build/recipe
```

Managed repository links are refreshed under the project/environment lock when
the selected revisions change. Stale aliases are removed; non-symlink entries
where an alias is expected are errors and are not overwritten.

The repository archive is extracted into a temporary directory, checked for
path and symlink escapes, stripped of GitHub's generated top-level directory,
and moved into place only after extraction succeeds. Directories, regular
files, safe relative symlinks, and standard global PAX metadata are accepted;
archive entries below `.git`, hard links, and other special entries are
rejected.

After installation, the libclang analysis is retried. If the downloaded
headers expose another missing `github.com/` or well-known include, that
repository is downloaded in the same way and scanning repeats until the
external dependency closure is available. Parallel translation units share one
resolver, so one repository is downloaded at most once per invocation.

Snapshots are verified against their adjacent checksum and are never refreshed
in place. `@default` is a regular file containing the default full commit ID;
reusing it does not resolve a moving branch again. To select a newer revision
for a project, record dependencies and use `fetch --update`. This does not
change `@default`. The resolver is shared by `build`, `fetch`, `run`, and `test`.
Before downloading it reports `Downloading <source>@<commit>` in preparation
step one. A missing non-GitHub header retains the original libclang diagnostic
and does not cause a network request.

#### Compiled library recipes

An active include of `name.hard.h`, `.hh`, `.hpp`, or `.h++` activates the
neighboring `name.hard` descriptor. Header extensions are case-insensitive.
The lookup shares the same canonical sibling-file mechanism used to find an
implementation source for a header; a missing descriptor is an error.
`name.hard` is a standalone, strict YAML document:

```yaml
version: 1
source: "github.com/leethomason/tinyxml2"
build_system: "cmake"
source_directory: "."
configure_arguments:
  - "-DCMAKE_BUILD_TYPE=Release"
  - "-Dtinyxml2_SHARED_LIBS=OFF"
  - "-Dtinyxml2_BUILD_TESTING=OFF"
  - "-Dtinyxml2_INSTALL_PKGCONFIG=OFF"
  - "-DCMAKE_INSTALL_LIBDIR=lib"
  - "-DCMAKE_INSTALL_INCLUDEDIR=include"
source_include_directories:
  - "."
include_directories:
  - "include"
static_libraries:
  - "lib/libtinyxml2.a"
```

Its C++ wrapper contains the public include and any ordinary C++ code:

```cpp
#pragma once

#include <tinyxml2.h>
```

The migration is immediate: embedded `hard.recipe.v1` and `hard.library.v1`
comments are not parsed. Unknown and duplicate fields, multiple documents,
aliases, anchors, merge keys, custom tags, unsupported versions, and invalid
source/install paths are rejected.

Dependencies may be declared in the descriptor:

```yaml
dependencies:
  - "zlib.hard"
  - "recipe/another.hard"
  - "github.com/owner/repository/library.hard"
```

Or a wrapper may actively include another recipe wrapper:

```cpp
#include "zlib.hard.h"
#include <png.h>
```

Both forms contribute to the same graph, including wrapper includes reached
through ordinary headers. Inactive branches are ignored. Descriptor references
use quoted-include lookup: the referring descriptor's directory, then `-iquote`,
`-I`, `-isystem`, and `-idirafter` directories. Relative paths, including `..`,
and absolute local references work like includes. GitHub and well-known paths
use the existing repository resolver and the same pin, override, checksum and
inherited-requirement rules. Each reference must end in `.hard`. It loads only
the descriptor, without preprocessing an adjacent wrapper. Cycles are errors.

Version 1 supports a GitHub source written exactly as
`github.com/<owner>/<repository>`, the `cmake` build system, and installed
static archives. `source_directory` is relative to the downloaded repository.
`source_include_directories` are used only by `fetch` while inspecting the
downloaded source tree. `include_directories` and `static_libraries` are
relative to the package install prefix and are used by `build`, `run`, and
`test`.

For a build command, `hard` downloads the repository snapshot, configures it
with the resolved `HARD_CC` as the authoritative `CMAKE_CXX_COMPILER`, builds
with the selected job count, and installs it into a content-addressed package
directory. `hard` also owns `CMAKE_INSTALL_PREFIX`; recipes cannot override
either managed CMake setting. Ambient `CXXFLAGS` are cleared for the configure
process. `HARD_CFLAGS` and `HARD_LDFLAGS` are not passed to the external build;
recipe-specific vendor options belong in `configure_arguments`. CMake always
runs with the declared vendor source directory as its working directory, so
relative configure-argument values and the package fingerprint do not depend
on the directory from which `hard` was invoked.

Dependencies build before their consumers. Their transitive install prefixes
are prepended to the CMake process's `CMAKE_PREFIX_PATH`, and `lib/pkgconfig`
and `share/pkgconfig` below each prefix are prepended to `PKG_CONFIG_PATH`.
Existing environment values are preserved. System search paths and other
ambient CMake settings remain available: this is not a hermetic build policy.
Recipe-specific discovery still follows the upstream CMake project. No
additional definitions, compiler flags or linker flags are exported to the
application; consumer exports are only include directories and static archives.

Installed include directories are appended only to translation units whose
active libclang include graph reaches the recipe header. Installed archives
are appended only when linking a binary whose reachable source closure uses
that recipe or one of its dependencies. Across the whole binary closure,
archives are deduplicated by package variant and ordered with dependents before
their dependencies. Includes hidden by an inactive preprocessor branch therefore do
not cause a download, package build, compiler flag, or link input.

Packages are stored at:

```text
HARD_ROOT/project/HARD_ENV/
└── github.com/<owner>/<repository>/package/<fingerprint>/
    ├── manifest.json
    └── generation-<id>/
        ├── build/
        └── install/
```

The fingerprint includes the `hard` executable, YAML descriptor contents,
the wrapper's own preprocessed code, dependency package variants, full downloaded
source tree, CMake executable, resolved `HARD_CC` executable, recipe paths,
configure arguments, and stable vendor source working directory. Included
public-header contents are not part of the wrapper's own-code component.
Different expanded code or dependency variants select different packages;
repeated unguarded includes retain separate contexts. Empty wrappers and direct
descriptor references with the same dependency variants share a package.
Neither the invocation working directory nor the recipe header's filename is
part of this package key. Packages are shared between projects, including
pinned projects with separate source views and projects with identical local
recipes. `HARD_ENV` and differing package inputs keep builds isolated. A
manifest verifies the complete installed file tree before reuse. Relative
installed symlinks to regular files inside the install prefix are supported;
escaping links are rejected and changes to their targets invalidate the package.

An interprocess lock serializes validation and building of each package.
Successful builds atomically publish a manifest pointing to a new generation;
consumers use that generation's stable paths. `--no-cache` also builds a new
generation and refreshes the manifest, but does not refresh the downloaded
GitHub snapshot or remove files that another build may still use. A failed
rebuild leaves no reusable manifest and removes its partial generation while
preserving previously published files. Old generations and the former
project-local library caches are not automatically migrated or removed, so the
first build after this cache-layout change compiles the library once.

Because vendor builds intentionally do not receive `HARD_CFLAGS`, changing ABI-affecting
project flags without also changing `HARD_ENV` can produce an incompatible
project/package combination; use a distinct environment for such flag changes.

Reusable recipes are available through the `recipe/` well-known namespace. For
example:

```cpp
#include <recipe/tinyxml2.hard.h>
```

This maps to `github.com/hard-build/recipe/tinyxml2.hard.h`. The existing GitHub
resolver first downloads the recipe repository, then discovers its active
recipe and obtains TinyXML2.

Discovery does not require installed public headers: libclang keeps processing
active recipe includes after missing vendor includes. For `build`, `run`, and
`test`, `HARD_CC -E -dI` supplies each wrapper's own expanded code and active
include context. On the bootstrap pass, temporary empty headers stand in for
unresolved ordinary includes. If missing vendor macros prevent preprocessing,
the keep-going graph first supplies provisional packages. Analysis and
preprocessing must succeed with real installed headers before compilation. A
graph that does not stabilize within 32 package-include updates is an error. A valid analysis
cache hit restores stored variants without another preprocessing subprocess.
`fetch` uses only libclang and source-tree includes; it neither starts the
compiler nor creates packages.

Each root or automatically discovered translation unit produces one
source-context forward file from the declarations visible in its final
libclang analysis. Only declarations physically originating in active managed
non-system dependencies can enter the output; declarations from the source
file itself and from system headers are excluded. `hard` emits named classes,
structs, supported class templates, scoped enums, and unscoped enums with an
explicit underlying type, declared directly at global or namespace scope.
It preserves ordinary and inline namespace nesting, removes template defaults,
skips class template specializations, and excludes local, nested, and
anonymous-namespace types.

Downloaded repositories are managed source trees, not opaque system libraries.
Their active headers can contribute declarations to each translation unit
forward file, and same-stem implementation sources are discovered, compiled,
and linked through the same dependency graph. Well-known paths are
canonicalized through their aliases before dependency and object paths are
derived.

All supported declarations are emitted, including unused enums. The bridge
obtains enum types, template parameters, and references from libclang; hard
serializes the complete forward without additional libclang validation.
Opaque enums precede templates across namespace groups. Underlying enum types
are preserved using their canonical spelling. Builtin typedefs in non-type
template parameters can also use their canonical type, such as `size_t` as
`unsigned long` on a platform where those are equivalent.

Template constraints retain their original tokens and order. Signatures that
depend on unavailable aliases, concepts, values, macros, or unsupported header
context are omitted as a whole; hard does not silently remove a `requires`
clause. Ordinary enums without a fixed underlying type cannot be forward-declared
and are omitted. Templates requiring such an
enum are omitted too. These are conservative supported forms, not a promise
to synthesize every C++ declaration. Conditional and macro-expanded class and
namespace names still come from the source's active AST.

The ordinary compiler checks the generated header together with the original
definitions. There is no separate forward-validation pass. A parse-cache record
does not assert that object compilation succeeded; an object-cache success is
written only after the compiler succeeds.

The forward path preserves the owner-relative source path below
`<owner>/build/<build-key>` and appends `.fwd.h` to the complete source name:

```text
first.cpp  -> first.cpp.fwd.h
second.cpp -> second.cpp.fwd.h
```

There is no `value_fwd.h` or other per-header forward output. Every generated
source forward begins with `#pragma once`. When regenerated content is
byte-for-byte unchanged, the existing regular file is retained.

A successful translation-unit analysis is persisted as a versioned
`.hard-parse-cache.json` record under the owner's `parse/build/<context-key>`
directory, preserving the source-relative path. The record
stores its managed dependency list, complete active non-system dependency
snapshot, active recipe headers and descriptor/variant dependency graph,
detected entry point, and final generated source-forward text. Records include
a checksum of that semantic result and the selected snapshot key. After the checksum is validated, a
prospective hit restores the packages and package include flags named by that
graph before its action fingerprint is validated against current inputs.

A parse-cache key includes the `hard` executable digest, libclang version, the
effective compiler flags (configured, hard-managed, and active-package flags),
configured entry-point names when relevant, and the content of the input plus
every active non-system dependency known from the previous successful
analysis, including non-system force-included headers and transitive `.hard`
descriptors. The invocation working directory participates only when a compiler
argument can depend on it, such as a relative include, forced-include, toolchain, or response-file path, or an
opaque forwarded driver argument. The storage context key retains the
invocation's include context but excludes selected snapshots; the record checks
the snapshot selection before reuse. Different invocation directories do not
share direct source-analysis records. System headers are represented by the
selected `HARD_ENV` rather than individual content hashes. Missing, malformed,
changed, or internally inconsistent records are misses. A hit skips libclang
analysis, restores a missing generated source forward when needed, and reports
`Parsing <path> (CACHED)`.

No parse record is written when `__has_include` occurs in the input itself or
the analysis flags. Non-system dependencies containing the token remain
content-hashed without disabling the input's cache, so ordinary sources that
include the C++ standard library remain cacheable. Like compiler depfiles, the
cache cannot notice a change in optional-header availability tested by
`__has_include` inside a dependency, or a newly created higher-priority header
that shadows an already resolved include, while the input and every previously
known non-system dependency remain unchanged. Use `--no-cache` after such
include-path topology changes.

After source preparation succeeds, every root source and automatically
discovered implementation source is compiled in parallel. Exactly one
source-context forward is force-included for that translation unit:

```text
HARD_CC <HARD_CFLAGS...> \
  -I<project-owner>/include \
  -include <runtime-root>/hard.h \
  <active-library-include-flags...> \
  -include <source.cpp.fwd.h> \
  -c <absolute-source> -o <object>
```

The generated `-include` pair appears after `HARD_CFLAGS` and before `-c`.
Even when no eligible declarations exist, the source forward is present and
contains `#pragma once`, which keeps the compiler command shape uniform. The
compiler process retains the invocation working directory, preserving the
meaning of relative user flags, while the source argument after `-c` is always
the lexical absolute source path.

The canonical target of `<runtime-root>/hard.h` is the one managed header
exception. It is force-included by the backend independently of
`HARD_CFLAGS`, but its declarations are excluded from the generated source
forward. Other project or external headers named `hard.h` are treated normally.

The object path preserves the owner-relative source path. The original source
extension is preserved before `.o`, avoiding collisions between sources such
as `file.c` and `file.cpp`:

```text
/workspace/example/src/file.c
  -> HARD_ROOT/project/HARD_ENV/root/workspace/example/build/<build-key>/src/file.c.o

/workspace/example/src/file.cpp
  -> HARD_ROOT/project/HARD_ENV/root/workspace/example/build/<build-key>/src/file.cpp.o
```

Each successful compilation stores an atomic cache record beside its object.
The cache key includes the `hard` executable, compiler path and content,
complete compiler argument vector with the absolute source, source content,
every resolved active non-system include, and the generated source forward.
As with parsing, the invocation working directory is included only for
relative or opaque cwd-dependent compiler arguments. The directory key retains
the invocation's include context as well; direct library objects from distinct
projects therefore remain isolated, unlike context-independent recipe packages.
System headers and other toolchain state are represented by
`HARD_ENV`. A hit is accepted only when the object is still a regular file
with the recorded content digest. Missing, changed, malformed, or non-regular
artifacts and records are cache misses.

`--no-cache` disables build, run, and test cache reads for this invocation. It
forces source analysis, source-forward generation, compilation, and linking;
active compiled library packages are rebuilt, `hard build` also forces binary
delivery, and `hard test` reruns its tests.
Program execution by `hard run` is never cached and therefore happens on every
successful invocation with or without this flag. Fresh successful build records
are written. The flag does not remove or refresh downloaded GitHub snapshots
below `HARD_ROOT/snapshot`.

#### Entry points and linking

Only global function definitions whose names appear in `HARD_ENTRYPOINTS` make
a translation unit an entry source. The default list is:

```text
main _start
```

The value is parsed as shell-style words. An explicitly empty value disables
entry-point detection. Declarations without a body, class methods, namespace
functions, local functions, and lambdas are not entry points. Defining more
than one configured entry-point name in one source is an error. Detection reuses
the final full libclang AST with the effective compiler flags. Only the
active preprocessor branch participates, and a function definition produced
by a macro is detected.

Only entry sources selected by the original command are executable targets.
An implementation source discovered through a header is dependency-only and
never creates a separate binary.

Starting with the selected root sources, `hard` searches the directory of
every discovered non-system header for an implementation source with the same
filename stem and a supported source extension. A found implementation is
added to dependency discovery and compilation, and the process repeats until
no new sources remain. Header/include cycles are suppressed. For example:

```text
common/object.h   -> common/object.cpp
container.hpp     -> container.cc
```

The header and implementation must have the same canonical directory and stem;
their supported extensions may differ and are matched case-insensitively.
Multiple implementation candidates, such as both `object.c` and `object.cpp`,
are an error. A header with no matching implementation remains header-only.

For each root entry source, `hard` recursively follows the resulting
associations and links its own object plus the reachable non-entry objects.
Cycles are handled without duplicating objects. Other entry sources and
unrelated objects are excluded.

Each binary is linked in parallel with the selected job count using ordinary
compiler-driver linking:

```text
HARD_CC <entry-and-dependency-objects...> \
  <reachable-static-library-archives...> <HARD_LDFLAGS...> \
  -o <internal-binary>
```

Successful links are cached from the compiler fingerprint, link argument
vector, and the content of every linked object. The invocation working
directory participates only when a linker flag is relative or is an opaque
forwarded argument. The internal binary digest is verified before a hit is
used.

`hard` does not add `-nostartfiles`, `-e`, or another custom startup option for
non-`main` names. A configured entry point such as `_start` must therefore be
compatible with ordinary linking and the supplied `HARD_LDFLAGS`; otherwise
the linker failure is reported normally.

The internal binary preserves the owner-relative entry source path below its
configuration's build directory and removes the source extension:

```text
/workspace/example/src/application.cpp
  -> HARD_ROOT/project/HARD_ENV/root/workspace/example/build/<build-key>/src/application
```

When `HARD_EXECUTABLE_SUFFIX=.exe`, inferred internal binary names receive that
suffix instead:

```text
/workspace/example/src/application.cpp
  -> HARD_ROOT/project/HARD_ENV/root/workspace/example/build/<build-key>/src/application.exe
```

After successful linking, the binary is copied atomically to its delivery
path. Without `-o`, each binary is placed beside its lexical entry source with
the source extension removed:

```text
src/application.cpp -> src/application
```

`-o path/to/application` selects one exact destination and therefore requires
exactly one entry source. `-o path/to/bin/` selects a directory and preserves
each entry source path relative to the current working directory beneath that
directory. An existing directory is also recognized without a trailing slash;
missing parent directories are created. Sources outside the working directory
are mirrored from their lexical absolute paths instead of escaping through
`..`. Output path collisions are errors. An existing regular destination file
is replaced atomically; symlinks and other non-regular destinations are
rejected. With no entry source, `-o` produces no file and is not an error. The
internal artifact is retained after delivery.

The configured executable suffix is also appended to the default delivery path
and paths inferred below a directory `-o`. An exact file destination supplied
as `-o path` remains exact and is not rewritten.

Delivery is skipped when cache reads are enabled and the existing regular
destination already has the same content and permissions as the internal
binary. `--no-cache` forces the copy as well.

Output modes:

Search, source analysis, and repository downloads share the
first step of one progress counter. After preparation, the exact total is one
preparation step plus one step for every root or automatically discovered
source and two steps for every binary—one link and one copy. Repository
requests do not add separate steps. Until preparation finishes, updates use an
unknown denominator:

```text
[1/?] Searching source files
[1/?] Parsing example.cpp
[1/?] Downloading github.com/owner/repository@<commit>
[1/?] Parsing example.cpp (dependencies updated)
[1/?] Generating example.cpp.fwd.h
[2/4] Compiling example.cpp
[3/4] Linking example
[4/4] Copying example
```

A cached preparation analysis is reported as
`[1/?] Parsing <source> (CACHED)`.
A cache hit still completes its progress step and appends `(CACHED)`, for
example `[2/4] Compiling example.cpp (CACHED)`. Cached compile and link steps
do not print a verbose command because no process was started. A skipped
delivery step is reported as `Copying <binary> (CACHED)`.

Each download update is emitted immediately before its HTTP request. Cached
repositories omit `Downloading`, but search and parsing still occupy step one.
After source preparation, compilation always continues at
`[2/M]`. Normal mode rewrites one line, verbose mode writes permanent activity
lines, and silent mode hides the complete progress stream. All entries follow
`--no-color`.

- Normal mode updates one line with preparation activity or `[N/M] Compiling
  <source>`, `[N/M] Linking <binary>`, or `[N/M] Copying <binary>`.
- `-v` writes permanent progress entries. The exact compiler or linker command
  immediately follows its `Compiling` or `Linking` entry. Every argument is
  POSIX-shell escaped, so the command can be copied and run manually. Copying
  is internal Go code and has no command line.
- Repeated analysis appears as `Parsing <source> (dependencies updated)` or
  `Parsing <source> (library includes updated)`, including retries after the
  dependency view is refreshed. `Generating <source>.fwd.h` marks forward
  generation; a cached analysis restores the forward without this stage.
  Internal cache reasons, libclang call numbers, declaration statistics,
  skipped declarations, and timings are omitted. Tool errors retain their
  diagnostics.
- Build does not print a preliminary header list. Preparation progress may show
  `Parsing <source>` while libclang analysis is running.
- `-s` suppresses progress and successful compiler output; compiler, linker,
  and copy errors still go to stderr.

Snapshots are mapped through the project's selected repository links, so
`Parsing` and `Compiling` use `github.com/<owner>/<repository>/<path>`, for
example `github.com/hard-build/library/application/application.cpp`, even when
the file physically resides under `snapshot/<source>/@<commit>`. Well-known
aliases use the same canonical label. Replacements retain their
logical repository names. Cached progress entries use the same labels. This
affects only progress output: verbose compiler commands, diagnostics, object
paths, and other artifacts continue to use the actual source path.

For an initially unknown chain `main.cpp` → `libA1/a.h` → `libB2/b.h`, with
ordinary headers available directly in each snapshot, main is analyzed three
times: discover A, discover B, then analyze the complete include context.
The forward is generated once, from the last AST. Known recorded dependencies
are prepared before analysis and can eliminate those discovery attempts.
Recipe builds can require another attempt after their package flags change;
newly discovered implementation sources each have their own analysis.

Root translation units without a configured entry point remain object files.
Automatically discovered implementation sources are dependency-only even when
they define a configured entry point.

Cache records and artifacts that no longer belong to the selected dependency
graph are not removed automatically.

### `hard run`

```bash
hard run [--locked] [--no-cache] [-s|--silent] [path...] [-- program-argument...]
```

`run` selects ordinary non-test translation units by the same rules as
`build`, prepares their complete managed dependency closure, compiles the
required objects, links one internal binary, and executes it. It never performs
the `build` delivery step: the binary remains below its owner's
`build/<build-key>` directory, and no extensionless copy is created beside the
entry source.

Exactly one originally selected root source must define a configured entry
function. Finding zero entry sources is an error. Finding more than one is also
an error and lists the candidate source paths. This check happens after source
analysis and before object compilation. Automatically discovered implementation
sources remain dependency-only and cannot become the selected program.

Positional values before `--` are source files or directories. Values after
`--` are passed to the program unchanged and are never interpreted as `hard`
flags or paths:

```bash
hard run src/application.cpp -- --mode=check "input file.txt"
```

With no path before `--`, source selection defaults to `.`. The program runs
with the current invocation directory as its working directory and inherits
`hard`'s stdin, stdout, and stderr. `--no-color` affects only `hard` progress,
not program output.

The internal binary uses `HARD_EXECUTABLE_SUFFIX`. When
`HARD_EXECUTABLE_RUNNER` is non-empty, `hard run` invokes
`<runner> <binary> <arguments...>`; otherwise it executes the binary directly.

Parsing, compilation, and linking use the same content caches and invalidation
rules as `build`. `--no-cache` forces all three build stages. Execution itself
is never cached: an unchanged invocation may report cached parsing, compilation,
and linking, but still starts the program every time.

Search, source analysis, and downloads share preparation step one. After
preparation, the total is one plus the number of compiled sources plus one link
step. There is no `Copying` step. The progress stream is finished before the
program starts so child output remains live and does not overwrite progress.
Verbose mode prints the exact shell-escaped compile, link, and run commands.
Silent mode hides `hard` progress and commands while leaving the program's
stdout and stderr untouched.

A successful program makes `hard run` return zero. A normal nonzero program
exit is propagated as the `hard` process exit status without an additional
`hard:` diagnostic. Build, link, or process-start failures use the ordinary
`hard` error path and status 1.

### `hard fetch`

```bash
hard fetch [--lock | --locked | --update=<repository>@<ref>...]
           [--no-cache] [-s|--silent] [path...]
```

`fetch` downloads the external GitHub dependencies required by the selected C
and C++ translation units without building them. This includes repositories
named by active recipe wrappers and their transitive `.hard` dependencies.
Unlike `build`, its default recursive selection includes ordinary, `*.test.*`,
and legacy `*_test.*` translation units. Explicit files and directories use the common path-selection
rules.

Dependency analysis uses libclang 18 with skipped function bodies and the
effective compiler flags, follows active project headers, and recursively discovers same-stem
implementation sources. It then downloads the complete transitive closure of
expanded `github.com/<owner>/<repository>/...` and well-known includes.
`HARD_CC` is not started by `fetch`. The persistent cache and archive-safety
rules are the same as for `build`, `run`, and `test`; existing repository
directories are not refreshed automatically.

Successful dependency analysis is cached independently of build analysis at
`<owner>/parse/fetch/<context-key>/<relative-source>.hard-parse-cache.json`, using
the same owner rules for recorded and unrecorded projects. Fetch records never
substitute for build/run/test records and contain no entry points or generated
forwards. Unrecorded invocations restore an input-validated dependency selection
from a prior root-source parse record before analysis, so warm unchanged
commands do not need a preliminary rediscovery pass. Root source discovery
runs once even on a cold cache.

The key includes the hard executable digest, libclang version, ordered base
analysis flags, and contents of the source and every previously known active
non-system header and transitive `.hard` descriptors, including force-included
headers. Cwd-dependent flags also include the invocation directory. `HARD_ENV`
separates immutable toolchains and system headers; record selection keys separate dependency
revisions and context keys separate include contexts. Missing, changed,
malformed, or semantically inconsistent records cause fresh analysis. The same
`__has_include` guard and depfile-style include-path topology limitations
described for build analysis apply.

A hit skips libclang, restores the dependency list and recipe source includes,
and still discovers same-stem implementations. Stored include and recipe edges
are replayed to validate inherited requirements, and recipe vendors are
revalidated without building packages. Project/corporate configuration and snapshot checksums are
still checked; cache hits cannot bypass `--locked` or inherited-pin conflicts.
`--no-cache` forces fresh analysis and replaces successful records, but does not
update recorded revisions or redownload valid snapshots. Failed analysis does
not retain an eligible record. Use the flag after adding a higher-priority
header or changing optional-header availability inside a dependency.

When a recipe is active, `fetch` temporarily appends its
`source_include_directories` below the downloaded repository and repeats
dependency analysis. It does not start CMake or `HARD_CC`, install a package,
write a manifest, or create build artifacts. Headers belonging to these packages
do not trigger same-stem vendor implementation discovery: their C/C++ sources
belong to the external build and may require generated configuration files.
Ordinary project headers still discover neighboring implementations as before.

Before the external build, public vendor headers may include configuration
headers that do not exist yet. During `fetch`, unresolved ordinary includes
originating inside an active recipe's source directory are deferred to that
package's build. Recipe wrappers themselves remain strict, even within a vendor
source directory. Ownership is checked on each including file, so an identical
missing include in the project still fails. Missing recipe wrappers, `.hard`
references, and GitHub/well-known includes remain errors, including inside a
vendor source tree. The check also runs after cache invalidation and leaves
`build`/`run`/`test` validation of installed headers unchanged. Wrappers do not
need conditional prebuilt headers or special CMake settings for fetch.

Fetch discovers branches visible with the currently available headers and
macros. It cannot guarantee discovery of dependencies hidden behind macros
from headers that only the vendor build generates. Dependencies that fetch
must obtain before configuration should be explicit in YAML `dependencies`;
the final build analysis determines the actual wrapper variant and includes.

This command does not generate forward headers, compile objects, link or copy
binaries, run tests, or create an environment build tree. An empty selection
is a successful no-op. `-j` limits concurrent libclang analyses.

Search, dependency parsing, and every actual request reuse one command
preparation step. Live activity is shown as `[1/?] Searching source files`,
`[1/?] Parsing <source>`, or
`[1/?] Downloading <source>@<commit>`; each download label is
emitted immediately before its HTTP request. The exact final total is one.
Normal mode rewrites one line, `-v` writes permanent activity lines, and `-s`
suppresses successful progress. Colors obey `--no-color`. Cached repositories
omit `Downloading`; reused analysis reports `Parsing <source> (CACHED)`:

```text
[1/?] Searching source files
[1/?] Parsing main.cpp (CACHED)
[1/?] Parsing github.com/leethomason/tinyxml2/tinyxml2.cpp (CACHED)
```

### `hard test`

```bash
hard test [--list-tests] [--test=<selector>]... \
  [--locked] [--no-cache] [-s|--silent] [path...]
```

Every selected test source is built and run as a separate executable.

Without a selection flag, every test in every selected executable runs.
`--list-tests` builds the selected executables and prints the test names
reported by them without running the tests. For one selected source, the
normalized output contains one full name per line:

```text
Random.ReturnsValue
Random.RejectsInvalidRange
SeededRandom.IsRepeatable
```

For multiple selected sources, each list is grouped below the lexical source
path:

```text
tests/random.test.cpp:
  Random.ReturnsValue
  Random.RejectsInvalidRange

tests/parser.test.cpp:
  Parser.AcceptsValidInput
  Parser.RejectsInvalidInput
```

`--test=<selector>` runs only matching full test names and may be repeated.
A selector without wildcards is exact. `*` matches any number of characters,
including zero, and `?` matches exactly one character:

```bash
hard test --test=Random.ReturnsValue tests/random.test.cpp
hard test --test='Random.*' tests/random.test.cpp
hard test --test='Parser.Test?' tests/parser.test.cpp
hard test \
  --test='Random.Returns*' \
  --test='Parser.Test?' \
  tests
```

Quote selectors containing `*` or `?` so the invoking shell does not expand
them. Repeated selectors form one positive selection. Empty selectors, `:`,
and `-` are rejected; negative filtering is not part of the public interface.
`--list-tests` and `--test` cannot be combined.

Before filtered execution, `hard` asks every successfully linked executable
for its actual test list. Every selector must match at least one test across
the complete invocation. A selector may match no tests in an individual
executable when it matches another selected executable, but a selector that
matches nowhere is an error and no filtered test execution begins. Internally,
`hard` converts the validated selectors to the corresponding GoogleTest
filter. GoogleTest-specific command-line arguments are not part of the
`hard test` interface.

The list produced by `--list-tests` is command output rather than progress.
It is therefore written to stdout even with `--silent`; that flag still
hides search, parse, compile, link, and listing progress.

Before processing a non-empty selection, `hard` obtains GoogleTest flags with:

```text
pkg-config --cflags gtest_main
pkg-config --libs gtest_main
```

The outputs are parsed as shell-style argument vectors with environment and
command substitution disabled. GoogleTest compiler flags are appended after
the backend-effective base compiler flags; active package include flags follow
recipe discovery. Its linker flags are appended after `HARD_LDFLAGS`. Failure
to start `pkg-config`, a nonzero result, or malformed output stops the command
before any test is built. An empty selection succeeds without requiring
`pkg-config` or GoogleTest.

For each test source, `hard` uses the build dependency analyzer to recursively
find same-stem, non-test implementation sources required by its non-system
headers. Other `*.test.*` and legacy `*_test.*` sources are never added
automatically. It prepares one source-context forward for every test and
production translation unit, then compiles them with the combined compiler flags. Dependency closures for
different test roots are prepared concurrently. An object output shared by
several test plans is compiled only once, and every source forward belongs only
to its translation unit. The shared object is then reused by every test that
reaches it. The canonical runtime support header remains force-included,
but its declarations do not enter generated source forwards, as in `build`.
`HARD_ENTRYPOINTS` is ignored for this command because `gtest_main` supplies
the test executable entry function.

The same GitHub snapshot resolver is shared by every test plan, so missing
expanded `github.com/` and well-known includes use the build download and cache
rules described above. Downloaded repositories are handled as managed source
trees: their headers contribute to source forwards, and same-stem
implementations participate in the test build. Their `Compiling` labels use
canonical `github.com/...` paths. Search, source parsing, and all live download
entries share preparation step `[1/?]` for the test invocation. Downloads do
not add a separate step. After preparation, the exact total includes that first
step and
compilation continues at `[2/M]`, whether dependencies were downloaded or
already cached.

Active `.hard` recipes use the same package build and cache rules as
ordinary builds. Each test translation unit receives only its own active
package include directories, and each test binary links only the static
archives reachable from that test source closure.

The test source object and all reachable production objects are linked with
ordinary compiler-driver linking:

```text
HARD_CC <test-and-dependency-objects...> \
  <reachable-static-library-archives...> \
  <HARD_LDFLAGS...> <gtest_main-linker-flags...> \
  -o <internal-test-binary>
```

The internal binary follows the same owner-relative path rule as a build binary:

```text
/workspace/example/tests/random.test.cpp
  -> HARD_ROOT/project/HARD_ENV/root/workspace/example/build/<build-key>/tests/random.test
```

With `HARD_EXECUTABLE_SUFFIX=.exe`, the corresponding path ends in
`random.test.exe`. Test listing and execution use
`HARD_EXECUTABLE_RUNNER` in the same way as `hard run`.

Test binaries remain in the environment build tree and are not copied into
the project. The `test` command does not accept `-o`.

Source parse records use the same persistent analysis cache. A hit is reported
as `Parsing <source> (CACHED)`.
Compilation and linking use the same content cache as `hard build`. After a
test executable exits successfully, `hard` also stores a successful-result
record beside it. The converted selector vector is part of that record's key,
so exact tests, wildcard selectors, and selector combinations are cached
independently. Repeating an unchanged filtered invocation can therefore report
`Testing <binary> (CACHED)`.

Listing is never cached as a successful test result because its normalized
output is the requested result. A repeated `--list-tests` invocation executes
the lightweight discovery mode again while still reusing eligible parsing,
compilation, and linking artifacts. Selector validation also performs real
discovery before considering a cached filtered result.
An unchanged binary with the same test arguments and working directory is not
run again; its progress entry is `Testing <binary> (CACHED)`.
Failed tests are never cached, and a record is invalidated before an actual
execution so an interrupted or failed forced run cannot leave an older success
eligible for reuse. Runtime inputs outside the binary—such as undeclared files,
environment-dependent services, network responses, or time—cannot be inferred;
use `hard test --no-cache` when those inputs matter.

The work is divided into invocation-wide phases:

1. prepare the dependency closure and source forward of each selected test;
2. compile each unique object;
3. link every test whose required objects compiled successfully;
4. list tests when `--list-tests` or `--test` was requested;
5. validate selectors and run every successfully linked test unless the
   command is list-only.

Each phase uses at most the selected `-j` worker count without multiplying
that limit through nested pools. Link jobs and test executables from different
test files therefore run concurrently. List-only progress uses
`Listing <binary>` instead of `Testing <binary>`. Filtered execution has
both steps for every successfully linked binary. A preparation, compilation,
or link failure skips only test plans that require the failed work; independent
tests continue.
A nonzero test result is recorded while other tests continue. The command
returns nonzero after all safe independent work has been attempted if any
preparation, compilation, link, progress-output, process-start, or
test-execution step failed.

All successfully prepared test plans share one progress counter. Its total is
one preparation step plus the number of unique compiled sources and one link
step for every test executable. A normal invocation adds one test step per
executable. A list-only invocation adds one listing step instead. A filtered
invocation adds both listing and testing steps. A shared production source
contributes one compilation step even when several tests use it.

Four header-only tests without selectors use one continuous counter:

```text
[1/?] Searching source files
[1/?] Parsing first.test.cpp
...
[2/13] Compiling first.test.cpp
...
[6/13] Linking first.test
...
[10/13] Testing first.test
...
[13/13] Testing fourth.test
```

Within a phase, entries appear in completion order and may therefore differ
from discovery order.

Normal mode updates one progress line for the complete invocation. Test stdout
and stderr are captured: output from a successful test is discarded, while
output from a failed test is written after the progress line is finished. `-v`
uses permanent progress entries and prints the exact shell-escaped compile or
link command after its completed entry. Each parallel test's command and
captured output are printed as one contiguous block immediately after that
test finishes, so output from different test executables does not interleave.
`-s` suppresses progress, verbose commands, successful tool diagnostics, and
successful test output while retaining compiler/linker errors and failed test
output; silent mode takes precedence over verbose mode. `--no-color` disables
progress colors and passes `--gtest_color=no` to every test executable.
Otherwise, `hard` passes `--gtest_color=yes` so captured GoogleTest output keeps
its ANSI colors in verbose output and when a failed test's output is reported.

## Project configuration and pinned dependencies

`hard.yaml` is optional, is committed with the project, and has exactly four
supported top-level fields:

```yaml
version: 1
format: format.v1
exclude: [build, bin]
repositories:
  github.com/leethomason/tinyxml2:
    source: github.com/leethomason/tinyxml2
    ref: "10.0.0"
    commit: "<full lowercase Git commit ID>"
    checksum: "sha256:<64 lowercase hexadecimal digits>"
```

The commit and checksum above are placeholders, not valid records. An empty
`repositories: {}` is valid and enables recording. Unknown or duplicate fields,
non-string keys, anchors, aliases, merge keys, custom tags, extra documents,
non-regular configuration files, and a flow-style top-level mapping are
rejected. Repository fields must be strings; commits are full 40- or 64-digit
lowercase hexadecimal IDs. The top-level mapping uses block style so automatic
updates can preserve other settings byte-for-byte.

Search starts at the invocation directory and ascends until the nearest
`hard.yaml`, the current Git root (a `.git` file or directory), or the
filesystem root. One invocation selects one configuration; explicit source
paths do not load or merge other project files. `version`, `environment`, help,
and completion do not load project configuration or fetch dependencies.

- `version` is required and must be integer `1`, the schema version.
- `format` is optional; absent means `format.v1`, empty/null is invalid, and an
  explicit `--format` overrides it. Styles remain installed runtime files.
- `exclude` is an optional list, empty by default. Literal relative file and
  directory paths are relative to the YAML directory, not the invocation
  directory. Glob patterns and Git ignore syntax are not supported.
  Traversal skips matching paths and directory subtrees; explicitly selecting
  a file overrides exclusion. Exclusions do not suppress headers or same-stem
  implementation sources needed through the active include graph.
- `repositories` enables project recording by its presence, even when empty.
  Omission uses cached defaults and inherited pins without writing hard.yaml.
  Each logical key remains
  `github.com/owner/repository`; `source` can identify another upstream or a
  corporate fork without changing includes or recipe source names.

Source-selection paths are supplied only on the command line and are relative
to the invocation directory. Without them, selection starts at `.` even when
the configuration is in a parent directory. A `paths` project field is unknown
and rejected. Exclusion paths cannot escape through `..` or contain wildcard
syntax.
Directory-symlink traversal and canonical file deduplication otherwise retain
the ordinary source-selection rules. No target, compiler flags, profiles,
command hooks, or extra project sections are supported.

### Recording and updating

`hard fetch --lock` creates `hard.yaml` in the current directory if none was
found, or adds `repositories` to the selected file. Ordinary `fetch`, `build`,
`run`, and `test` use recorded revisions and add new dependencies automatically
when recording is enabled. Without it, the selected revisions remain in memory
for that invocation and sources are still verified snapshots. `fetch --lock`
can record an existing cached default; when its branch spelling is unknown,
the full commit is recorded as `ref` as well as `commit`.
Only active include graphs are discovered, including recipe and vendor-source
repositories; separate platforms may add different entries. One logical
repository has one selected revision. There is no semantic-version solver.

Existing project records constrain versions without activating dependencies.
Only repositories reached by the selected sources' active includes or recipes
are downloaded, verified and exposed in the include view. Unused records remain
in `hard.yaml`, including when a previously used dependency becomes inactive.
Explicit `--update` requests also obtain and verify the named repositories even
when the current source selection does not use them.

When an active include or recipe requests a dependency from a downloaded
repository, hard reads `hard.yaml` at the root of that repository's selected
snapshot. If its `repositories` section records a dependency not yet recorded
by the consuming project, hard inherits the exact `source`, `ref`, `commit`,
and `checksum`; it does not resolve the recorded branch or tag again.
This applies recursively to ordinary includes,
well-known aliases, and recipe vendor sources. The same strict project-file
validation applies. A missing file or missing entry retains default-branch
resolution for a new dependency; malformed configuration or a bad checksum for
the selected snapshot is an error, not permission to fall back.

Only dependencies actually requested by active includes or recipes are added.
Other entries in a downloaded manifest are not fetched or copied just because
the manifest exists. Its `format` and `exclude` settings do not affect the
consuming project, and hard does not search snapshot ancestors or nested
directories for another configuration.

The consuming project's recorded `source`, `ref`, `commit`, and `checksum` take
precedence over inherited records. This also resolves disagreements between
multiple downloaded repositories: all use the one project-selected revision.
No additional override field is needed. Any recorded entry is a project choice,
including one previously inherited automatically. Updating a recipe repository
does not implicitly update already recorded vendor dependencies.

Without a recorded project choice, active inherited requirements must agree on
source, commit, and checksum. Different `ref` names for identical contents are
compatible. A conflict names the repository, both selections and their origins,
and leaves the project file unchanged. During initial discovery,
an inherited record can replace a provisional default-branch choice that has
not yet been written; source analysis then restarts with the selected revision.
Hard does not choose between incompatible inherited requirements by discovery
order or attempt version-range solving.

`hard fetch --update=github.com/owner/repository@ref` updates an already recorded
repository; repeat the option for several distinct repositories. Duplicate
updates are errors. A new dependency should first be discovered with ordinary
fetch or `fetch --lock`. `--lock`, `--locked`, and updates are mutually
exclusive. Only fetch accepts `--lock` and `--update`; build, fetch, run, and
test accept `--locked`.

Explicit updates override inherited pins. For example,
`hard fetch --update=github.com/leethomason/tinyxml2@10.0.0` selects `10.0.0`
even when `recipe/hard.yaml` records `11.0.0`, without editing or forking the
recipe repository. Repeating `fetch --lock` preserves that project selection.
This permits an override; it does not guarantee API, ABI, or recipe compatibility
with the chosen version. Existing corporate replacement restrictions still
apply to recorded entries and explicit updates.

Direct GitHub revision lookup requests only the full commit SHA, not the commit's
metadata and patches, so large commits do not require large JSON responses.
The SHA response is size-limited and validated; malformed or truncated responses
fail without changing the project record. The corporate proxy's JSON protocol
is unchanged.

`--locked` requires an existing repositories section and fails on an unrecorded
dependency without resolving its branch. Recorded snapshots absent from the
cache may still be downloaded: this is not offline mode. Known branch/tag
references never move implicitly. `--no-cache` forces analysis in `fetch` and
analysis/artifact work in build commands, without updating revisions.
An inherited record does not authorize an addition under `--locked`. Existing
project selections retain their precedence over inherited records in this mode.

The `ref` records intent; `commit` selects the snapshot. Editing only `ref` does
not resolve or select another revision; use `--update` to change it consistently.
Downloads and cached snapshots must match `checksum`, including during explicit
updates that resolve to an already recorded source and commit. A mismatch is
fatal, not a request
to rewrite the checksum. The checksum is trust-on-first-use integrity, not a
signature or proof of the original publisher's identity.

Updates are committed atomically after dependency resolution succeeds, before
ordinary object compilation or program execution. A later compiler or program
failure does not undo a successfully resolved record. Failed resolution leaves
the file unchanged. Only the repositories section is serialized; other bytes
and comments are preserved. Directory-inode advisory locking serializes hard's
updates, and a changed original file is rejected instead of overwriting an
editor's concurrent changes. This YAML lock is released after resolution.
A separate cache-owner lock protects the single include view until the command
finishes, including run/test child processes. A child must not recursively
invoke hard in the same directory and environment while its parent holds this
lock. Downloaded snapshots may remain after a failed command.

### Pinned source and artifact layout

Recorded and unrecorded projects use the same layout. Snapshots are shared
across projects and environments:

```text
HARD_ROOT/snapshot/<actual-source>/@default
HARD_ROOT/snapshot/<actual-source>/@<commit>/
HARD_ROOT/snapshot/<actual-source>/@<commit>.checksum
HARD_ROOT/project/<HARD_ENV>/root/<absolute-project-dir>/include/
HARD_ROOT/project/<HARD_ENV>/root/<absolute-project-dir>/parse/fetch/<context-key>/
HARD_ROOT/project/<HARD_ENV>/root/<absolute-project-dir>/parse/build/<context-key>/
HARD_ROOT/project/<HARD_ENV>/root/<absolute-project-dir>/build/<build-key>/
HARD_ROOT/project/<HARD_ENV>/<logical-repository>/parse/fetch/<context-key>/
HARD_ROOT/project/<HARD_ENV>/<logical-repository>/parse/build/<context-key>/
HARD_ROOT/project/<HARD_ENV>/<logical-repository>/build/<build-key>/
HARD_ROOT/project/<HARD_ENV>/<logical-repository>/package/<fingerprint>/
```

`<absolute-project-dir>` is the invocation directory without its leading slash.
The [annotated tree](cache-layout.md) explains each file and directory and shows
both a hard/library consumer and a project with a local TinyXML2 recipe.

Include-view entries are relative symlinks to exact snapshots. One locked view
is reused and refreshed when dependencies change; no selection-digest or
dependency-set directory is created. Build keys include selected snapshot paths
and the complete relevant configured context, but not ordinary local source
contents. Parse paths use a context key without selected snapshots; each record
validates its selection and content inputs before reuse.
Direct source keys retain the invocation and include-view paths, so distinct
projects' direct objects remain isolated even under the same logical repository.
Vendor package keys instead cover recipe contents, their source snapshot and
build tools, allowing compatible cross-project reuse. Removing a replacement
rule without changing selected contents does not invalidate analysis.

For both recorded and unrecorded projects, a successful root-source parse record
also stores the last required selection for one command, root-source list and configuration.
It fingerprints the hard executable, analyzed sources and active non-system
headers, and used `@default` files. Restoration occurs under the include-view
lock only when these inputs match; malformed or stale records are ignored.
Snapshots still require checksum validation, and inherited pins are checked
through active include edges. This cached selection is not a project pin:
changing an input starts fresh discovery rather than preserving a now-inactive
inherited choice. The project configuration must match, and restored choices
must agree with current project pins and explicit updates. Newly written pins
are reflected in the hint so the next invocation can reuse it. `--no-cache` bypasses it,
and sources covered by the existing `__has_include` guard prevent its
publication. The ordinary include-path topology limitations also apply.
Only the last parse result per source/context is kept, so switching revisions
can require another parse. The old standalone `dependencies.json` is ignored,
not removed. Searching root sources is done once per invocation, outside any
dependency-resolution retries.

`@default` is a regular commit-ID file, not a symlink. It is published under
the source-directory lock after successful snapshot verification. It has no
separate checksum: the selected `@<commit>.checksum` validates the source tree.
The reserved `@` prefix separates revision metadata from nested corporate
repository paths. Project pins and inherited requirements override the default;
explicit project updates and `--no-cache` do not change it.

Required snapshots are hydrated and verified when constructing a view;
unrequested records do not cause downloads or snapshot validation. Fetch creates only dependency-analysis
records, not build artifacts. Existing unversioned source trees and hashed
snapshots are neither adopted nor deleted; a first build with this layout may
download and compile again. No cache garbage collection is performed.
Treat snapshots as immutable: changed contents, missing checksum metadata, or
source mutations during recipe preparation are errors. Managed cache parent
directories cannot be symlinks; HARD_ROOT itself may be an intentional symlink.

The normalized tree digest is SHA-256 over the JSON-encoded string
`hard-source-tree-v1` followed by lexically ordered depth-first JSON array
records `[relativePath, kind, executable, value]`, each followed by a newline.
The root itself is omitted; directory values are empty, file values are SHA-256
content digests, and symlink values are their literal targets. The executable
boolean reflects any executable permission bit on regular files. Archive root
names, timestamps, ownership and non-executable permission differences are
excluded. Extraction retains the normal single-root, safe-path and safe-symlink
rules; archives from proxies use the same tar.gz format as GitHub snapshots.

### Corporate sources, credentials and proxy protocol

Set `HARD_CONFIG` to a separate regular YAML file:

```yaml
proxy:
  url: https://dependencies.corp.example
  fallback: false
replace:
  github.com/leethomason/tinyxml2:
    source: git.corp.example/third-party/tinyxml2
    ref: "10.0.0-company.2"
auth:
  dependencies.corp.example:
    token_env: HARD_AUTH_DEPENDENCIES
```

`HARD_PROXY`, when set, overrides the proxy URL (an empty value disables it).
Proxy and replacement settings are not copied into project top-level fields.
Replacement selections appear as repository `source`, `ref`, commit and checksum.
A rule conflicting with an existing source/ref is an error until an explicit
matching update is requested. With corporate configuration present, commands
require recording or `fetch --lock`.

An explicit replacement that changes an inherited source or requested ref
overrides that upstream requirement, allowing a corporate fork without editing
the recipe repository. It still cannot change an existing project pin without
`--update`. A replacement identical to the inherited source/ref retains its
exact commit and checksum. Proxy-only changes do not override inherited pins.
Removing a replacement rule does not revert an already recorded fork: the
project selection remains authoritative over upstream manifests.

Authentication is optional and keyed by exact `host[:port]`. `token_env` names
an environment variable with the `HARD_AUTH_` prefix; its non-empty value is
sent as a Bearer token only to that host. Values are never written to YAML.
The container wrapper does not forward these credential variables or
`HARD_CONFIG` from the host. URLs cannot contain userinfo, query credentials or fragments.
HTTPS is required except for literal loopback IP HTTP endpoints used in tests.
Direct GitHub redirects can use another HTTPS host but do not carry the previous
host's authorization. Proxy redirects cannot leave the proxy origin.

The implemented client protocol appends these paths to the configured base URL:

| Request | Successful response |
| --- | --- |
| `GET /v1/resolve?source=<encoded-source>&ref=<encoded-ref>` | HTTP 200 JSON with `commit` and `ref`; empty requested ref asks for the default branch and requires a non-empty response ref |
| `GET /v1/snapshot?source=<encoded-source>&commit=<full-commit>` | HTTP 200 tar.gz source snapshot at that exact commit |

Both resolution and download go through the proxy. Direct source support is
currently GitHub; other source hosts require the proxy. `fallback: true`
explicitly permits direct GitHub fallback only after proxy HTTP 404, 502, 503
or 504. Authentication failures, network/TLS errors, malformed responses,
disallowed redirects and checksum mismatches never trigger fallback. Server
response bodies and transport URLs are not included in diagnostics, to avoid
exposing credentials. The proxy server itself is not part of hard.

## Configuration

`hard` reads configuration from environment variables:

| Variable | Purpose | Default |
| --- | --- | --- |
| `HARD_ROOT` | Persistent source and artifact root | `~/.local/share/hard` |
| `HARD_ENV` | Isolated build-environment name | `host` |
| `HARD_CC` | Compiler executable | `c++` |
| `HARD_CFLAGS` | Project toolchain flags for libclang and object compilation | See below |
| `HARD_LDFLAGS` | Executable linker flags | See below |
| `HARD_ENTRYPOINTS` | Global entry-function names | `main _start` |
| `HARD_EXECUTABLE_SUFFIX` | Suffix for inferred executable paths | Empty |
| `HARD_EXECUTABLE_RUNNER` | Executable used to start built binaries | Empty |

Unset or empty `HARD_ROOT`, `HARD_ENV`, and `HARD_CC` use their defaults.

Default compiler flags:

```text
-std=c++20
-O3
-flto=auto
-Wall
-Wextra
```

Regardless of this value, the backend then appends
`-I<project-owner>/include` and `-include <runtime-root>/hard.h`. The runtime root
is not configured by an environment variable: `hard` derives it from the
physical path of the running backend executable. This keeps source resolution
and the runtime support header available even when `HARD_CFLAGS` is explicitly
empty, while allowing the host runtime bundle and container image to remain
self-contained.

Hard scans immediate version directories below `<runtime-root>/lib/clang` for
`include`. When exactly one exists, hard appends it as an `-idirafter`
directory only to libclang analysis. The argument is not part of the configured
or compiler-effective `HARD_CFLAGS`, so verbose compiler commands do not
contain it. No matching directory is normal for a system-installed libclang.
A present non-directory or inaccessible resource path and multiple matching
resource directories are errors.

Default linker flags:

```text
-std=c++20
-O3
-flto=auto
-Wall
-Wextra
-static-libgcc
-static-libstdc++
```

When `HARD_CFLAGS` or `HARD_LDFLAGS` is present, its value is parsed as
shell-style arguments and replaces the complete default vector. Quoting is
honored, but environment expansion and command substitution are disabled. An
explicitly empty value means no user-provided flags in that category. The
hard-managed source include and runtime support include still apply.

`HARD_ENTRYPOINTS` follows the same shell-style parsing and disabled expansion
rules. Unlike `HARD_ROOT`, `HARD_ENV`, and `HARD_CC`, an explicitly empty value
does not select the default: it disables `hard build` binary linking while
preserving object compilation, makes `hard run` fail because it has no entry
target, and has no effect on `hard test`.

`HARD_EXECUTABLE_SUFFIX` is read as one literal value. It must be empty or
start with `.` and must not contain `/` or `\\`. A non-empty value is appended
to inferred internal build and test binaries, default build delivery paths, and
paths inferred below a directory `-o`. It is not duplicated when the inferred
path already ends with the exact suffix. A file destination supplied exactly
through `-o path` is never rewritten.

`HARD_EXECUTABLE_RUNNER` is one executable name or path, with no shell-word
splitting. Empty means direct execution. A non-empty value changes the process
vector for `hard run`, GoogleTest listing, and GoogleTest execution to
`<runner> <binary> <arguments...>`. It is also part of the successful-test
cache fingerprint. Neither value is inferred from `HARD_ENV`; images and host
toolchain configurations must set them explicitly when required.

`HARD_ENV` is the cache boundary for immutable toolchain state. Use a distinct
value whenever the compiler, libclang resource headers, standard library,
libc, sysroot, ABI, target, container, or a user-provided system-include tree
supplied through `-isystem` or `-idirafter` changes. System headers are not
content-hashed; keeping the same `HARD_ENV` asserts that they remain compatible
and unchanged. `--no-cache` can force a one-off rebuild in the current
environment, while a new `HARD_ENV` keeps old artifacts isolated. Artifact
generation requires HARD_ENV to name one directory component: empty names,
`.`/`..`, absolute paths and path separators are invalid after default selection.

These variables describe host-mode execution. Target mode does not forward
their host values into the container; container images use the fixed values
listed under [Container targets](#container-targets). The host-side
`HARD_ROOT` value is still used to select the directory mounted at `/hard`.

Example:

```bash
export HARD_ROOT="$HOME/.local/share/hard"
export HARD_ENV=clang-debug
export HARD_CC=clang++
export HARD_CFLAGS='-std=c++20 -O0 -g -Wall -Wextra'
export HARD_LDFLAGS='-std=c++20 -O0 -g'
export HARD_ENTRYPOINTS='main _start'

hard build -j src
hard fetch tests
hard run src/application.cpp -- --mode=check
hard test tests
```

## Build, installation, and artifact layout

From the repository root, `make` builds the Go backend as `build/hard`.
`make install` uses this user-local layout by default:

```text
~/.local/
├── bin/hard
├── libexec/hard/
│   ├── default-target
│   ├── hard
│   ├── hard.h
│   └── format/
│       └── format.v1
└── share/
    ├── bash-completion/completions/hard
    ├── zsh/site-functions/_hard
    ├── fish/vendor_completions.d/hard.fish
    └── hard/
```

The public `bin/hard` command is a POSIX shell wrapper. It derives the logical
installation prefix from its own location in `<prefix>/bin` and uses the
sibling `<prefix>/libexec/hard` directory only for host execution and the
installed default target. Without `--target`, it reads
`libexec/hard/default-target`; a missing file preserves the host default.
`--target=host` replaces the wrapper with the host backend and prefixes its
bundled tool directory to `PATH`. `--target=linux64`, `--target=windows64`, an
explicit syntactically valid tag for either repository, or
`--target=docker://image` executes the
documented `docker run` invocation without resolving the host runtime. It
never builds an image.

The wrapper also owns completion values for `--target`. It answers that value
position directly for both Cobra completion protocols, combines stable target
names with validated GHCR tags, and uses the five-minute completion cache
described under [Installation](#installation). Other completion requests are
forwarded to the installed host backend, regardless of `default-target`, so
completion never invokes Docker. The Go backend keeps only the synthetic
target flag declaration needed when generating the Bash, Zsh, and Fish
completion scripts; concrete target values are not compiled into it.

`PREFIX` defaults to `$HOME/.local`, `BUILD_DIR` defaults to `build`, and
`DESTDIR` can stage an installation without changing its logical prefix.
Because the wrapper derives the prefix at runtime, installations under another
`PREFIX` and staged installations preserve the same relative layout without
rewriting the wrapper. `make install` is host-only: it does not invoke Docker
or install target images or container assets, and it writes `host` to
`default-target`.

The portable archive extends the runtime bundle with the host backend's shared
libraries, LLVM resource headers, `bin/clang-format`, license records, and the
three completion files shown above. Its top-level
`bin/hard` can execute that bundle in place immediately after extraction.
The installer places the runtime below `~/.local/libexec/hard` without a
`default-target`, so host remains the default.
It does not put runtime assets below `HARD_ROOT`.

The backend, support header, and format files form an immutable runtime bundle:

```text
<runtime-root>/
├── hard
├── hard.h
└── format/
    └── format.v1
```

Generated artifacts and source snapshots use the separate
[persistent cache layout](cache-layout.md). Source snapshots live below
`snapshot/<actual-source>/@<commit>`. Each environment's local project and
logical repository owners live below `project/<HARD_ENV>`; their analysis,
build and package directories are described in that annotated tree.
Local source edits invalidate content records rather than creating a new
configuration directory. `fetch` writes only its separate analysis records.
Legacy top-level `source`, `fetch` and `env` directories are not created by
the current backend, and existing old caches are left untouched.

An entry source or test source normally creates an extensionless internal
binary beside its object, such as `application` beside `application.cpp.o`.
A configured `HARD_EXECUTABLE_SUFFIX` changes inferred names, for example to
`application.exe`. Build binaries are delivered according to `-o` or beside
their lexical entry sources by default. Run and test binaries are not copied
out of the build tree.

`make install` supplies `hard.h` and `format.v1` beside the host backend. The
container image supplies its own copies beside its own backend. `HARD_ENV`
therefore isolates generated artifacts and toolchain state but no longer owns
runtime assets. The build tree can hold artifacts for multiple projects
without placing intermediate files in those projects.

Downloaded repository snapshots are shared by all `HARD_ENV` values below one
`HARD_ROOT` and remain in place until removed explicitly.

## Exit status

Commands return zero when all requested work succeeds. Invalid paths, invalid
configuration, missing tools or support files, parsing errors, formatter
failures, compiler failures, linker failures, failed test executables, invalid
library recipes, and CMake package failures produce a nonzero status. GitHub
request, archive-validation, extraction, and installation failures during
build, fetch, run, or test also produce a nonzero status. `hard run` propagates
the nonzero exit status of a normally started program; other command failures
return status 1.

Where independent work can continue safely, `hard` processes it and returns an
aggregate failure when the phase completes. Failures to start a required tool
stop new work in that phase.

## License

`hard` is available under the [MIT License](../LICENSE).
