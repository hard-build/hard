# hard

`hard` is a convention-based build tool for C and C++ projects. It derives its
work from the source tree instead of requiring project-specific build files,
keeps generated artifacts outside the project, and provides one interface for
formatting, dependency fetching, building, and testing.

> [!IMPORTANT]
> The current implementation is the Go module in `hard/`. Root-level files
> provide user documentation, the default format, the environment support
> header, and container assets. Incremental cache validation and stale artifact
> cleanup remain future work.

## Design goals

- Treat source code and include relationships as the build description.
- Require no hand-written project build file.
- Keep generated headers, object files, and cached artifacts outside source
  directories.
- Isolate artifacts produced by different compilers, ABIs, containers, or
  build environments.
- Run independent formatting, dependency analysis, compilation, linking, and
  test-execution work in parallel while keeping readable output.
- Expose only formatting, dependency fetching, building, and testing as public
  operations.

## Requirements

Building the Go module in `hard/` requires:

- Linux;
- Go 1.23 or later;
- CGO and a C++20 host toolchain;
- LLVM/Clang 18 development headers at `/usr/lib/llvm-18/include` and the
  `libclang-18` shared library.

Depending on the command, using `hard` also requires:

- a C/C++ compiler supporting the configured flags (`c++` by default) for
  `build` and `test`;
- `clang-format` for source formatting;
- `pkg-config` and GoogleTest's `gtest_main` package for test compilation and
  linking;
- network access to GitHub when a referenced `github.com/<owner>/<repository>/`
  or well-known repository snapshot is not already cached below
  `HARD_ROOT/source`.

## Command-line interface

```text
hard [-v|--verbose] [--no-color] [-jN|--jobs=N] <command> [path...]
```

The default job count is one. `-jN` or `--jobs=N` selects `N` workers. Bare
`-j`, bare `--jobs`, `-j0`, and `--jobs=0` use all logical CPUs. Negative job
counts are rejected. A command never creates a nested `N × N` pool. In
`hard fetch`, the selected count is used for dependency analysis. In `hard test`,
the selected count is the invocation-wide maximum for each dependency,
forward-generation, compilation, linking, and test-execution phase.

`-v` writes permanent progress entries and command-specific details.
`--no-color` disables ANSI colors. Every command accepts `-s` or `--silent`,
which suppresses normal output while preserving errors.

Flags may appear before the command, after it, or between positional paths.

The public interface contains exactly these commands:

```text
hard format [--format=<name>] [-s|--silent] [path...]
hard build  [-s|--silent] [-o <path>] [path...]
hard fetch  [-s|--silent] [path...]
hard test   [-s|--silent] [path...]
```

If no path is supplied, `.` is used. Directories are scanned recursively. If
paths are supplied, only explicitly named matching files and matching files
below explicitly named directories are selected as roots. During `build`,
`fetch`, and `test`, implementation sources associated with project headers
may additionally be discovered as dependencies of those roots. `build` and
`test` compile those implementations; `fetch` only inspects them.

| Command | Selected files |
| --- | --- |
| `build` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, excluding `*_test.*` |
| `fetch` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, including `*_test.*` |
| `format` | Build extensions plus `*.h`, `*.hh`, `*.hpp`, `*.h++` |
| `test` | `*_test.c`, `*_test.cc`, `*_test.cpp`, `*_test.c++` |

Extensions and the `_test` suffix are matched without regard to case.
Unsupported explicitly named files are ignored. Missing or inaccessible paths
are errors; finding no matching files is a successful no-op.

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
clang-format --style=file:<HARD_ROOT/format/<name>> -i <file>
```

`--format` defaults to `format.v1`. Its value is resolved relative to
`HARD_ROOT/format`. Empty values, absolute paths, lexical escapes through `..`,
non-regular files, and symlinks resolving outside the real format directory
are rejected. An internal symlink to a regular style file is allowed.

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
hard build [-s|--silent] [-o <path>] [path...]
```

The implemented build pipeline currently discovers dependencies, generates
forward-declaration headers, compiles object files, detects configured entry
functions, links their reachable objects, and delivers executable binaries.

