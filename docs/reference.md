# Hard Build Reference

This document defines the complete user-facing behavior of `hard`. For an
overview and first build, start with the [project README](../README.md).

The public interface contains only `environment`, `format`, `build`, `fetch`,
`run`, and `test`.

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
the six public commands, command flags, filesystem paths, `host`, `linux64`,
`windows64`, the `linux64:v3.0-ubuntu.22.04` and
`linux64:v3.0-alpine.3.22-static` versions, the
`windows64:v4.0-llvm-mingw.20260616-ucrt` version, `docker://`, and the default
`format.v1` format. The wrapper itself supplies target values. Other dynamic requests
always execute the installed host backend, even when a container target is the
installed default, so pressing Tab never starts or pulls a Docker container.
POSIX shells without programmable completion and PowerShell do not receive a
completion integration.

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
- CMake when an active source include contains a `hard.recipe.v1` recipe;
- `clang-format` for source formatting;
- `pkg-config` and GoogleTest's `gtest_main` package for test compilation and
  linking;
- network access to GitHub when a referenced `github.com/<owner>/<repository>/`
  or well-known repository snapshot is not already cached below
  `HARD_ROOT/source`.

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
hard environment
hard format [--format=<name>] [-s|--silent] [path...]
hard build  [--no-cache] [-s|--silent] [-o <path>] [path...]
hard fetch  [-s|--silent] [path...]
hard run    [--no-cache] [-s|--silent] [path...]
            [-- program-argument...]
hard test   [--list-tests] [--test=<selector>]...
            [--no-cache] [-s|--silent] [path...]
```

### `hard environment`

```bash
hard environment
hard --target=linux64 environment
```

`environment` writes a human-readable report describing where and how the
current invocation would build binaries. It takes no paths, does not search a
source tree, and does not create or read build artifacts. The report contains:

- the resolved hard executable, runtime root, `HARD_ENV`, and `HARD_ROOT`;
- operating-system identity from `/etc/os-release`, kernel and architecture
  from `uname`, the first CPU model from `/proc/cpuinfo`, the logical CPU
  count, and libc from `getconf GNU_LIBC_VERSION` with `ldd --version` as a
  fallback;
- `HARD_CC`, its resolved executable, the first `--version` line, and its
  `-dumpmachine` target;
- the configured executable suffix and runner;
- effective compiler flags including the hard-managed includes, linker flags,
  and entry points, rendered as one shell-quoted argument per line;
- the libclang API version and the single portable Clang resource directory,
  or `system default` when the runtime supplies none.

Failure of an individual system or toolchain probe is printed as
`unavailable`; the remaining diagnostics continue. Invalid `HARD_*`
configuration and failure to write the requested report remain command errors.
The output has a title, aligned fields, and separate runtime, system, compiler,
build-configuration, and parser sections. By default the title and labels use
cyan, section headings use green, and `unavailable`, `none`, `direct`, and
`system default` use yellow. `--no-color` preserves the complete layout while
removing all ANSI sequences. The command has no `--silent` flag because the
report itself is its result.

### Container targets

The POSIX wrapper accepts `--target=<name>` and `--target <name>` anywhere
before `--`. `host` executes the private backend from the same installation
prefix. Container execution accepts the latest `linux64` or `windows64` image,
their documented versioned forms, or any image reference prefixed with
`docker://`:

```bash
hard --target=host build src
hard --target=linux64 build src
hard test --target linux64:v3.0-ubuntu.22.04 tests
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
not interpreted by the wrapper. Empty, repeated, malformed, and unknown named
targets are errors. The legacy `linux.v1` spelling is not supported.

For a container target, the wrapper only executes `docker run`; it never builds
an image or resolves the host runtime. The image entrypoint runs the container
backend. Unversioned forms check for a newer image on every invocation:

```text
linux64 -> ghcr.io/hard-build/linux64:latest     (--pull=always)
windows64 -> ghcr.io/hard-build/windows64:latest (--pull=always)
```

Explicit forms name immutable images and download them only when missing:

```text
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

