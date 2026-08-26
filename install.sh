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
step_number=0
step_total=8
style_bold=
style_cyan=
style_green=
style_red=
style_reset=

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	style_bold=$(printf '\033[1m')
	style_cyan=$(printf '\033[36m')
	style_green=$(printf '\033[32m')
	style_red=$(printf '\033[31m')
	style_reset=$(printf '\033[0m')
fi

print_header() {
	printf '%sHard Build installer%s\n' "$style_bold" "$style_reset"
}

print_step() {
	step_number=$((step_number + 1))
	printf '\n%s%s[%d/%d]%s %s\n' \
		"$style_bold" "$style_cyan" "$step_number" "$step_total" \
		"$style_reset" "$1"
}

print_detail() {
	printf '  %s\n' "$1"
}

fail() {
	printf '%s%shard installer: error:%s %s\n' \
		"$style_bold" "$style_red" "$style_reset" "$1" >&2
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
	print_detail "Selected release: $release_tag"
}

download_file() {
	print_detail "$2"
	curl --fail --location --retry 3 \
		--progress-bar --show-error \
		--output "$download_directory/$1" \
		"$2"
}

verify_download() {
	if ! (
		cd "$download_directory"
		sha256sum --check "$checksum_name"
	) >/dev/null; then
		fail "checksum verification failed for $archive_name"
	fi
	print_detail "SHA-256 checksum matches."
}

extract_release() {
	tar -xzf "$download_directory/$archive_name" --directory "$download_directory"

	archive_root=$download_directory/hard-linux-amd64
	[ -x "$archive_root/bin/hard" ] || fail "release archive has no executable wrapper"
	[ -x "$archive_root/libexec/hard/hard" ] || fail "release archive has no executable backend"
	[ -x "$archive_root/libexec/hard/bin/clang-format" ] || fail "release archive has no clang-format"
	[ -f "$archive_root/libexec/hard/hard.h" ] || fail "release archive has no hard.h"
	[ -f "$archive_root/libexec/hard/format/format.v1" ] || fail "release archive has no format.v1"
	[ -e "$archive_root/libexec/hard/lib/libclang.so" ] || fail "release archive has no libclang"
	[ -f "$archive_root/share/bash-completion/completions/hard" ] ||
		fail "release archive has no Bash completion"
	[ -f "$archive_root/share/zsh/site-functions/_hard" ] ||
		fail "release archive has no Zsh completion"
	[ -f "$archive_root/share/fish/vendor_completions.d/hard.fish" ] ||
		fail "release archive has no Fish completion"
	print_detail "Release contents are complete."
}

