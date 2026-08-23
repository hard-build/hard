# hard

`hard` is a convention-based build tool for C and C++ projects. It derives its
work from the source tree instead of requiring project-specific build files,
keeps generated artifacts outside the project, and provides one interface for
formatting, dependency fetching, building, running, and testing.

> [!IMPORTANT]
> The current implementation is the Go module in `hard/`. Root-level files
> provide user documentation, the default format, the environment support
> header, and container assets. Unreferenced stale artifact cleanup remains
> future work.

## Design goals

- Treat source code and include relationships as the build description.
- Require no hand-written project build file.
- Keep generated headers, object files, and cached artifacts outside source
  directories.
- Isolate artifacts produced by different compilers, ABIs, containers, or
  build environments.
- Run independent formatting, dependency analysis, compilation, linking, and
  test-execution work in parallel while keeping readable output.
- Expose only formatting, dependency fetching, building, running, and testing
  as public operations.

## Requirements

Building the Go module in `hard/` requires:

- Linux;
- Go 1.23 or later;
- CGO and a C++20 host toolchain;
- LLVM/Clang 18 development headers at `/usr/lib/llvm-18/include` and the
  `libclang-18` shared library.

Depending on the command, using `hard` also requires:

- a C/C++ compiler supporting the configured flags (`c++` by default) for
  `build`, `run`, and `test`;
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
`hard fetch`, the selected count is used for dependency analysis. In `hard run`,
it limits source preparation and compilation; exactly one binary is linked and
executed. In `hard test`, the selected count is the invocation-wide maximum for
source preparation, compilation, linking, and test-execution phases.

`-v` writes permanent progress entries and command-specific details.
`--no-color` disables ANSI colors. Every command accepts `-s` or `--silent`,
which suppresses normal output while preserving errors.

Persistent flags may appear before or after the command. Command-local flags,
including `-s`, appear after the command and may be interspersed with paths.

The public interface contains exactly these commands:

```text
hard format [--format=<name>] [-s|--silent] [path...]
hard build  [--no-cache] [-s|--silent] [-o <path>] [path...]
hard fetch  [-s|--silent] [path...]
hard run    [--no-cache] [-s|--silent] [path...]
            [-- program-argument...]
hard test   [--list-tests] [--test=<selector>]...
            [--no-cache] [-s|--silent] [path...]
```

If no path is supplied, `.` is used. Directories are scanned recursively. If
paths are supplied, only explicitly named matching files and matching files
below explicitly named directories are selected as roots. During `build`,
`fetch`, `run`, and `test`, implementation sources associated with project
headers may additionally be discovered as dependencies of those roots. `build`,
`run`, and `test` compile those implementations; `fetch` only inspects them.

| Command | Selected files |
| --- | --- |
| `build` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, excluding `*_test.*` |
| `run` | Same as `build` |
| `fetch` | `*.c`, `*.cc`, `*.cpp`, `*.c++`, including `*_test.*` |
| `format` | Build extensions plus `*.h`, `*.hh`, `*.hpp`, `*.h++` |
| `test` | `*_test.c`, `*_test.cc`, `*_test.cpp`, `*_test.c++` |

Extensions and the `_test` suffix are matched without regard to case.
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
hard build [--no-cache] [-s|--silent] [-o <path>] [path...]
```

The implemented build pipeline currently discovers dependencies, generates one
source-context forward per translation unit, compiles objects, detects
configured entry functions, links reachable objects, and delivers binaries.

Every root or automatically discovered translation unit is analyzed through
libclang 18. `hard` passes `HARD_CFLAGS`, the project working directory, and
C++ language mode unless `HARD_CFLAGS` already selects a language. The detailed
preprocessing record supplies one active, preprocessor-aware include graph for
direct, transitive, macro-expanded, conditional, and force-included headers.

libclang classifies system headers, so only resolved non-system dependencies
participate in implementation discovery, source-context forward extraction,
and cache fingerprints. Their canonical absolute paths are deduplicated and
sorted per source. `HARD_ENV` is the immutability boundary for the compiler
toolchain, standard library, libc, sysroot, Clang resource headers, and every
path marked as system by libclang, including user-provided `-isystem`
directories. System headers are excluded from parse and compilation cache
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

to request a fresh snapshot on the next build, fetch, run, or test. The
resolver is shared by `build`, `fetch`, `run`, and `test`. Immediately before
each actual request, it reports
`Downloading github.com/<owner>/<repository>` as the command's first progress
step. A missing non-GitHub header retains the original libclang
diagnostic and does not cause a network request.

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
snapshot, detected entry point, and final validated source-forward text.
Records include a checksum of the stored semantic result.

A parse-cache key includes the `hard` executable digest, libclang version,
working directory, `HARD_CFLAGS`, configured entry-point names when relevant,
and the content of the input plus every active non-system dependency known
from the previous successful analysis, including non-system force-included
headers. System headers are represented by the selected `HARD_ENV` rather than
individual content hashes. Missing, malformed, changed, or internally
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
  -include <source.cpp.fwd.h> \
  -c <source> -o <object>
```

