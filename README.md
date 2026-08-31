<p align="center">
  <img src="assets/hard-build.svg" alt="Hard Build" width="760">
</p>

# Hard Build

Build C and C++ programs directly from their source tree.

`hard` treats selected source files and their active `#include` relationships
as the build description. Point it at a file or directory: it finds the
required implementation sources, prepares reachable dependencies, compiles
what changed, and links the entry programs. Your project does not need a
hand-written `CMakeLists.txt` or another project build file.

## Quick Start

On Linux x86-64, install the latest portable release:

```bash
curl -fsSL https://raw.githubusercontent.com/hard-build/hard/main/install.sh | sh
```

The installer places `hard` below `~/.local`, configures the current shell, and
adds completion for Bash, Zsh, and Fish. It does not use `sudo` or install
system packages. Open a new shell when it finishes.

Create `example.cpp`:

```cpp
#include <iostream>

int main()
{
    std::cout << "Hello, hard!\n";
}
```

Build it, then run the produced binary:

```bash
hard build example.cpp
./example
```

The expected output is:

```text
Hello, hard!
```

The default `host` target uses your C++20 compiler. To build and run in the
latest maintained Linux environment instead, use Docker:

```bash
hard --target=linux64 run example.cpp
```

## What hard can do

The public interface contains seven commands:

| Goal | Command | Result |
| --- | --- | --- |
| Check the hard version | `hard version` | Prints the version embedded in the running backend |
| Inspect the toolchain | `hard environment` | Shows the operating system, compiler, target, flags, libc, and libclang |
| Format sources | `hard format` | Formats selected C and C++ sources and headers in place |
| Fetch dependencies | `hard fetch` | Resolves includes and downloads missing public source snapshots without compiling |
| Build programs | `hard build` | Compiles selected and discovered sources and delivers linked executables |
| Build and run | `hard run` | Builds one entry program and executes it with live input and output |
| Run tests | `hard test` | Discovers, builds, filters, and runs GoogleTest executables |

Across those commands, `hard` also provides:

- include-driven implementation discovery, such as `object.h` → `object.cpp`,
  including transitive relationships and cycles;
- public GitHub source dependencies and reusable `recipe/*.hard.h` definitions
  for compiled third-party libraries;
- persistent content caches for analysis, compilation, linking, delivery, and
  successful test results;
- host builds, maintained Linux container environments, Windows x86-64
  cross-compilation, and arbitrary compatible `docker://` images;
- parallel work with `-j` and detailed compiler and linker commands with `-v`;
- downloaded sources, intermediate artifacts, and caches outside the project
  tree, leaving only delivered build executables beside project sources or in
  the requested output directory.

For example, the same source can be built for different environments:

```bash
hard --target=host build example.cpp
hard --target=linux64 build example.cpp
hard --target=windows64 build example.cpp  # produces example.exe
```

