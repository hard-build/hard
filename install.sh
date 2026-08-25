#!/bin/sh

set -eu

release_base_url=https://github.com/hard-build/hard/releases
release_tag=
release_download_url=
archive_name=
checksum_name=
download_directory=
install_stage=
install_complete=0
previous_runtime_moved=0
runtime_root=

fail() {
	printf 'hard installer: %s\n' "$1" >&2
	exit 1
}

cleanup() {
	if [ "$install_complete" -eq 0 ] && [ "$previous_runtime_moved" -eq 1 ] && \
		[ -d "$install_stage/previous-runtime" ]; then
		if [ -e "$runtime_root" ] || [ -L "$runtime_root" ]; then
			if ! mv "$runtime_root" "$install_stage/failed-runtime"; then
				printf 'hard installer: previous runtime remains at %s\n' \
					"$install_stage/previous-runtime" >&2
				install_stage=
			fi
		fi
		if [ -n "$install_stage" ] && \
			! mv "$install_stage/previous-runtime" "$runtime_root"; then
			printf 'hard installer: previous runtime remains at %s\n' \
				"$install_stage/previous-runtime" >&2
			install_stage=
		fi
	fi
	if [ -n "$install_stage" ] && [ -d "$install_stage" ]; then
		rm -rf "$install_stage"
	fi
	if [ -n "$download_directory" ] && [ -d "$download_directory" ]; then
		rm -rf "$download_directory"
	fi
}

trap cleanup 0
trap 'exit 1' HUP INT TERM

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

resolve_release() {
	if ! resolved_release_url=$(curl --fail --silent --show-error --location --retry 3 \
		--output /dev/null \
		--write-out '%{url_effective}' \
		"$release_base_url/latest"); then
		fail "cannot resolve latest release"
	fi

	release_tag=${resolved_release_url##*/}
	case "$release_tag" in
		v*) release_version=${release_tag#v} ;;
		*) fail "latest release has an invalid tag: $release_tag" ;;
	esac
	case "$release_version" in
		*.*)
			release_major=${release_version%%.*}
			release_minor=${release_version#*.}
			;;
		*) fail "latest release has an invalid tag: $release_tag" ;;
	esac
	if [ -z "$release_major" ] || [ -z "$release_minor" ]; then
		fail "latest release has an invalid tag: $release_tag"
	fi
	case "$release_major$release_minor" in
		*[!0-9]*) fail "latest release has an invalid tag: $release_tag" ;;
	esac

	release_download_url=$release_base_url/download/$release_tag
	archive_name=hard-$release_tag.tar.gz
	checksum_name=$archive_name.sha256
}

download_release() {
	download_directory=$(mktemp -d "${TMPDIR:-/tmp}/hard-download.XXXXXX")
	printf 'hard installer: downloading the portable Linux archive.\n'
	curl --fail --location --retry 3 \
		--output "$download_directory/$archive_name" \
		"$release_download_url/$archive_name"
	curl --fail --location --retry 3 \
		--output "$download_directory/$checksum_name" \
		"$release_download_url/$checksum_name"
	(
		cd "$download_directory"
		sha256sum --check "$checksum_name"
	)
	tar -xzf "$download_directory/$archive_name" --directory "$download_directory"

	archive_root=$download_directory/hard-linux-amd64
	[ -x "$archive_root/bin/hard" ] || fail "release archive has no executable wrapper"
	[ -x "$archive_root/libexec/hard/hard" ] || fail "release archive has no executable backend"
	[ -x "$archive_root/libexec/hard/bin/clang-format" ] || fail "release archive has no clang-format"
	[ -f "$archive_root/libexec/hard/hard.h" ] || fail "release archive has no hard.h"
	[ -f "$archive_root/libexec/hard/format/format.v1" ] || fail "release archive has no format.v1"
	[ -e "$archive_root/libexec/hard/lib/libclang.so" ] || fail "release archive has no libclang"
}

