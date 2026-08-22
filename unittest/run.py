from __future__ import annotations

import argparse
from collections import Counter
from dataclasses import dataclass
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
from typing import Any

try:
    import yaml
except ModuleNotFoundError:
    print(
        "hard unittest: PyYAML is required; "
        "install unittest/requirements.txt",
        file=sys.stderr,
    )
    raise SystemExit(2)


CONFIG_NAME = "test.yaml"
PROGRESS_PATTERN = re.compile(
    r"^\[\d+/(?:\d+|\?)\] (?P<action>[A-Za-z]+)(?: (?P<detail>.*))?$",
    re.MULTILINE,
)
GTEST_PASSED_PATTERN = re.compile(
    r"^\[  PASSED  \] (?P<count>\d+) tests?\.$",
    re.MULTILINE,
)


class ConfigurationError(Exception):
    pass


class CheckError(Exception):
    pass


class UniqueKeyLoader(yaml.SafeLoader):
    pass


def construct_unique_mapping(
    loader: UniqueKeyLoader, node: yaml.MappingNode, deep: bool = False
) -> dict[Any, Any]:
    mapping: dict[Any, Any] = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        try:
            duplicate = key in mapping
        except TypeError as error:
            raise yaml.constructor.ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                "found an unhashable mapping key",
                key_node.start_mark,
            ) from error
        if duplicate:
            raise yaml.constructor.ConstructorError(
                "while constructing a mapping",
                node.start_mark,
                f"found duplicate key {key!r}",
                key_node.start_mark,
            )
        mapping[key] = loader.construct_object(value_node, deep=deep)
    return mapping


UniqueKeyLoader.add_constructor(
    yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG,
    construct_unique_mapping,
)


@dataclass(frozen=True)
class ProgressEntry:
    action: str
    detail: str
    output: str


@dataclass(frozen=True)
class RunnerOptions:
    hard: str
    output: Path
    jobs: int