The Ubuntu image definition is
`target/linux64/v3.0-ubuntu.22.04.Dockerfile`. It downloads the official
`hard-v3.0.tar.gz` portable release, verifies SHA-256
`4a5d0227e80148684559d148be815cd6169f311fd0abe5b43ad2940b301e9fc1`,
requires its bundled `VERSION` to equal `v3.0`, and installs its complete
`libexec/hard` runtime at `/usr/local/libexec/hard`. The container therefore
uses the backend, `hard.h`, format, clang-format, libclang 18.1.8, Clang
resource headers, and compatibility libraries published from Git tag `v3.0`,
rather than compiling the current branch.

The static Alpine definition is
`target/linux64/v3.0-alpine.3.22-static.Dockerfile`. The portable release is a
glibc runtime and cannot execute on musl, so this image downloads the source
archive for Git tag `v3.0`, verifies SHA-256
`ee24cbeec82087f31a0c07d7a346f85c0f3d5b36fd25199a90ebb69c1e1bee35`, and
builds the backend natively against Alpine's libclang 18. Generated programs
are linked completely statically against musl. The image also builds static
GoogleTest archives from Alpine's packaged sources.

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

The Ubuntu image uses Ubuntu 22.04. Its C++ toolchain is GCC 11 with glibc 2.35.
`-static-libgcc` and `-static-libstdc++` do not make glibc static, so the image
does not promise that generated programs run on systems older than the Ubuntu
22.04 ABI. The runtime also contains GoogleTest 1.11, CMake 3.22, GNU Make,
Meson with Ninja, pkg-config, and the Autoconf, Automake, and Libtool toolchain.
Distribution package revisions are resolved when the image is built. Ubuntu
22.04 standard security maintenance ends in May 2027.

An image version is published only when its previously unseen Dockerfile is
added. The newest Ubuntu version advances `linux64:latest`, and the newest
LLVM-MinGW UCRT version advances `windows64:latest`; adding an Alpine static
image leaves `linux64:latest` unchanged. Modifying, deleting, or re-adding a
known Dockerfile does not rebuild or republish it. CI loads each new image
locally, builds and executes a C++20 smoke program, and only then pushes its
tags. For a `*-static` image, the same pre-publication step rejects an ELF
interpreter or `NEEDED` shared library. For `windows64`, it requires an AMD64
PE file with UCRT contract imports, executes it through `hard run` and Wine,
and runs all declarative integration scenarios inside the image.

The wrapper bind-mounts the host `${HARD_ROOT:-$HOME/.local/share/hard}` at
`/hard` and the current working directory at the same absolute path inside the
container. The v3.0 images use
`/hard/env/linux64:v3.0-ubuntu.22.04/build` and
`/hard/env/linux64:v3.0-alpine.3.22-static/build`. Every image preserves the
shared source snapshots while isolating its build artifacts from other target
versions and `env/host`.

The Windows image uses
`/hard/env/windows64:v4.0-llvm-mingw.20260616-ucrt/build` and keeps its Wine
prefix below the same environment directory.

The container runs with the current numeric UID and GID, preventing root-owned
build outputs, and forwards stdin without allocating a TTY. Only the working
directory and `HARD_ROOT` are mounted. Explicit inputs or resolved symlinks
outside both trees are therefore unavailable in the container. A non-empty
host `HARD_ROOT` selects the bind-mount source but is not copied into the
container environment. Other host `HARD_*` values are not forwarded. The
Ubuntu image fixes its complete target configuration as follows:

```text
HARD_ROOT=/hard
HARD_ENV=linux64:v3.0-ubuntu.22.04
HARD_CC=c++
HARD_CFLAGS=-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto
            -Wall -Wextra
HARD_LDFLAGS=-std=c++20 -O3 -flto=auto -Wall -Wextra
             -static-libgcc -static-libstdc++
HARD_ENTRYPOINTS=main _start
```

The Alpine image uses the same fixed root, compiler, compiler flags, and
entrypoints; only the environment name and linker flags differ:

```text
HARD_ENV=linux64:v3.0-alpine.3.22-static
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

The backend always adds `-I/hard/source` and
`-include /usr/local/libexec/hard/hard.h` internally. These are hard-managed
include mechanics rather than part of the image `HARD_CFLAGS` value.

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

If no path is supplied, `.` is used. Directories are scanned recursively. If
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

`--format` defaults to `format.v1`. Its value is resolved relative to
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
hard build [--no-cache] [-s|--silent] [-o <path>] [path...]
```