Every root or automatically discovered translation unit is analyzed through
libclang 18. `hard` passes `HARD_CFLAGS`, the project working directory, and
C++ language mode unless `HARD_CFLAGS` already selects a language. The detailed
preprocessing record supplies one active, preprocessor-aware include graph for
direct, transitive, macro-expanded, conditional, and force-included headers.

libclang classifies system headers, so only resolved non-system dependencies
participate in implementation discovery, forward generation, and forced
inclusion. Their canonical absolute paths are deduplicated and sorted per source. No
preliminary header list is printed, although preparation progress identifies
the source or header currently being parsed. The same analysis reports
unresolved include spellings. A missing include whose expanded path begins with:

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
hard/<path> -> github.com/hard-build/library/<path>
```

The repository is installed in the canonical GitHub cache, and `hard` creates
a relative source alias:

```text
HARD_ROOT/source/hard -> github.com/hard-build/library
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

to request a fresh snapshot on the next build, fetch, or test. The resolver is
shared by `build`, `fetch`, and `test`. Immediately before each actual request,
it reports `Downloading github.com/<owner>/<repository>` as the command's first
progress step. A missing non-GitHub header retains the original libclang
diagnostic and does not cause a network request.

Each discovered header is parsed independently through the same libclang
bridge and `HARD_CFLAGS`. The preprocessor is active, but declarations are
filtered by physical source file, so included headers cannot contribute to the
current header's forward file. `hard` emits only named classes, structs, and
class templates declared directly at global or namespace scope. It preserves
ordinary and inline namespace nesting, removes template defaults, skips class
template specializations, and excludes local, nested, and anonymous-namespace
types.

Downloaded repositories are managed source trees, not opaque system libraries.
Their discovered headers receive the same forward-generation and force-include
treatment as project headers. Same-stem implementation sources are discovered,
compiled, and linked through the same dependency graph. Well-known paths are
canonicalized through their aliases before artifact paths and forward headers
are derived.

Each candidate declaration is validated by reparsing the generated forward
header with libclang. A candidate that makes the standalone forward header
invalid is omitted, allowing valid declarations from macro-heavy amalgamated
headers to remain usable. Syntax errors reported in the source header itself
still fail generation.

Forward headers mirror canonical absolute header paths below the environment
build directory and preserve the original extension:

```text
file.h   -> file_fwd.h
file.hh  -> file_fwd.hh
file.hpp -> file_fwd.hpp
file.h++ -> file_fwd.h++
```

After dependency and forward generation succeed, every root source and
automatically discovered implementation source is compiled in parallel. Its
own direct, transitive, and force-included non-system dependencies are
mapped to their generated forward headers and force-included in sorted
dependency order:

```text
HARD_CC <HARD_CFLAGS...> \
  -include <first-dependency-forward-header> \
  -include <second-dependency-forward-header> \
  ... \
  -c <source> -o <object>
```

The repeated `-include` pairs appear after `HARD_CFLAGS` and before `-c`.
Shared dependencies are included for every source that needs them, while
forward headers belonging only to unrelated sources are not added. A source
with no discovered non-system dependencies receives no additional `-include`.

The canonical target of `HARD_ROOT/env/HARD_ENV/hard.h` is the one managed
header exception. It remains force-included through `HARD_CFLAGS`, but `hard`
does not parse it as a standalone forward target, generate a corresponding
`hard_fwd.h`, or append another `-include` for that forward. Other project or
external headers named `hard.h` are treated normally.

The object path mirrors the absolute source path. The original source
extension is preserved before `.o`, avoiding collisions between sources such
as `file.c` and `file.cpp`:

```text
/home/user/project/src/file.c
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.c.o

/home/user/project/src/file.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/src/file.cpp.o
```

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
a full libclang translation unit with `HARD_CFLAGS`. Only the active
preprocessor branch participates, and a function definition produced by a
macro is detected.

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
HARD_CC <entry-and-dependency-objects...> <HARD_LDFLAGS...> \
  -o <internal-binary>