install_release() {
	local_bin=$HOME/.local/bin
	local_libexec=$HOME/.local/libexec
	local_share=$HOME/.local/share
	runtime_root=$local_libexec/hard
	wrapper=$local_bin/hard
	bash_completion=$local_share/bash-completion/completions/hard
	zsh_completion=$local_share/zsh/site-functions/_hard
	fish_completion=$local_share/fish/vendor_completions.d/hard.fish

	mkdir -p \
		"$local_bin" \
		"$local_libexec" \
		"$local_share/hard" \
		"${bash_completion%/*}" \
		"${zsh_completion%/*}" \
		"${fish_completion%/*}"
	if [ -e "$runtime_root" ] || [ -L "$runtime_root" ]; then
		if [ ! -d "$runtime_root" ] || [ -L "$runtime_root" ]; then
			fail "refusing to replace non-directory runtime: $runtime_root"
		fi
	fi
	if [ -e "$wrapper" ] && [ ! -f "$wrapper" ] && [ ! -L "$wrapper" ]; then
		fail "refusing to replace non-file wrapper: $wrapper"
	fi
	for completion_file in "$bash_completion" "$zsh_completion" "$fish_completion"; do
		if [ -e "$completion_file" ] || [ -L "$completion_file" ]; then
			if [ ! -f "$completion_file" ] || [ -L "$completion_file" ]; then
				fail "refusing to replace non-file completion: $completion_file"
			fi
		fi
	done

	install_stage=$(mktemp -d "$local_libexec/.hard-install.XXXXXX")
	cp -a "$archive_root/libexec/hard" "$install_stage/runtime"
	cp "$archive_root/bin/hard" "$install_stage/wrapper"
	cp "$archive_root/share/bash-completion/completions/hard" "$install_stage/bash-completion"
	cp "$archive_root/share/zsh/site-functions/_hard" "$install_stage/zsh-completion"
	cp "$archive_root/share/fish/vendor_completions.d/hard.fish" "$install_stage/fish-completion"
	chmod 0755 "$install_stage/wrapper"
	chmod 0644 \
		"$install_stage/bash-completion" \
		"$install_stage/zsh-completion" \
		"$install_stage/fish-completion"

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
	if ! mv "$install_stage/bash-completion" "$bash_completion"; then
		fail "cannot install Bash completion into $bash_completion"
	fi
	if ! mv "$install_stage/zsh-completion" "$zsh_completion"; then
		fail "cannot install Zsh completion into $zsh_completion"
	fi
	if ! mv "$install_stage/fish-completion" "$fish_completion"; then
		fail "cannot install Fish completion into $fish_completion"
	fi
	install_complete=1
	print_detail "Command: $wrapper"
	print_detail "Runtime: $runtime_root"
	print_detail "Completions: $local_share"
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
			completion_token='bash-completion/completions/hard'
			completion_entry='[ -r "$HOME/.local/share/bash-completion/completions/hard" ] && . "$HOME/.local/share/bash-completion/completions/hard"'
			;;
		zsh)
			shell_config=$HOME/.zshrc
			path_entry='export PATH="$HOME/.local/bin:$PATH"'
			completion_token='zsh/site-functions/_hard'
			completion_entry='autoload -Uz compinit && compinit && . "$HOME/.local/share/zsh/site-functions/_hard"'
			;;
		fish)
			shell_config=$HOME/.config/fish/config.fish
			path_entry='fish_add_path "$HOME/.local/bin"'
			completion_entry=
			;;
		*)
			shell_name=${shell_name:-sh}
			shell_config=$HOME/.profile
			path_entry='export PATH="$HOME/.local/bin:$PATH"'
			completion_entry=
			;;
	esac
	print_step "Configuring the $shell_name shell"

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
		print_detail "Added $local_bin to $shell_config."
	else
		print_detail "$local_bin is already configured in $shell_config."
	fi

	if [ -n "$completion_entry" ]; then
		completion_already_configured=0
		if [ -f "$shell_config" ]; then
			while IFS= read -r line || [ -n "$line" ]; do
				case "$line" in
					\#*) continue ;;
					*"$completion_token"*)
						completion_already_configured=1
						break
						;;
				esac
			done < "$shell_config"
		fi
		if [ "$completion_already_configured" -eq 0 ]; then
			if [ -s "$shell_config" ]; then
				printf '\n' >> "$shell_config"
			fi
			printf '%s\n' "$completion_entry" >> "$shell_config"
			print_detail "Enabled $shell_name completion in $shell_config."
		else
			print_detail "$shell_name completion is already configured in $shell_config."
		fi
	elif [ "$shell_name" = fish ]; then
		print_detail "Fish discovers completion from $fish_completion."
	else
		print_detail "Programmable completion is not configured for $shell_name."
	fi
}

print_summary() {
	printf '\n%s%sInstallation complete%s\n' \
		"$style_bold" "$style_green" "$style_reset"
	printf '  hard %s was installed in %s.\n' "$release_tag" "$HOME/.local"
	if [ "$path_added_to_environment" -eq 1 ]; then
		printf '  To use hard in the current %s shell, run:\n    %s\n' \
			"$shell_name" "$path_entry"
	else
		printf '  %s is already present in PATH.\n' "$local_bin"
	fi

	printf '\n%sNext steps%s\n' "$style_bold" "$style_reset"
	printf '\n%sBuild a C++ hello world on the host%s\n' "$style_bold" "$style_reset"
	printf '  The minimum requirement is a compiler with C++20 support.\n'
	printf '  Install it for your distribution:\n\n'
	printf '    Ubuntu 22.04+/Debian 12+:  sudo apt update && sudo apt install g++\n'
	printf '    Arch/CachyOS:               sudo pacman -S gcc\n'
	printf '    Fedora/RHEL 9+/Rocky 9+:    sudo dnf install gcc-c++\n'
	printf '    openSUSE Tumbleweed:        sudo zypper install gcc-c++\n'
	printf '\n  Alpine uses musl, while the portable host runtime requires glibc.\n'
	printf '  Use the Docker target shown below on Alpine.\n'
	printf '\n  Then verify the compiler and build your source:\n\n'
	printf '    c++ --version\n'
	printf '    hard build example.cpp\n'
	printf '\n  If Docker is already installed, a host compiler is not required:\n\n'
	printf '    hard --target=linux64 build example.cpp\n'
	printf '\n  These are recommendations only; the installer did not run them.\n'
}

if [ "$#" -ne 0 ]; then
	fail "no arguments are accepted"
fi
print_header
print_step "Checking system compatibility"
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
print_detail "Linux x86-64 and required archive tools are available."

print_step "Resolving the latest hard release"
resolve_release

download_directory=$(mktemp -d "${TMPDIR:-/tmp}/hard-download.XXXXXX")
print_step "Downloading $archive_name"
download_file "$archive_name" "$release_download_url/$archive_name"

print_step "Downloading $checksum_name"
download_file "$checksum_name" "$release_download_url/$checksum_name"

print_step "Verifying the archive checksum"
verify_download

print_step "Extracting and validating the release"
extract_release

print_step "Installing hard $release_tag"
install_release
configure_path
print_summary