The implemented build pipeline currently discovers dependencies, generates one
source-context forward per translation unit, compiles objects, detects
configured entry functions, links reachable objects, and delivers binaries.

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

causes `hard` to download a tar snapshot of that public GitHub repository's
current default branch. The archive contains source files only, not Git history,
and is installed at:

```text
HARD_ROOT/source/github.com/<owner>/<repository>
```

Well-known include prefixes map shorter public paths to canonical repositories.
The current mapping is:

```text
hard/<path>   -> github.com/hard-build/library/<path>
recipe/<path> -> github.com/hard-build/recipe/<path>
```

The repository is installed in the canonical GitHub cache, and `hard` creates
a relative source alias:

```text
HARD_ROOT/source/hard   -> github.com/hard-build/library
HARD_ROOT/source/recipe -> github.com/hard-build/recipe
```

An existing alias must be a symbolic link resolving to the mapped repository;
conflicting files, directories, and links are errors and are never replaced.

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

An existing repository directory is a persistent local cache and is never
updated automatically. Remove:

```text
HARD_ROOT/source/github.com/<owner>/<repository>
```

to request a fresh snapshot on the next build, fetch, run, or test. The
resolver is shared by `build`, `fetch`, `run`, and `test`. Immediately before
each actual request, it reports
`Downloading github.com/<owner>/<repository>` as the command's first progress
step. A missing non-GitHub header retains the original libclang
diagnostic and does not cause a network request.

#### Compiled library recipes

An active included header may describe one compiled static library in a
leading block comment. A `.hard.h` suffix is recommended so the recipe header
does not collide with the library public header, but the suffix is a convention
rather than a parser requirement:

```cpp
/* clang-format off */
/* hard.recipe.v1
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
*/
/* clang-format on */
#pragma once

#include <tinyxml2.h>
```

The marker must occur among the file's leading whitespace and comments before
the first C++ token. The header may contain ordinary includes and arbitrary C++
code after the recipe. Exactly one marker block is allowed. YAML decoding is
strict. The previous `hard.library.v1` spelling is not recognized. Unknown and
duplicate fields, multiple documents, aliases, anchors, merge keys, custom
tags, absolute paths, and paths escaping through `..` are rejected.

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

Installed include directories are appended only to translation units whose
active libclang include graph reaches the recipe header. Installed archives
are appended only when linking a binary whose reachable source closure uses
that recipe. Includes hidden by an inactive preprocessor branch therefore do
not cause a download, package build, compiler flag, or link input.

Packages are stored at:

```text
HARD_ROOT/env/HARD_ENV/library/
└── github.com/<owner>/<repository>/<fingerprint>/
    ├── build/
    ├── install/
    └── manifest.json
```

The fingerprint includes the `hard` executable, recipe header and bytes, full
downloaded source tree, CMake executable, resolved `HARD_CC` executable,
recipe paths, configure arguments, and stable vendor source working directory.
The invocation working directory is not part of this package key. A manifest
verifies the complete installed file tree before reuse. `--no-cache` rebuilds
the package but does not refresh the downloaded GitHub snapshot. Because vendor
builds intentionally do not receive `HARD_CFLAGS`, changing ABI-affecting
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

Each root or automatically discovered translation unit produces one
source-context forward file from the declarations visible in its final
libclang analysis. Only declarations physically originating in active managed
non-system dependencies can enter the output; declarations from the source
file itself and from system headers are excluded. `hard` emits named classes,
structs, and class templates declared directly at global or namespace scope.
It preserves ordinary and inline namespace nesting, removes template defaults,
skips class template specializations, and excludes local, nested, and
anonymous-namespace types.

Downloaded repositories are managed source trees, not opaque system libraries.
Their active headers can contribute declarations to each translation unit
forward file, and same-stem implementation sources are discovered, compiled,
and linked through the same dependency graph. Well-known paths are
canonicalized through their aliases before dependency and object paths are
derived.

Each candidate declaration is validated by reparsing the cumulative generated
forward file with libclang. A candidate that makes it invalid is omitted,
allowing valid declarations from macro-heavy amalgamated headers to remain
usable. Since extraction uses the translation-unit AST, conditional and
macro-dependent declarations remain specific to that source and flag context.

