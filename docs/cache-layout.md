# Persistent cache layout

This is the backend's persistent layout for `HARD_ROOT`. It applies with or
without project dependency recording. The examples use fictional project paths
`/workspace/example` and `/workspace/example2`, and `HARD_ENV=host`.

`example` includes headers from `github.com/hard-build/library` through
`#include <hard/...>`. `example2` includes a local `tinyxml2.hard.h` recipe that
builds TinyXML2 with CMake. Local sources, recipes and optional `hard.yaml`
files remain in those projects, outside `HARD_ROOT`.

## Tree

Paths containing several slash-separated components are abbreviated directory
chains. Symlink targets beginning with `...` are schematic; actual links are
relative and select a concrete commit. Directories and records are created
only when the corresponding work is needed.

```text
HARD_ROOT/
|
|-- snapshot/                                  # Sources shared by projects and environments
|   `-- github.com/
|       |-- hard-build/library/                # Actual repository source
|       |   |-- @default                       # Regular text file: default full commit ID
|       |   |-- @<hard-commit>/                # Source tree at this exact commit
|       |   |   `-- ...
|       |   `-- @<hard-commit>.checksum         # Normalized source-tree checksum
|       |
|       `-- leethomason/tinyxml2/               # Actual TinyXML2 source
|           |-- @default                       # Regular text file: default full commit ID
|           |-- @<tinyxml2-commit>/            # Source tree at this exact commit
|           |   |-- CMakeLists.txt
|           |   |-- tinyxml2.cpp
|           |   |-- tinyxml2.h
|           |   `-- ...
|           `-- @<tinyxml2-commit>.checksum     # Normalized source-tree checksum
|
`-- project/
    `-- host/                                  # HARD_ENV: analysis and build environment
        |-- root/                              # Local project state
        |   |-- workspace/example/             # Absolute project directory without leading /
        |   |   |-- include/                   # Selected dependencies for this project
        |   |   |   |-- github.com/hard-build/
        |   |   |   |   `-- library -> .../snapshot/github.com/hard-build/library/@<hard-commit>
        |   |   |   `-- hard -> github.com/hard-build/library
        |   |   |                              # Short include namespace
        |   |   |-- fetch/<analysis-key>/       # Fetch analysis configuration
        |   |   |   `-- main.cpp.hard-parse-cache.json
        |   |   `-- build/<build-key>/          # Build configuration
        |   |       |-- main.cpp.hard-parse-cache.json
        |   |       |                          # Build/run/test analysis record
        |   |       |-- main.cpp.fwd.h         # Generated source-context declarations
        |   |       |-- main.cpp.o              # Local project object
        |   |       |-- main.cpp.o.hard-cache.json
        |   |       |                          # Object input and output validation
        |   |       |-- main                    # Internal project executable
        |   |       `-- main.hard-cache.json   # Link input and output validation
        |   |
        |   `-- workspace/example2/            # Second local project's state
        |       |-- include/
        |       |   `-- github.com/leethomason/
        |       |       `-- tinyxml2 -> .../snapshot/github.com/leethomason/tinyxml2/@<tinyxml2-commit>
        |       |                              # Source snapshot, not installed package
        |       |-- fetch/<analysis-key>/
        |       |   `-- main.cpp.hard-parse-cache.json
        |       |                              # Includes the local recipe's analysis inputs
        |       `-- build/<build-key>/
        |           |-- main.cpp.hard-parse-cache.json
        |           |-- main.cpp.fwd.h
        |           |-- main.cpp.o              # Second project's object
        |           |-- main.cpp.o.hard-cache.json
        |           |-- main                    # Linked against the installed static archive
        |           `-- main.hard-cache.json
        |
        `-- github.com/                        # Shared logical repository state
            |-- hard-build/library/
            |   |-- fetch/<analysis-key>/
            |   |   `-- ...                    # Discovered library translation-unit analysis
            |   `-- build/<build-key>/
            |       `-- ...                    # Library objects, forwards and cache records
            |
            `-- leethomason/tinyxml2/
                |-- fetch/<analysis-key>/
                |   `-- tinyxml2.cpp.hard-parse-cache.json
                `-- package/<fingerprint>/     # Recipe, source and build-tool identity
                    |-- manifest.json          # Selected generation and installed file digests
                    |-- generation-<id>/       # One package build; stable consumer paths
                    |   |-- build/             # CMake working files and intermediate outputs
                    |   `-- install/
                    |       |-- include/
                    |       |   `-- tinyxml2.h # Installed public headers
                    |       `-- lib/
                    |           `-- libtinyxml2.a
                    |                          # Installed static archive
                    `-- generation-<previous-id>/
                        `-- ...                # Retained for existing consumers
```