The generated `-include` pair appears after `HARD_CFLAGS` and before `-c`.
Even when no eligible declarations exist, the source forward is present and
contains `#pragma once`, which keeps the compiler command shape uniform.

The canonical target of `HARD_ROOT/env/HARD_ENV/hard.h` is the one managed
header exception. It remains force-included through `HARD_CFLAGS`, but its
declarations are excluded from the generated source forward. Other project or
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

Each successful compilation stores an atomic cache record beside its object.
The cache key includes the `hard` executable, compiler path and content,
working directory, complete compiler argument vector, source content, every
resolved active non-system include, and the generated source forward. System
headers and other toolchain state are represented by `HARD_ENV`. A hit is
accepted only when the object is still a regular file with the recorded
content digest. Missing, changed, malformed, or non-regular artifacts and
records are cache misses.

`--no-cache` disables build, run, and test cache reads for this invocation. It
forces source analysis, source-forward generation, compilation, and linking;
`hard build` also forces binary delivery, while `hard test` reruns its tests.
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

Successful links are cached from the compiler fingerprint, link argument
vector, working directory, and the content of every linked object. The
internal binary digest is verified before a hit is used.

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
and C++ translation units without building them. Unlike `build`, its default
recursive selection includes both ordinary and `*_test.*` translation units.
Explicit files and directories use the common path-selection rules.

Dependency analysis uses libclang 18 with `HARD_CFLAGS`, follows active project
headers, and recursively discovers same-stem implementation sources. It then
downloads the complete transitive closure of expanded
`github.com/<owner>/<repository>/...` and well-known includes. `HARD_CC` is not
started by `fetch`. The persistent cache and archive-safety rules are the same
as for `build`, `run`, and `test`; existing repository directories are not
refreshed automatically.

`fetch` does not read or write persistent parse-result records, because it
must remain independent of the environment build tree.

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
tests/random_test.cpp:
  Random.ReturnsValue
  Random.RejectsInvalidRange

tests/parser_test.cpp:
  Parser.AcceptsValidInput
  Parser.RejectsInvalidInput
```

`--test=<selector>` runs only matching full test names and may be repeated.
A selector without wildcards is exact. `*` matches any number of characters,
including zero, and `?` matches exactly one character:

```bash
hard test --test=Random.ReturnsValue tests/random_test.cpp
hard test --test='Random.*' tests/random_test.cpp
hard test --test='Parser.Test?' tests/parser_test.cpp
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
`HARD_CFLAGS`; its linker flags are appended after `HARD_LDFLAGS`. Failure to
start `pkg-config`, a nonzero result, or malformed output stops the command
before any test is built. An empty selection succeeds without requiring
`pkg-config` or GoogleTest.

For each test source, `hard` uses the build dependency analyzer to recursively
find same-stem, non-test implementation sources required by its non-system
headers. Other `*_test.*` sources are never added automatically. It prepares
one source-context forward for every test and production translation unit,
then compiles them with the combined compiler flags. Dependency closures for
different test roots are prepared concurrently. An object output shared by
several test plans is compiled only once, and every source forward belongs only
to its translation unit. The shared object is then reused by every test that
reaches it. The canonical environment support header remains force-included,
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
preserving object compilation, makes `hard run` fail because it has no entry
target, and has no effect on `hard test`.

`HARD_ENV` is the cache boundary for immutable toolchain state. Use a distinct
value whenever the compiler, libclang resource headers, standard library,
libc, sysroot, ABI, target, container, or a user-provided `-isystem` tree
changes. System headers are not content-hashed; keeping the same `HARD_ENV`
asserts that they remain compatible and unchanged. `--no-cache` can force a
one-off rebuild in the current environment, while a new `HARD_ENV` keeps old
artifacts isolated. Artifact generation rejects environment names that escape
`HARD_ROOT/env`.

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
                ├── file.cpp.hard-parse-cache.json
                ├── file.cpp.fwd.h
                ├── file.cpp.o
                ├── file.cpp.o.hard-cache.json
                ├── application
                ├── application.hard-cache.json
                └── application.hard-test-cache.json
```

An entry source or test source also creates an extensionless internal binary
beside its object, such as `application` beside `application.cpp.o`. Build
binaries are delivered according to `-o` or beside their lexical entry sources
by default. Run and test binaries are not copied out of the build tree.

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
installation failures during build, fetch, run, or test also produce a nonzero
status. `hard run` propagates a normally started program's nonzero exit status;
other command failures return status 1.

Where independent work can continue safely, `hard` processes it and returns an
aggregate failure when the phase completes. Failures to start a required tool
stop new work in that phase.

## License

`hard` is available under the [MIT License](LICENSE).
