# hard integration fixtures

This directory contains declarative positive integration scenarios for
`hard`. Each numbered directory owns its sources and one readable `test.yaml`
file describing, in execution order, what to build or test and what to verify.
Adding a scenario does not require editing a central list.

The Python runner discovers every immediate subdirectory containing
`test.yaml`. It supports only the `build`, `run`, and `test` actions described
below; scenario files cannot execute arbitrary shell commands.

## Requirements

The runner requires Python 3 and PyYAML. Install the test dependency into the
chosen Python environment when it is not already available:

```bash
python3 -m pip install -r unittest/requirements.txt
```

Most C and C++ fixtures use only local and system headers. The
`011.compiled_library_recipe` scenario downloads TinyXML2 from GitHub and
requires CMake in addition to the configured C++ compiler. GoogleTest
scenarios require `pkg-config` and `gtest_main`, as does `hard test` itself.

## Running scenarios

Run every discovered scenario from the repository root:

```bash
make -C unittest
```

Run one scenario through Make:

```bash
make -C unittest SCENARIO=003.transitive_dependency
```

Run one or more scenarios directly:

```bash
python3 unittest/run.py 003.transitive_dependency
python3 unittest/run.py 003.transitive_dependency 004.circular_dependency
```

The Make variables and their defaults are:

```make
PYTHON = python3
HARD = hard
OUTPUT = /tmp/hard-unittest
JOBS = 0
SCENARIO =
```

Override them without editing the Makefile:

```bash
make -C unittest \
  PYTHON=python3 \
  HARD=/tmp/hard-check \
  OUTPUT=/tmp/hard-unittest-binaries \
  JOBS=4
```

`OUTPUT/<scenario>` receives delivered application binaries. Generated
headers, objects, and GoogleTest executables remain below `HARD_ROOT`. No
build artifact is written into a fixture source directory.

## Scenario format

A scenario has a non-empty description and a non-empty ordered list of steps:

```yaml
description: Discover transitive production implementations.

steps:
  - build:
      sources:
        - example.cpp
      compiled_once:
        - formatter/formatter.cpp
        - value/value.cpp

  - run:
      binary: example
      exit_code: 0
      stdout: |
        value=42
```

Each list item must contain exactly one action. Actions execute from top to
bottom. Unknown actions, unknown fields, duplicate YAML keys, incorrect value
types, absolute paths, and paths containing `..` are configuration errors.

The runner deliberately passes `--no-cache` to every `build` and `test` action.
This keeps `compiled_once`, copied-binary, and executed-test assertions about
the current scenario step independent of artifacts left by an earlier run.
Active compiled library packages are rebuilt for those steps; downloaded
GitHub source snapshots remain shared in `HARD_ROOT/source`.

### `build`

The `build` action runs the equivalent of:

```text
hard build --no-cache -v --no-color --jobs=<JOBS> \
  -o <OUTPUT>/<scenario>/ <sources...>
```

Fields:

- `sources` is a required non-empty list of explicit source or directory
  arguments;
- `compiled_once` is an optional non-empty list of exact relative progress
  labels that must each occur in one `Compiling` event.

Every later `run` action uses the output of the most recent `build` action.
The runner requires that build to contain exactly one matching `Copying` event,
so a stale binary below `OUTPUT` cannot satisfy the scenario.

### `run`

The `run` action executes one delivered application. It must follow a `build`
action.

Fields:

- `binary` is the required path relative to `OUTPUT/<scenario>`;
- `exit_code` is a required integer from 0 through 255;
- `stdout` is the required exact standard output;
- `stderr` is optional exact standard error and defaults to an empty string.

YAML literal blocks make line endings visible. `|` includes the final newline;
use `|-` when no final newline is expected:

```yaml
  - run:
      binary: example
      exit_code: 0
      stdout: |-
        output without a final newline
```

The binary must be a regular executable, must have been copied by the latest
build step, and must match all three expected process results.

### `test`

The `test` action runs the equivalent of:

```text
hard test --no-cache -v --no-color --jobs=<JOBS> <sources...>
```

Fields:

- `sources` is a required non-empty list of explicit test source or directory
  arguments;
- `compiled_once` is an optional non-empty list of exact relative progress
  labels that must each occur in one `Compiling` event;
- `binaries` is a required non-empty mapping from each exact `Testing` label to
  its expected positive `passed` count.

Example:

```yaml
  - test:
      sources:
        - .
      compiled_once:
        - counter.cpp
      binaries:
        increment_test:
          passed: 3
        reset_test:
          passed: 2
```

The `hard test` command must return status zero. Every configured binary must
have exactly one `Testing` block, no unconfigured test binary may run, and the
GoogleTest block belonging to each binary must contain exactly one matching
passing summary.

## Adding a scenario

Create one new immediate subdirectory with its sources and `test.yaml`:

```text
unittest/
└── 011.new_scenario/
    ├── example.cpp
    └── test.yaml
```

The next full run discovers it automatically. Keep every scenario-specific
command argument and expectation in that local YAML file. Change `run.py` only
when the common, deliberately limited action vocabulary itself must change.