```

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

Output modes:

Search, source analysis, header analysis, and repository downloads share the
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

Each download update is emitted immediately before its HTTP request. Cached
repositories omit `Downloading`, but search and parsing still occupy step one.
After dependency and forward preparation, compilation always continues at
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
  `Parsing <source-or-header>` while libclang analysis is running.
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

Incremental cache validation and stale artifact cleanup are not implemented;
root and automatically discovered sources are rebuilt on every invocation.

### `hard fetch`

```bash
hard fetch [-s|--silent] [path...]
```

`fetch` downloads the external GitHub dependencies required by the selected C
and C++ translation units without building them. Unlike `build`, its default
recursive selection includes both ordinary and `*_test.*` translation units.
Explicit files and directories use the common path-selection rules.

Dependency analysis uses libclang 18 with `HARD_CFLAGS`, follows active project
headers, and recursively discovers same-stem implementation sources. It then
downloads the complete transitive closure of expanded
`github.com/<owner>/<repository>/...` and well-known includes. `HARD_CC` is not
started by `fetch`. The persistent cache and archive-safety rules are the same
as for `build` and `test`; existing repository directories are not refreshed
automatically.

This command does not generate forward headers, compile objects, link or copy
binaries, run tests, or create an environment build tree. An empty selection
is a successful no-op. `-j` limits concurrent libclang analyses.

Search, dependency parsing, and every actual request reuse one command
preparation step. Live activity is shown as `[1/?] Searching source files`,
`[1/?] Parsing <source-or-header>`, or
`[1/?] Downloading github.com/<owner>/<repository>`; each download label is
emitted immediately before its HTTP request. The exact final total is one.
Normal mode rewrites one line, `-v` writes permanent activity lines, and `-s`
suppresses successful progress. Colors obey `--no-color`. Cached repositories
omit `Downloading`, but search and parsing are still reported.

### `hard test`

```bash
hard test [-s|--silent] [path...]
```

Every selected test source is built and run as a separate executable. Before
processing a non-empty selection, `hard` obtains GoogleTest flags with:

```text
pkg-config --cflags gtest_main
pkg-config --libs gtest_main
```

The outputs are parsed as shell-style argument vectors with environment and
command substitution disabled. GoogleTest compiler flags are appended after
`HARD_CFLAGS`; its linker flags are appended after `HARD_LDFLAGS`. Failure to
start `pkg-config`, a nonzero result, or malformed output stops the command
before any test is built. An empty selection succeeds without requiring
`pkg-config` or GoogleTest.

For each test source, `hard` uses the build dependency analyzer to recursively
find same-stem, non-test implementation sources required by its non-system
headers. Other `*_test.*` sources are never added automatically. It generates
the required forward headers and compiles the test plus its production
dependencies with the combined compiler flags. Dependency closures for
different test roots are prepared concurrently. A header or object output
shared by several test plans is generated or compiled only once. The shared
object is then reused by every test that reaches it. The canonical environment
support header remains force-included but receives no generated forward, as in
`build`. `HARD_ENTRYPOINTS` is ignored for this command because `gtest_main`
supplies the test executable entry function.

The same GitHub snapshot resolver is shared by every test plan, so missing
expanded `github.com/` and well-known includes use the build download and cache
rules described above. Downloaded repositories are handled as managed source
trees: their forward headers and same-stem implementations participate in the
test build, and their `Compiling` labels use canonical `github.com/...` paths.
Search, dependency parsing, forward parsing, and all live download entries
share preparation step `[1/?]` for the test invocation. Downloads do not add a
separate step. After preparation, the exact total includes that first step and
compilation continues at `[2/M]`, whether dependencies were downloaded or
already cached.

The test source object and all reachable production objects are linked with
ordinary compiler-driver linking:

```text
HARD_CC <test-and-dependency-objects...> \
  <HARD_LDFLAGS...> <gtest_main-linker-flags...> \
  -o <internal-test-binary>
```

The internal binary follows the same mirrored path rule as a build binary:

```text
/home/user/project/tests/random_test.cpp
  -> HARD_ROOT/env/HARD_ENV/build/home/user/project/tests/random_test
