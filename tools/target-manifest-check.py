#!/usr/bin/env python3

import json
import re
import sys
from pathlib import Path


NAME = re.compile(r"[a-z0-9]+(?:[._-][a-z0-9]+)*")
VERSIONED_DOCKERFILE = re.compile(r"v[0-9]+\.[0-9]+-.+\.Dockerfile")
REQUIRED_ARGUMENTS = {"HARD_REVISION", "HARD_VERSION", "IMAGE_VERSION"}


def fail(message: str) -> None:
    raise SystemExit(f"target manifest: {message}")


def main() -> None:
    if len(sys.argv) > 2:
        fail("usage: target-manifest-check.py [target/manifest.json]")

    manifest = Path(sys.argv[1]) if len(sys.argv) == 2 else Path("target/manifest.json")
    root = manifest.resolve().parent.parent
    try:
        data = json.loads(manifest.read_text())
    except (OSError, json.JSONDecodeError) as error:
        fail(str(error))

    if not isinstance(data, dict) or set(data) != {"schema", "targets"}:
        fail("root must contain exactly schema and targets")
    if data["schema"] != 1:
        fail("schema must be 1")
    if not isinstance(data["targets"], list) or not data["targets"]:
        fail("targets must be a non-empty array")

    listed = set()
    identities = set()
    latest_images = set()
    ordering = []

    for index, target in enumerate(data["targets"]):
        location = f"targets[{index}]"
        if not isinstance(target, dict) or set(target) != {
            "dockerfile",
            "image",
            "publish_latest",
            "variant",
        }:
            fail(f"{location} has invalid fields")

        image = target["image"]
        variant = target["variant"]
        dockerfile = target["dockerfile"]
        publish_latest = target["publish_latest"]
        if not isinstance(image, str) or NAME.fullmatch(image) is None:
            fail(f"{location}.image is invalid")
        if not isinstance(variant, str) or NAME.fullmatch(variant) is None:
            fail(f"{location}.variant is invalid")
        if not isinstance(dockerfile, str):
            fail(f"{location}.dockerfile is invalid")
        if not isinstance(publish_latest, bool):
            fail(f"{location}.publish_latest must be boolean")

        expected = f"target/{image}/{variant}.Dockerfile"
        if dockerfile != expected:
            fail(f"{location}.dockerfile must be {expected}")
        if dockerfile in listed:
            fail(f"duplicate Dockerfile: {dockerfile}")
        identity = (image, variant)
        if identity in identities:
            fail(f"duplicate target: {image}:{variant}")
        if publish_latest and image in latest_images:
            fail(f"multiple latest targets for image: {image}")

        path = root / dockerfile
        if path.is_symlink() or not path.is_file():
            fail(f"Dockerfile is not a regular file: {dockerfile}")
        text = path.read_text()
        arguments = set()
        for line in text.splitlines():
            match = re.fullmatch(r"ARG\s+([A-Z][A-Z0-9_]*)(?:=(.*))?", line)
            if match is None or match.group(1) not in REQUIRED_ARGUMENTS:
                continue
            if match.group(2) is not None:
                fail(f"{dockerfile} gives {match.group(1)} a default")
            arguments.add(match.group(1))
        if arguments != REQUIRED_ARGUMENTS:
            missing = ", ".join(sorted(REQUIRED_ARGUMENTS - arguments))
            fail(f"{dockerfile} is missing build arguments: {missing}")
        if "-X main.versionPrerelease=" not in text:
            fail(f"{dockerfile} does not clear the prerelease identifier")
        if "/usr/local/libexec/hard/VERSION" in text or "/tmp/hard-version" in text:
            fail(f"{dockerfile} installs a runtime VERSION file")
        if f'ENV HARD_ENV={image}:${{IMAGE_VERSION}}' not in text:
            fail(f"{dockerfile} does not derive HARD_ENV from IMAGE_VERSION")
        if f'"${{HARD_VERSION}}-{variant}"' not in text:
            fail(f"{dockerfile} does not bind IMAGE_VERSION to its variant")

        listed.add(dockerfile)
        identities.add(identity)
        if publish_latest:
            latest_images.add(image)
        ordering.append(identity)

    if ordering != sorted(ordering):
        fail("targets must be ordered by image and variant")

    generic = {
        path.relative_to(root).as_posix()
        for path in (root / "target").glob("*/*.Dockerfile")
        if VERSIONED_DOCKERFILE.fullmatch(path.name) is None
    }
    if listed != generic:
        missing = sorted(generic - listed)
        unknown = sorted(listed - generic)
        details = []
        if missing:
            details.append("unlisted: " + ", ".join(missing))
        if unknown:
            details.append("not generic: " + ", ".join(unknown))
        fail("; ".join(details))

    print(f"Target manifest verified: {len(listed)} targets")


if __name__ == "__main__":
    main()