Continue with the guided
[example](https://github.com/hard-build/example/tree/main/001.helloworld), or
jump to the [complete reference](docs/reference.md) when you need exact command
and cache behavior.

## How It Works

`hard` derives work from the selected sources and their active includes:

- an included project header can discover a same-directory implementation with
  the same filename stem, such as `object.h` → `object.cpp`;
- implementation discovery continues transitively and handles include cycles;
- an unresolved `github.com/<owner>/<repository>/...` include downloads a
  source snapshot of that public repository;
- an included `recipe/*.hard.h` header can build and statically link a compiled
  third-party library;
- entry sources define `main` or another name configured by
  `HARD_ENTRYPOINTS`;
- test sources use the preferred `*.test.cpp` form; legacy `*_test.cpp` names
  remain supported.

If no path is supplied, commands scan the current directory recursively.
Explicit files and directories limit the root selection, while required
implementation sources may still be discovered through includes.

| Command | Root files |
| --- | --- |
| `build` and `run` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, excluding tests |
| `fetch` | The same extensions, including test sources |
| `format` | All source extensions plus `*.h`, `*.hh`, `*.hpp`, `*.h++` |
| `test` | `*.test.c`, `*.test.cc`, `*.test.cpp`, `*.test.c++` and legacy `*_test.*` |

Extensions and test suffixes are matched case-insensitively.

## Commands

```text
hard [--target=<name>] [-v|--verbose] [--no-color]
     [-jN|--jobs=N] <command> [path...]
```

`-jN` selects an exact worker count. Bare `-j`, bare `--jobs`, and zero use
all logical CPUs. The default is one worker. `-v` prints permanent progress and
the compiler, linker, or child commands that actually run. `--no-color`
disables ANSI colors. Commands that scan sources accept `-s` or `--silent`.

The public interface contains exactly seven commands:

```text
hard version
hard environment
hard format [--format=<name>] [-s|--silent] [path...]
hard build  [--no-cache] [-s|--silent] [-o <path>] [path...]
hard fetch  [-s|--silent] [path...]
hard run    [--no-cache] [-s|--silent] [path...]
            [-- program-argument...]
hard test   [--list-tests] [--test=<selector>]...
            [--no-cache] [-s|--silent] [path...]
```

### `hard version`

Prints the version embedded in the running backend:

```bash
hard version
```

A development build reports `v4.0-development`. A release build reports
`v4.0`; release packaging removes the prerelease component from the binary and
checks the result against the release tag. The command does not read runtime
files, load `HARD_*` configuration, inspect the toolchain, or scan sources.

### `hard environment`

Prints the effective build environment without scanning sources or creating
artifacts:

```bash
hard environment
hard --target=linux64 environment
```

The report includes the embedded hard version, runtime and cache roots,
operating system, kernel,
CPU and libc, resolved compiler and target, executable naming and runner
settings, configured compiler flags, linker flags and entry points, and the
libclang version and resource directory. The CFLAGS list omits the source-root
include and runtime `hard.h` force include that the backend manages internally.
A diagnostic that is not available on a particular system is shown as
`unavailable` without hiding the rest of the report. The title is enclosed
between horizontal rules. Sections and labels are colored by default, fields
are aligned, and flags and entry points use one shell-quoted argument per line.
Use `--no-color` to retain the same layout without ANSI styling.

### `hard format`

Formats selected sources in place with the bundled `clang-format` and
`format.v1` style:

```bash
hard format src tests
hard format --format=format.v1 .
```

Verbose mode prints a unified diff for every changed file. An empty selection
is a successful no-op.

### `hard build`

Analyzes includes, prepares reachable third-party libraries, compiles selected
and discovered translation units, and links every selected entry source:

```bash
hard build -j src
hard build -o output/application src/application.cpp
hard build -o output/ src
```

Without `-o`, a linked binary is delivered beside its entry source with the
source extension removed. An exact `-o` file requires one entry source. A
directory output preserves relative source paths and supports multiple entry
sources.

Successful analysis, compilation, linking, and delivery are reused from the
content cache. `--no-cache` forces the build work for the current invocation
without deleting downloaded repository snapshots.

### `hard fetch`

Downloads the complete external source closure needed by selected ordinary and
test translation units:

```bash
hard fetch -j src tests
```

`fetch` performs dependency analysis but does not compile, link, run CMake,
create environment artifacts, or execute tests. Existing snapshots are reused.

### `hard run`

Builds exactly one selected entry program and executes its internal binary
without copying it beside the source:

```bash
hard run src/application.cpp
hard run src/application.cpp -- --mode=check "input file.txt"
```

Arguments after `--` are passed to the program unchanged. The program inherits
the current working directory and live standard streams. Build stages may be
cached, but the program itself runs on every successful invocation. Its normal
nonzero exit status is propagated.

### `hard test`

Builds each selected test source as a separate GoogleTest executable and runs
it:

```bash
hard test tests
hard test --list-tests tests
hard test --test=Random.ReturnsValue tests
hard test --test='Parser.*' tests
```

`--list-tests` prints normalized full test names without executing them.
`--test` may be repeated; `*` matches any number of characters and `?` matches
one. Quote wildcard selectors so the shell does not expand them. Every selector
must match at least one discovered test.

Successful test results are cached. Use `hard test --no-cache` when undeclared
runtime inputs such as files, services, network responses, or time matter.

## Dependencies and Recipes

An include below the public GitHub namespace downloads the repository's current
default-branch snapshot when it is not already cached:

```cpp
#include <github.com/nlohmann/json/single_include/nlohmann/json.hpp>
```

Well-known namespaces provide shorter paths:

```text
hard/<path>   -> github.com/hard-build/library/<path>
recipe/<path> -> github.com/hard-build/recipe/<path>
```

For example, this include obtains a recipe, builds TinyXML2 with CMake, and
links its installed static library:

```cpp
#include <recipe/tinyxml2.hard.h>
```

A project can also keep its own `*.hard.h` recipe beside its sources. The full
`hard.recipe.v1` YAML schema, validation rules, package layout, and cache
behavior are documented in the
[compiled-library recipe reference](docs/reference.md#compiled-library-recipes).

Downloaded repository directories are persistent snapshots and are not
refreshed automatically.

## Targets

The wrapper accepts `host`, the latest `linux64` or `windows64` image, any
syntactically valid explicit tag for either known image repository, or an
arbitrary image prefixed with `docker://`. Both target-option forms are
accepted anywhere before `--`:

```bash
hard --target=host build src
hard --target=linux64 build src
hard test --target linux64:v4.0-glibc.2.35 tests
hard --target=windows64 build src
hard --target=docker://registry.example/toolchain:tag environment
```

The portable installer leaves the target default absent, which selects `host`.
`make install` records `host` explicitly. An explicit target always overrides
the installed default.

The `host` target executes the backend from the same installation prefix and
uses the host compiler and dependencies. Unversioned container targets always
check for and run their latest images:

```text
linux64 -> ghcr.io/hard-build/linux64:latest
windows64 -> ghcr.io/hard-build/windows64:latest
```

An explicit `linux64:<tag>` or `windows64:<tag>` target is downloaded only
when missing. The wrapper validates only the Docker tag syntax and does not
interpret its version, libc, distribution, or toolchain components. Documented
version tags are immutable:

```text
linux64:v4.0-glibc.2.35
  -> ghcr.io/hard-build/linux64:v4.0-glibc.2.35
linux64:v4.0-musl.1.2.5-static
  -> ghcr.io/hard-build/linux64:v4.0-musl.1.2.5-static
linux64:v3.0-ubuntu.22.04
  -> ghcr.io/hard-build/linux64:v3.0-ubuntu.22.04
linux64:v3.0-alpine.3.22-static
  -> ghcr.io/hard-build/linux64:v3.0-alpine.3.22-static
windows64:v4.0-llvm-mingw.20260616-ucrt
  -> ghcr.io/hard-build/windows64:v4.0-llvm-mingw.20260616-ucrt
```

For `docker://registry.example/toolchain:tag`, the wrapper strips only the
`docker://` prefix and passes `registry.example/toolchain:tag` to Docker with
`--pull=missing`. The image must provide a compatible entrypoint and its own
`HARD_*` configuration while using `/hard` for persistent state and accepting
the project at the mounted working-directory path. Empty image names and names
beginning with `-` are rejected.

The current glibc image builds `hard` v4.0 for an Ubuntu 22.04 environment with
glibc 2.35. Both its versioned tag and `latest` use
`HARD_ENV=linux64:v4.0-glibc.2.35`, so they share compatible artifacts. The
current musl image builds `hard` v4.0 natively on Alpine 3.22 with musl 1.2.5,
uses `HARD_ENV=linux64:v4.0-musl.1.2.5-static`, and links generated executables
fully statically. The older Ubuntu- and Alpine-named tags remain available.

The Windows image contains `hard` v4.0, LLVM-MinGW 20260616 with Clang 22.1.8
and UCRT, and Wine. It sets the generic executable suffix to `.exe` and runner
to `wine`; `hard` itself does not infer behavior from the `windows64` name or
from `HARD_ENV`. The bundled C++ runtime is linked statically, but Windows
system and UCRT DLL imports remain, so the result is not a fully static
executable.

The wrapper mounts `HARD_ROOT` at `/hard` and the current working directory at
the same absolute container path. Source snapshots and caches therefore persist
across disposable containers, while host and container artifacts remain
separated by `HARD_ENV`.

Only the working directory and `HARD_ROOT` are mounted. Inputs and resolved
symlinks outside both trees are unavailable in the container. Host `HARD_*`
values other than the mount source are not forwarded; the image owns its
toolchain configuration.

Container images are `linux/amd64`; generated programs require an x86-64-v3
CPU. Docker must be installed and running separately.

## Requirements

The portable release requires Linux x86-64 and glibc 2.27 or newer. It bundles
the backend, libclang 18.1.8, Clang resource headers, `clang-format`, and the
required `libtinfo` compatibility library.

Host execution additionally needs tools required by the selected command:

- a C++20 compiler for `build`, `run`, and `test`;
- `pkg-config` and GoogleTest's `gtest_main` package for `test`;
- CMake when a reachable compiled-library recipe uses it;
- network access when a required GitHub snapshot is not cached.

The installer does not install these tools, Docker, or system packages. The
container images contain their own compiler, GoogleTest, CMake, GNU Make,
Meson/Ninja, pkg-config, Autoconf, Automake, and Libtool toolchain. The
`windows64` image additionally contains Wine.

Ubuntu 18.04's default GCC 7 does not satisfy the default C++20 build contract.
Use `linux64` there or configure a suitable host toolchain.

## Configuration

`hard` reads configuration from environment variables:

| Variable | Purpose | Default |
| --- | --- | --- |
| `HARD_ROOT` | Persistent source and artifact root | `~/.local/share/hard` |
| `HARD_ENV` | Toolchain and cache environment name | `host` |
| `HARD_CC` | Compiler executable | `c++` |
| `HARD_CFLAGS` | Project compiler and libclang flags | `-std=c++20 -O3 -flto=auto -Wall -Wextra` |
| `HARD_LDFLAGS` | Linker flags | Compiler defaults plus `-static-libgcc -static-libstdc++` |
| `HARD_ENTRYPOINTS` | Global entry-function names | `main _start` |
| `HARD_EXECUTABLE_SUFFIX` | Suffix for inferred executable paths | Empty |
| `HARD_EXECUTABLE_RUNNER` | Program used to execute built binaries | Empty (direct execution) |

An explicit `HARD_CFLAGS` or `HARD_LDFLAGS` value replaces the complete
default vector. The backend always adds its source-root include and runtime
`hard.h` force include. An explicitly empty `HARD_ENTRYPOINTS` disables build
entry linking.

`HARD_EXECUTABLE_SUFFIX` must be empty or start with `.` and cannot contain a
path separator. It is appended to internal binaries, default delivery paths,
and names inferred below a directory `-o`; an exact file supplied with `-o`
remains exact. `HARD_EXECUTABLE_RUNNER` is one executable name or path. When it
is non-empty, `hard run`, test listing, and test execution invoke
`<runner> <binary> ...`; otherwise binaries execute directly.

When a portable runtime contains exactly one `lib/clang/<version>/include`
directory, the backend adds it to libclang analysis as an after-system include.
This parser-only argument is not part of `HARD_CFLAGS` and does not appear in
compiler commands.

`HARD_ENV` is the cache boundary for the compiler, standard library, libc,
sysroot, ABI, container, and other immutable toolchain state. Select a
different environment name after changing that state.

Example debug configuration:

```bash
export HARD_ENV=clang-debug
export HARD_CC=clang++
export HARD_CFLAGS='-std=c++20 -O0 -g -Wall -Wextra'
export HARD_LDFLAGS='-std=c++20 -O0 -g'
```

Container targets use their image-owned configuration rather than these host
values. The host-side `HARD_ROOT` still selects the directory mounted at
`/hard`.

## Installed Files and Caches

The portable installer uses:

```text
~/.local/
├── bin/hard
├── libexec/hard/          wrapper runtime, backend, headers, and tools
└── share/
    ├── bash-completion/completions/hard
    ├── zsh/site-functions/_hard
    ├── fish/vendor_completions.d/hard.fish
    └── hard/              downloaded sources and persistent caches
```

Persistent state below `HARD_ROOT` is separated into shared sources and
environment-specific artifacts:

```text
HARD_ROOT/
├── source/
│   ├── github.com/
│   ├── hard -> github.com/hard-build/library
│   └── recipe -> github.com/hard-build/recipe
└── env/
    ├── host/
    ├── linux64:v4.0-glibc.2.35/
    ├── linux64:v4.0-musl.1.2.5-static/
    └── windows64:v4.0-llvm-mingw.20260616-ucrt/
```

Generated forwards, objects, internal binaries, package installations, and
cache records remain below the selected environment. Build binaries are copied
according to `-o` or beside their entry sources; run and test binaries remain
internal. Executable suffixes and runners come from the corresponding generic
configuration variables, not from `HARD_ENV`.

Stale generated artifacts and downloaded snapshots are not removed
automatically.

## Building Hard from Source

Building the Go backend requires:

- Linux;
- Go 1.23 or later;
- CGO and a C++20 host toolchain;
- LLVM/Clang 18 headers at `/usr/lib/llvm-18/include`;
- the `libclang-18` shared library.

From the repository root:

```bash
make
make check
make unittest
make install
```

`make` writes the development backend, which reports `v4.0-development`, to
`build/hard`. `make check` verifies formatting,
runs the ordinary and race test suites, vet, an isolated build, module and
shell-script checks, and staged and unstaged Git diffs. `make unittest` runs
the declarative C and C++ integration scenarios with the existing `hard`
command from `PATH`; it does not build or install `hard`. See the
[integration fixture guide](unittest/README.md) for its requirements and
configuration variables. `make install` defaults to `PREFIX=$HOME/.local`,
installs the wrapper and host runtime, creates the persistent data root, and
records `host` as the target. `DESTDIR` can stage an installation without
changing its logical prefix.

## Full Reference

The [complete command and behavior reference](docs/reference.md) documents
validation rules, exact cache behavior, output and progress semantics, recipe
schema details, target configuration, artifact paths, and edge cases.

## License

`hard` is available under the [MIT License](LICENSE).