def require_mapping(value: Any, location: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ConfigurationError(f"{location} must be a mapping")
    for key in value:
        if not isinstance(key, str):
            raise ConfigurationError(f"{location} contains a non-string key")
    return value


def require_keys(
    mapping: dict[str, Any],
    *,
    allowed: set[str],
    required: set[str],
    location: str,
) -> None:
    unknown = sorted(set(mapping) - allowed)
    if unknown:
        raise ConfigurationError(
            f"{location} contains unknown field {unknown[0]!r}"
        )
    missing = sorted(required - set(mapping))
    if missing:
        raise ConfigurationError(
            f"{location} is missing required field {missing[0]!r}"
        )


def require_nonempty_string(value: Any, location: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ConfigurationError(f"{location} must be a non-empty string")
    if "\0" in value:
        raise ConfigurationError(f"{location} must not contain a NUL byte")
    return value


def require_relative_path(value: Any, location: str, *, allow_dot: bool) -> str:
    path_value = require_nonempty_string(value, location)
    path = Path(path_value)
    if path.is_absolute() or ".." in path.parts:
        raise ConfigurationError(
            f"{location} must be a relative path that does not contain '..'"
        )
    if not allow_dot and path == Path("."):
        raise ConfigurationError(f"{location} must name a file or binary")
    return path_value


def require_path_list(
    value: Any, location: str, *, allow_dot: bool
) -> list[str]:
    if not isinstance(value, list) or not value:
        raise ConfigurationError(f"{location} must be a non-empty list")
    return [
        require_relative_path(item, f"{location}[{index}]", allow_dot=allow_dot)
        for index, item in enumerate(value)
    ]


def validate_build(value: Any, location: str) -> dict[str, Any]:
    build = require_mapping(value, location)
    require_keys(
        build,
        allowed={"sources", "compiled_once"},
        required={"sources"},
        location=location,
    )
    build["sources"] = require_path_list(
        build["sources"], f"{location}.sources", allow_dot=True
    )
    if "compiled_once" in build:
        build["compiled_once"] = require_path_list(
            build["compiled_once"],
            f"{location}.compiled_once",
            allow_dot=False,
        )
    return build


def validate_run(value: Any, location: str) -> dict[str, Any]:
    run = require_mapping(value, location)
    require_keys(
        run,
        allowed={"binary", "exit_code", "stdout", "stderr"},
        required={"binary", "exit_code", "stdout"},
        location=location,
    )
    run["binary"] = require_relative_path(
        run["binary"], f"{location}.binary", allow_dot=False
    )
    exit_code = run["exit_code"]
    if type(exit_code) is not int or not 0 <= exit_code <= 255:
        raise ConfigurationError(
            f"{location}.exit_code must be an integer from 0 through 255"
        )
    if not isinstance(run["stdout"], str):
        raise ConfigurationError(f"{location}.stdout must be a string")
    if "stderr" in run and not isinstance(run["stderr"], str):
        raise ConfigurationError(f"{location}.stderr must be a string")
    run.setdefault("stderr", "")
    return run


def validate_test(value: Any, location: str) -> dict[str, Any]:
    test = require_mapping(value, location)
    require_keys(
        test,
        allowed={"sources", "compiled_once", "binaries"},
        required={"sources", "binaries"},
        location=location,
    )
    test["sources"] = require_path_list(
        test["sources"], f"{location}.sources", allow_dot=True
    )
    if "compiled_once" in test:
        test["compiled_once"] = require_path_list(
            test["compiled_once"],
            f"{location}.compiled_once",
            allow_dot=False,
        )
    binaries = require_mapping(test["binaries"], f"{location}.binaries")
    if not binaries:
        raise ConfigurationError(f"{location}.binaries must not be empty")
    for binary, raw_expectation in binaries.items():
        require_relative_path(
            binary, f"{location}.binaries key", allow_dot=False
        )
        expectation = require_mapping(
            raw_expectation, f"{location}.binaries.{binary}"
        )
        require_keys(
            expectation,
            allowed={"passed"},
            required={"passed"},
            location=f"{location}.binaries.{binary}",
        )
        passed = expectation["passed"]
        if type(passed) is not int or passed < 1:
            raise ConfigurationError(
                f"{location}.binaries.{binary}.passed must be a positive integer"
            )
    test["binaries"] = binaries
    return test


def load_scenario(path: Path) -> dict[str, Any]:
    try:
        with path.open("r", encoding="utf-8") as source:
            configuration = yaml.load(source, Loader=UniqueKeyLoader)
    except (OSError, UnicodeError, yaml.YAMLError) as error:
        raise ConfigurationError(f"cannot read {path}: {error}") from error

    root = require_mapping(configuration, str(path))
    require_keys(
        root,
        allowed={"description", "steps"},
        required={"description", "steps"},
        location=str(path),
    )
    root["description"] = require_nonempty_string(
        root["description"], f"{path}.description"
    )
    steps = root["steps"]
    if not isinstance(steps, list) or not steps:
        raise ConfigurationError(f"{path}.steps must be a non-empty list")

    validated_steps: list[dict[str, Any]] = []
    has_hard_command = False
    has_build = False
    for index, raw_step in enumerate(steps, start=1):
        location = f"{path}.steps[{index}]"
        step = require_mapping(raw_step, location)
        if len(step) != 1:
            raise ConfigurationError(
                f"{location} must contain exactly one action"
            )
        action = next(iter(step))
        if action == "build":
            value = validate_build(step[action], f"{location}.build")
            has_build = True
            has_hard_command = True
        elif action == "run":
            if not has_build:
                raise ConfigurationError(
                    f"{location}.run must follow a build action"
                )
            value = validate_run(step[action], f"{location}.run")
        elif action == "test":
            value = validate_test(step[action], f"{location}.test")
            has_hard_command = True
        else:
            raise ConfigurationError(
                f"{location} contains unknown action {action!r}"
            )
        validated_steps.append({action: value})

    if not has_hard_command:
        raise ConfigurationError(
            f"{path}.steps must contain a build or test action"
        )
    root["steps"] = validated_steps
    return root


def parse_progress(output: str) -> list[ProgressEntry]:
    matches = list(PROGRESS_PATTERN.finditer(output))
    entries: list[ProgressEntry] = []
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(output)
        entries.append(
            ProgressEntry(
                action=match.group("action"),
                detail=match.group("detail") or "",
                output=output[match.end() : end],
            )
        )
    return entries


def require_compiled_once(output: str, expected_sources: list[str]) -> None:
    counts = Counter(
        entry.detail for entry in parse_progress(output) if entry.action == "Compiling"
    )
    for source in expected_sources:
        if counts[source] != 1:
            raise CheckError(
                f"expected one compilation of {source!r}, found {counts[source]}"
            )


def print_command(command: list[str]) -> None:
    print(f"$ {shlex.join(command)}", flush=True)


def print_captured(output: str) -> None:
    if not output:
        return
    sys.stdout.write(output)
    if not output.endswith("\n"):
        sys.stdout.write("\n")
    sys.stdout.flush()


def run_hard(command: list[str], scenario_directory: Path) -> str:
    print_command(command)
    try:
        completed = subprocess.run(
            command,
            cwd=scenario_directory,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            errors="replace",
            check=False,
        )
    except OSError as error:
        raise CheckError(f"cannot start {command[0]!r}: {error}") from error
    print_captured(completed.stdout)
    if completed.returncode != 0:
        raise CheckError(
            f"{command[1]} exited with status {completed.returncode}"
        )
    return completed.stdout


def run_build(
    step: dict[str, Any],
    scenario_directory: Path,
    scenario_output: Path,
    options: RunnerOptions,
) -> str:
    command = [
        options.hard,
        "build",
        "-v",
        "--no-color",
        f"--jobs={options.jobs}",
        "-o",
        str(scenario_output) + os.sep,
        *step["sources"],
    ]
    output = run_hard(command, scenario_directory)
    require_compiled_once(output, step.get("compiled_once", []))
    return output


def run_application(
    step: dict[str, Any],
    scenario_directory: Path,
    scenario_output: Path,
    build_output: str | None,
) -> None:
    if build_output is None:
        raise CheckError("run action has no preceding build output")
    binary = scenario_output.joinpath(step["binary"])
    copy_count = sum(
        1
        for entry in parse_progress(build_output)
        if entry.action == "Copying" and entry.detail == str(binary)
    )
    if copy_count != 1:
        raise CheckError(
            f"latest build copied {str(binary)!r} {copy_count} times instead of once"
        )
    if binary.is_symlink() or not binary.is_file():
        raise CheckError(f"missing regular executable: {binary}")
    if not os.access(binary, os.X_OK):
        raise CheckError(f"binary is not executable: {binary}")

    command = [str(binary)]
    print_command(command)
    try:
        completed = subprocess.run(
            command,
            cwd=scenario_directory,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            errors="replace",
            check=False,
        )
    except OSError as error:
        raise CheckError(f"cannot start {binary}: {error}") from error

    if completed.returncode != step["exit_code"]:
        raise CheckError(
            f"{binary} returned {completed.returncode}, "
            f"expected {step['exit_code']}"
        )
    if completed.stdout != step["stdout"]:
        raise CheckError(
            f"unexpected stdout from {binary}: "
            f"expected {step['stdout']!r}, got {completed.stdout!r}"
        )
    if completed.stderr != step["stderr"]:
        raise CheckError(
            f"unexpected stderr from {binary}: "
            f"expected {step['stderr']!r}, got {completed.stderr!r}"
        )


def require_gtest_binaries(
    output: str, expected_binaries: dict[str, dict[str, int]]
) -> None:
    testing = [
        entry for entry in parse_progress(output) if entry.action == "Testing"
    ]
    actual_counts = Counter(entry.detail for entry in testing)
    expected_names = set(expected_binaries)
    actual_names = set(actual_counts)
    if actual_names != expected_names:
        missing = sorted(expected_names - actual_names)
        unexpected = sorted(actual_names - expected_names)
        details: list[str] = []
        if missing:
            details.append(f"missing {missing}")
        if unexpected:
            details.append(f"unexpected {unexpected}")
        raise CheckError("GoogleTest binary mismatch: " + ", ".join(details))

    for binary, expectation in expected_binaries.items():
        if actual_counts[binary] != 1:
            raise CheckError(
                f"expected one execution of {binary!r}, found {actual_counts[binary]}"
            )
        entry = next(item for item in testing if item.detail == binary)
        passed_matches = list(GTEST_PASSED_PATTERN.finditer(entry.output))
        if len(passed_matches) != 1:
            raise CheckError(
                f"expected one passing summary for {binary!r}, "
                f"found {len(passed_matches)}"
            )
        passed = int(passed_matches[0].group("count"))
        if passed != expectation["passed"]:
            raise CheckError(
                f"{binary!r} passed {passed} tests, "
                f"expected {expectation['passed']}"
            )


def run_tests(
    step: dict[str, Any],
    scenario_directory: Path,
    options: RunnerOptions,
) -> None:
    command = [
        options.hard,
        "test",
        "-v",
        "--no-color",
        f"--jobs={options.jobs}",
        *step["sources"],
    ]
    output = run_hard(command, scenario_directory)
    require_compiled_once(output, step.get("compiled_once", []))
    require_gtest_binaries(output, step["binaries"])


def run_scenario(
    directory: Path, configuration: dict[str, Any], options: RunnerOptions
) -> None:
    print(f"\n=== {directory.name}: {configuration['description']} ===", flush=True)
    scenario_output = options.output / directory.name
    latest_build_output: str | None = None
    for raw_step in configuration["steps"]:
        action, step = next(iter(raw_step.items()))
        if action == "build":
            latest_build_output = run_build(
                step, directory, scenario_output, options
            )
        elif action == "run":
            run_application(
                step,
                directory,
                scenario_output,
                latest_build_output,
            )
        else:
            run_tests(step, directory, options)
    print(f"{directory.name} passed", flush=True)


def scenario_directories(root: Path, requested: list[str]) -> list[Path]:
    if not requested:
        return sorted(
            path
            for path in root.iterdir()
            if path.is_dir() and path.joinpath(CONFIG_NAME).is_file()
        )

    seen: set[str] = set()
    directories: list[Path] = []
    for name in requested:
        if not name or Path(name).name != name or name in {".", ".."}:
            raise ConfigurationError(
                f"scenario must be an immediate directory name: {name!r}"
            )
        if name in seen:
            raise ConfigurationError(f"duplicate scenario: {name}")
        seen.add(name)
        directory = root / name
        if not directory.is_dir():
            raise ConfigurationError(f"scenario directory does not exist: {directory}")
        if not directory.joinpath(CONFIG_NAME).is_file():
            raise ConfigurationError(
                f"scenario has no {CONFIG_NAME}: {directory}"
            )
        directories.append(directory)
    return directories


def nonnegative_integer(value: str) -> int:
    try:
        parsed = int(value)
    except ValueError as error:
        raise argparse.ArgumentTypeError("must be a non-negative integer") from error
    if parsed < 0:
        raise argparse.ArgumentTypeError("must be a non-negative integer")
    return parsed


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run declarative hard integration scenarios."
    )
    parser.add_argument("--hard", default="hard", help="hard executable")
    parser.add_argument(
        "--output",
        default="/tmp/hard-unittest",
        help="application output root",
    )
    parser.add_argument(
        "--jobs",
        type=nonnegative_integer,
        default=0,
        help="hard job count; zero uses all logical CPUs",
    )
    parser.add_argument(
        "scenarios",
        nargs="*",
        help="scenario directory names; all test.yaml directories by default",
    )
    return parser.parse_args()


def main() -> int:
    arguments = parse_arguments()
    root = Path(__file__).resolve().parent
    options = RunnerOptions(
        hard=arguments.hard,
        output=Path(arguments.output).resolve(),
        jobs=arguments.jobs,
    )
    try:
        directories = scenario_directories(root, arguments.scenarios)
    except (OSError, ConfigurationError) as error:
        print(f"hard unittest: {error}", file=sys.stderr)
        return 2
    if not directories:
        print(f"hard unittest: no {CONFIG_NAME} scenarios found", file=sys.stderr)
        return 2

    failures = 0
    for directory in directories:
        try:
            configuration = load_scenario(directory / CONFIG_NAME)
            run_scenario(directory, configuration, options)
        except (ConfigurationError, CheckError) as error:
            failures += 1
            print(f"{directory.name} failed: {error}", file=sys.stderr, flush=True)

    if failures:
        print(
            f"{failures} hard integration scenario(s) failed",
            file=sys.stderr,
        )
        return 1
    print(f"all {len(directories)} hard integration scenarios passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