The forward path mirrors the lexical absolute source path below the environment
build directory and appends `.fwd.h` to the complete source name:

```text
first.cpp  -> first.cpp.fwd.h
second.cpp -> second.cpp.fwd.h
```

There is no `value_fwd.h` or other per-header forward output. Every generated
source forward begins with `#pragma once`. When regenerated content is
byte-for-byte unchanged, the existing regular file is retained.

A successful translation-unit analysis is persisted as a versioned
`.hard-parse-cache.json` record beside the mirrored source path. The record
stores its managed dependency list, complete active non-system dependency
snapshot, active library recipe headers, detected entry point, and final
validated source-forward text. Records include a checksum of that semantic
result. After the checksum is validated, a prospective hit restores the
packages and package include flags named by those headers before its action
fingerprint is validated against current inputs.

A parse-cache key includes the `hard` executable digest, libclang version, the
effective compiler flags (configured, hard-managed, and active-package flags),
configured entry-point names when relevant, and the content of the input plus
every active non-system dependency known from the previous successful
analysis, including non-system force-included headers. The invocation working
directory participates only when a compiler argument can depend on it, such as
a relative include, forced-include, toolchain, or response-file path, or an
opaque forwarded driver argument. Sources using only cwd-independent flags can
reuse parsing when selected from a parent directory and then from their own
directory. System headers are represented by the selected `HARD_ENV` rather
than individual content hashes. Missing, malformed, changed, or internally
inconsistent records are misses. A hit skips libclang analysis, restores a
missing generated source forward when needed, and reports
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
  -I<HARD_ROOT>/source \
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

The object path mirrors the absolute source path. The original source
extension is preserved before `.o`, avoiding collisions between sources such
as `file.c` and `file.cpp`:

```text
/home/user/project/src/file.c
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.c.o

/home/user/project/src/file.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.cpp.o
```

Each successful compilation stores an atomic cache record beside its object.
The cache key includes the `hard` executable, compiler path and content,
complete compiler argument vector with the absolute source, source content,
every resolved active non-system include, and the generated source forward.
As with parsing, the invocation working directory is included only for
relative or opaque cwd-dependent compiler arguments. Thus a source using
cwd-independent flags can reuse its object across equivalent selections from
different directories, while `HARD_CFLAGS=-I.` deliberately keeps those
contexts separate. System headers and other toolchain state are represented by
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
below `HARD_ROOT/source`.

#### Entry points and linking

Only global function definitions whose names appear in `HARD_ENTRYPOINTS` make
a translation unit an entry source. The default list is:

```text
main _start
```

The value is parsed as shell-style words. An explicitly empty value disables
entry-point detection. Declarations without a body, class methods, namespace
functions, local functions, and lambdas are not entry points. Defining more
than one configured entry-point name in one source is an error. Detection uses
a full libclang translation unit with the effective compiler flags. Only the
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

The internal binary mirrors the entry source path below the environment build
directory and removes the source extension:

```text
/home/user/project/src/application.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/application
```

When `HARD_EXECUTABLE_SUFFIX=.exe`, inferred internal binary names receive that
suffix instead:

```text
/home/user/project/src/application.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/application.exe
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
[1/?] Downloading github.com/owner/repository
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
- Build does not print a preliminary header list. Preparation progress may show
  `Parsing <source>` while libclang analysis is running.
- `-s` suppresses progress and successful compiler output; compiler, linker,
  and copy errors still go to stderr.

For sources in the canonical GitHub cache, the `Compiling` label is relative
to `HARD_ROOT/source`, for example
`github.com/hard-build/library/application/application.cpp`. A source selected
through a well-known alias is canonicalized to that same label. This affects
only progress output: verbose compiler commands, diagnostics, object paths, and
other artifacts continue to use the actual source path.

Root translation units without a configured entry point remain object files.
Automatically discovered implementation sources are dependency-only even when
they define a configured entry point.

Cache records and artifacts that no longer belong to the selected dependency
graph are not removed automatically.

### `hard run`

```bash
hard run [--no-cache] [-s|--silent] [path...] [-- program-argument...]
```

`run` selects ordinary non-test translation units by the same rules as
`build`, prepares their complete managed dependency closure, compiles the
required objects, links one internal binary, and executes it. It never performs
the `build` delivery step: the binary remains only at its mirrored path below
`HARD_ROOT/env/HARD_ENV/build`, and no extensionless copy is created beside the
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
hard fetch [-s|--silent] [path...]
```