The version-independent Windows image additionally uses this environment-wide
runtime directory (it is not created for `host`):

```text
HARD_ROOT/project/<windows-HARD_ENV>/
`-- @runtime/                                  # Runtime state shared by this environment
    `-- wine/                                  # Shared Wine prefix
        |-- drive_c/                           # Wine's virtual Windows drive
        |-- dosdevices/                        # Wine-managed drive mappings
        `-- ...                                # Registry and other Wine-managed state
```

The Windows image's Wine launcher creates missing parent directories before
starting Wine. This keeps runtime initialization out of the backend and host
wrapper; `@runtime/wine` is initialized only when Wine is used.

## Storage and selection rules

- All external sources use `snapshot/<source>/@<commit>`, with the actual
  repository path rather than its hash. Corporate forks therefore have their
  own source directories. Proxy addresses do not participate in source identity.
- `@default` is a regular file containing a full commit ID and a newline. It
  is neither a directory nor a symlink. Its selected snapshot has its own
  `@<commit>.checksum`; no `@default.checksum` is created.
- Without project recording, first use resolves the default revision, downloads
  and verifies its snapshot, then atomically publishes `@default`. Later uses
  reuse and verify that revision without following a moving branch.
- Project pins and inherited requirements take precedence over the default.
  Explicit project updates and `--no-cache` never switch `@default`.
  `fetch --lock` can record a cached default without refreshing it. When only
  its commit is known, that exact commit is also its recorded `ref`.
- A project without `repositories` does not acquire or modify a dependency
  record automatically. The `hard.yaml` schema and explicit recording controls
  remain unchanged.
- There is no top-level `source`, `fetch` or `env` directory in the new layout.
  All generated state belongs to `project/<HARD_ENV>/root/<absolute-dir>` or
  `project/<HARD_ENV>/<logical-repository>`, apart from the Windows image's
  environment-wide `project/<HARD_ENV>/@runtime/wine` runtime state.
- Local projects have a single `include` directory, without selection-digest
  or dependency-set subdirectories. It exposes concrete selected snapshots and
  applicable `hard`/`recipe` aliases. A project/environment lock prevents
  concurrent invocations from switching these links during analysis or building.
  The lock currently spans the whole command, including run/test children.
  A child must not recursively invoke hard in the same directory/environment.
- Library source analysis and direct-compilation outputs belong to their logical
  repository, not to the consuming local project. Keys must retain the complete
  relevant analysis/build context; sharing storage does not permit mixing
  incompatible flags, dependency selections or project-specific include contexts.
  Direct compilation currently keeps each invocation directory's context in
  the keys; cross-project package reuse is independent of this restriction.
- Analysis/build directory keys distinguish configurations. Ordinary local
  source edits invalidate individual content-checked records rather than
  creating a complete new configuration tree. Relative source paths are
  preserved within each owner's cache.
  An explicitly selected local source outside the invocation directory uses
  its own directory as its local owner, retaining the invocation's context key.
- Recipe packages retain content fingerprints and immutable generations.
  The package lock covers validation, CMake work and manifest publication;
  consumers retain their selected generation paths after the lock is released.
  Failed builds cannot publish a usable manifest. Published old generations
  are not overwritten or automatically removed.
- `fetch` does not create objects, forwards, executables or recipe packages.
  Test executables may additionally have `.hard-test-cache.json` records.
- Old caches are not deleted or silently assigned revisions. No installed
  runtime, release tag, wrapper mount or credential-forwarding change is part
  of this migration.

## Migration and verification

The backend does not create the former top-level `source`, `fetch` or `env`
trees. Old caches are left untouched, and the first invocation after upgrading
may download and build again. Historical container images keep their historical
backend and runtime layout; changing cache paths does not republish those images.

Unrecorded commands can repeat discovery while rebuilding their in-memory
dependency selection, then reuse final-selection analysis records and cached
defaults. They never create `hard.yaml` implicitly.

Regression tests cover default reuse and concurrent publication, pinning and
inheritance, fork paths, corruption detection, owner and configuration routing,
project locks, cross-project recipe sharing, and retained package generations.
Run `make check` from the repository root for the complete local verification.
The [reference](reference.md) and [README](../README.md) describe the public
behavior and migration rules.