```

Test binaries remain in the environment build tree and are not copied into
the project. The `test` command does not accept `-o`.

The work is divided into invocation-wide phases:

1. prepare the dependency closure of each selected test;
2. generate each unique forward header;
3. compile each unique object;
4. link every test whose required objects compiled successfully;
5. run every successfully linked test.

Each phase uses at most the selected `-j` worker count without multiplying
that limit through nested pools. Link jobs and test executables from different
test files therefore run concurrently. A preparation, compilation, or link
failure skips only test plans that require the failed work; independent tests
continue.
A nonzero test result is recorded while other tests continue. The command
returns nonzero after all safe independent work has been attempted if any
preparation, compilation, link, progress-output, process-start, or
test-execution step failed.

All successfully prepared test plans share one progress counter. Its total is
one preparation step plus the number of unique compiled sources and one link
step and one test step for every test executable. A shared production source
contributes one compilation step even when several tests use it. Four
header-only tests use one continuous counter:

```text
[1/?] Searching source files
[1/?] Parsing first_test.cpp
...
[2/13] Compiling first_test.cpp
...
[6/13] Linking first_test
...
[10/13] Testing first_test
...
[13/13] Testing fourth_test
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
| `HARD_ROOT` | Installation and artifact root | `~/.local/share/hard` |
| `HARD_ENV` | Isolated build-environment name | `host` |
| `HARD_CC` | Compiler executable | `c++` |
| `HARD_CFLAGS` | libclang analysis and object compiler flags | See below |
| `HARD_LDFLAGS` | Executable linker flags | See below |
| `HARD_ENTRYPOINTS` | Global entry-function names | `main _start` |

Unset or empty `HARD_ROOT`, `HARD_ENV`, and `HARD_CC` use their defaults.

Default compiler flags:

```text
-std=c++20
-O3
-flto=auto
-Wall
-Wextra
-I<HARD_ROOT>/source
-include
<HARD_ROOT>/env/<HARD_ENV>/hard.h
```

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
explicitly empty value means no flags in that category.
Consequently, an explicit `HARD_CFLAGS` must include
`-I<HARD_ROOT>/source` if downloaded GitHub or well-known headers should remain
reachable.

`HARD_ENTRYPOINTS` follows the same shell-style parsing and disabled expansion
rules. Unlike `HARD_ROOT`, `HARD_ENV`, and `HARD_CC`, an explicitly empty value
does not select the default: it disables `hard build` binary linking while
preserving object compilation. It has no effect on `hard test`.

Use a distinct `HARD_ENV` for every incompatible compiler, standard library,
ABI, target, or container. Artifact generation rejects environment names that
escape `HARD_ROOT/env`.

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
hard test tests
```

## Build, installation, and artifact layout

From the repository root, `make` builds the Go backend as `build/hard`.
`make install` uses this user-local layout by default:

```text
~/.local/
├── bin/hard
├── libexec/hard/hard
└── share/hard/
    ├── env/host/hard.h
    └── format/format.v1
```

The public `bin/hard` command is a POSIX shell wrapper. It replaces itself with
the backend in `libexec/hard/hard` and forwards every argument unchanged.

`PREFIX` defaults to `$HOME/.local`, `BUILD_DIR` defaults to `build`, and
`DESTDIR` can stage an installation without changing its logical prefix. When
installing under a different `PREFIX`, set `HARD_ROOT` to
`<PREFIX>/share/hard` at runtime because the wrapper does not modify the
environment.

The environment support header and generated artifacts use this layout:

```text
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
            └── <absolute path without the leading slash>/
                ├── file.cpp.o
                ├── file_fwd.h
                └── file_fwd.hpp
```

An entry source or test source also creates an extensionless internal binary
beside its object, such as `application` beside `application.cpp.o`. Build
binaries are delivered according to `-o` or beside their lexical entry sources
by default; test binaries are not copied out of the build tree.

`make install` supplies the root `hard.h` as `env/host/hard.h`. The `hard`
command does not generate environment support headers. Other environments must
supply `env/<HARD_ENV>/hard.h` when the default `HARD_CFLAGS` are used. The
build tree can therefore hold artifacts for multiple projects without placing
intermediate files in those projects.

Downloaded repository snapshots are shared by all `HARD_ENV` values below one
`HARD_ROOT` and remain in place until removed explicitly.

## Exit status

Commands return zero when all requested work succeeds. Invalid paths, invalid
configuration, missing tools or support files, parsing errors, formatter
failures, compiler failures, linker failures, and failed test executables
produce a nonzero status. GitHub request, archive-validation, extraction, and
installation failures during build, fetch, or test also produce a nonzero
status.

Where independent work can continue safely, `hard` processes it and returns an
aggregate failure when the phase completes. Failures to start a required tool
stop new work in that phase.

## License

`hard` is available under the [MIT License](LICENSE).