`fetch` downloads the external GitHub dependencies required by the selected C
and C++ translation units without building them. This includes repositories
named by active `hard.recipe.v1` recipe headers. Unlike `build`, its default
recursive selection includes ordinary, `*.test.*`, and legacy `*_test.*`
translation units. Explicit files and directories use the common path-selection
rules.

Dependency analysis uses libclang 18 with the effective compiler flags,
follows active project headers, and recursively discovers same-stem
implementation sources. It then downloads the complete transitive closure of
expanded `github.com/<owner>/<repository>/...` and well-known includes.
`HARD_CC` is not started by `fetch`. The persistent cache and archive-safety
rules are the same as for `build`, `run`, and `test`; existing repository
directories are not refreshed automatically.

`fetch` does not read or write persistent parse-result records, because it
must remain independent of the environment build tree.

When a recipe is active, `fetch` temporarily appends its
`source_include_directories` below the downloaded repository and repeats
dependency analysis. It does not start CMake or `HARD_CC`, install a package,
write a manifest, or create `HARD_ROOT/env`.

This command does not generate forward headers, compile objects, link or copy
binaries, run tests, or create an environment build tree. An empty selection
is a successful no-op. `-j` limits concurrent libclang analyses.

Search, dependency parsing, and every actual request reuse one command
preparation step. Live activity is shown as `[1/?] Searching source files`,
`[1/?] Parsing <source>`, or
`[1/?] Downloading github.com/<owner>/<repository>`; each download label is
emitted immediately before its HTTP request. The exact final total is one.
Normal mode rewrites one line, `-v` writes permanent activity lines, and `-s`
suppresses successful progress. Colors obey `--no-color`. Cached repositories
omit `Downloading`, but search and parsing are still reported.

### `hard test`

```bash
hard test [--list-tests] [--test=<selector>]... \
  [--no-cache] [-s|--silent] [path...]
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

Active `hard.recipe.v1` recipes use the same package build and cache rules as
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

The internal binary follows the same mirrored path rule as a build binary:

```text
/home/user/project/tests/random.test.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/tests/random.test
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
`-I<HARD_ROOT>/source` and `-include <runtime-root>/hard.h`. The runtime root
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
generation rejects environment names that escape `HARD_ROOT/env`.

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
explicit documented version, or `--target=docker://image` executes the
documented `docker run` invocation without resolving the host runtime. It
never builds an image.

The wrapper also owns the fixed completion values for `--target`. It answers
that value position directly for both Cobra completion protocols. Other
completion requests are forwarded to the installed host backend, regardless
of `default-target`, so completion never invokes Docker. The Go backend keeps
only the synthetic target flag declaration needed when generating the Bash,
Zsh, and Fish completion scripts; concrete target values are not compiled
into it.

`PREFIX` defaults to `$HOME/.local`, `BUILD_DIR` defaults to `build`, and
`DESTDIR` can stage an installation without changing its logical prefix.
Because the wrapper derives the prefix at runtime, installations under another
`PREFIX` and staged installations preserve the same relative layout without
rewriting the wrapper. `make install` is host-only: it does not invoke Docker
or install target images or container assets, and it writes `host` to
`default-target`.

The portable archive extends the runtime bundle with the host backend's shared
libraries, LLVM resource headers, `bin/clang-format`, license records, a
version file, and the three completion files shown above. Its top-level
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

Generated artifacts and downloaded source snapshots use the separate
persistent layout:

```text
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
        │   └── <absolute path without the leading slash>/
        │       ├── file.cpp.hard-parse-cache.json
        │       ├── file.cpp.fwd.h
        │       ├── file.cpp.o
        │       ├── file.cpp.o.hard-cache.json
        │       ├── application
        │       ├── application.hard-cache.json
        │       └── application.hard-test-cache.json
        └── library/
            └── github.com/<owner>/<repository>/<fingerprint>/
                ├── build/
                ├── install/
                └── manifest.json
```

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