install_release() {
	local_bin=$HOME/.local/bin
	local_libexec=$HOME/.local/libexec
	runtime_root=$local_libexec/hard
	wrapper=$local_bin/hard

	mkdir -p "$local_bin" "$local_libexec" "$HOME/.local/share/hard"
	if [ -e "$runtime_root" ] || [ -L "$runtime_root" ]; then
		if [ ! -d "$runtime_root" ] || [ -L "$runtime_root" ]; then
			fail "refusing to replace non-directory runtime: $runtime_root"
		fi
	fi
	if [ -e "$wrapper" ] && [ ! -f "$wrapper" ] && [ ! -L "$wrapper" ]; then
		fail "refusing to replace non-file wrapper: $wrapper"
	fi

	install_stage=$(mktemp -d "$local_libexec/.hard-install.XXXXXX")
	cp -a "$archive_root/libexec/hard" "$install_stage/runtime"
	cp "$archive_root/bin/hard" "$install_stage/wrapper"
	chmod 0755 "$install_stage/wrapper"

	if ! mv "$install_stage/wrapper" "$wrapper"; then
		fail "cannot install wrapper into $wrapper"
	fi
	if [ -d "$runtime_root" ]; then
		previous_runtime_moved=1
		if ! mv "$runtime_root" "$install_stage/previous-runtime"; then
			previous_runtime_moved=0
			fail "cannot stage previous runtime from $runtime_root"
		fi
	fi
	if ! mv "$install_stage/runtime" "$runtime_root"; then
		fail "cannot install runtime into $runtime_root"
	fi
	install_complete=1
}

configure_path() {
	path_added_to_environment=0
	case ":${PATH:-}:" in
		*":$local_bin:"*) ;;
		*)
			PATH=$local_bin${PATH:+:$PATH}
			export PATH
			path_added_to_environment=1
			;;
	esac

	shell_name=${SHELL:-}
	shell_name=${shell_name##*/}
	case "$shell_name" in
		bash)
			shell_config=$HOME/.bashrc
			path_entry='export PATH="$HOME/.local/bin:$PATH"'
			;;
		zsh)
			shell_config=$HOME/.zshrc
			path_entry='export PATH="$HOME/.local/bin:$PATH"'
			;;
		fish)
			shell_config=$HOME/.config/fish/config.fish
			path_entry='fish_add_path "$HOME/.local/bin"'
			;;
		*)
			shell_name=${shell_name:-sh}
			shell_config=$HOME/.profile
			path_entry='export PATH="$HOME/.local/bin:$PATH"'
			;;
	esac

	if [ -e "$shell_config" ] || [ -L "$shell_config" ]; then
		[ -f "$shell_config" ] || fail "shell configuration is not a regular file: $shell_config"
	fi
	mkdir -p "${shell_config%/*}"
	path_already_configured=0
	if [ -f "$shell_config" ]; then
		while IFS= read -r line || [ -n "$line" ]; do
			case "$line" in
				\#*) continue ;;
				*'$HOME/.local/bin'* | *'~/.local/bin'* | *"$local_bin"*)
					path_already_configured=1
					break
					;;
			esac
		done < "$shell_config"
	fi
	if [ "$path_already_configured" -eq 0 ]; then
		if [ -s "$shell_config" ]; then
			printf '\n' >> "$shell_config"
		fi
		printf '%s\n' "$path_entry" >> "$shell_config"
		printf 'hard installer: added %s to %s.\n' "$local_bin" "$shell_config"
	else
		printf 'hard installer: %s is already configured in %s.\n' "$local_bin" "$shell_config"
	fi
}

if [ "$#" -ne 0 ]; then
	fail "no arguments are accepted"
fi
case "$(uname -s)" in
	Linux) ;;
	*) fail "the portable archive currently supports Linux only" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) ;;
	*) fail "the portable archive currently supports x86_64 only" ;;
esac
[ -n "${HOME:-}" ] || fail "HOME is not set"

require_command curl
require_command tar
require_command sha256sum
require_command mktemp
resolve_release
download_release
install_release
configure_path

printf '\nhard %s was installed in %s.\n' "$release_tag" "$HOME/.local"
if [ "$path_added_to_environment" -eq 1 ]; then
	printf 'To use hard in the current %s shell, run:\n  %s\n' "$shell_name" "$path_entry"
else
	printf '%s is already present in PATH.\n' "$local_bin"
fi
