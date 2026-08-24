#!/bin/sh

set -eu

release_base_url=https://github.com/hard-build/hard/releases/latest/download
archive_name=hard-linux-amd64.tar.gz
checksum_name=$archive_name.sha256
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

usage() {
	cat <<'EOF'
Usage: install.sh [docker|host|both]

Installation modes:
  docker  Recommended. Installs Docker and uses the reproducible linux.v1
          container by default. Native C++ build dependencies are not installed.
  host    Installs the native compiler, pkg-config, and GoogleTest development
          files. Builds run directly on this system by default. Optional external
          library build tools are not installed.
  both    Installs both dependency sets. linux.v1 remains the default; pass
          --target=host to run a native build.
EOF
}

choose_mode() {
	if [ "$#" -gt 1 ]; then
		usage >&2
		fail "expected at most one installation mode"
	fi
	if [ "$#" -eq 1 ]; then
		case "$1" in
			docker | host | both)
				mode=$1
				return
				;;
			-h | --help)
				usage
				exit 0
				;;
			*)
				usage >&2
				fail "unknown installation mode: $1"
				;;
		esac
	fi

	if [ ! -r /dev/tty ]; then
		fail "no terminal is available; use: sh -s -- docker"
	fi
	usage >/dev/tty
	printf '\nSelect a mode [docker]: ' >/dev/tty
	if ! IFS= read -r mode </dev/tty; then
		fail "cannot read installation mode"
	fi
	if [ -z "$mode" ]; then
		mode=docker
	fi
	case "$mode" in
		1 | docker) mode=docker ;;
		2 | host) mode=host ;;
		3 | both) mode=both ;;
		*) fail "unknown installation mode: $mode" ;;
	esac
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

as_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		sudo "$@"
	fi
}

prepare_root_access() {
	if [ "$(id -u)" -ne 0 ]; then
		require_command sudo
		printf 'hard installer: administrative access is required to install system packages.\n'
		sudo -v
	fi
}

distribution_matches() {
	case " $distribution_id $distribution_like " in
		*" $1 "*) return 0 ;;
		*) return 1 ;;
	esac
}

detect_distribution() {
	[ -r /etc/os-release ] || fail "cannot identify this Linux distribution"
	ID=
	ID_LIKE=
	VERSION_ID=
	# /etc/os-release is the system-provided shell-compatible distribution record.
	. /etc/os-release
	distribution_id=${ID:-}
	distribution_like=${ID_LIKE:-}
	distribution_version=${VERSION_ID:-}

	case "$distribution_id" in
		ubuntu | debian | linuxmint | pop | elementary)
			package_family=apt
			;;
		arch | cachyos | manjaro)
			package_family=pacman
			;;
		fedora)
			package_family=fedora
			;;
		rhel | rocky | almalinux | centos)
			package_family=rhel
			;;
		opensuse-leap | opensuse-tumbleweed | sles)
			package_family=zypper
			;;
		*)
			if distribution_matches debian; then
				package_family=apt
			elif distribution_matches arch; then
				package_family=pacman
			elif distribution_matches rhel; then
				package_family=rhel
			elif distribution_matches fedora; then
				package_family=fedora
			elif distribution_matches suse; then
				package_family=zypper
			else
				fail "unsupported Linux distribution: ${distribution_id:-unknown}"
			fi
			;;
	esac
}

install_host_dependencies() {
	printf 'hard installer: installing native host build dependencies.\n'
	case "$package_family" in
		apt)
			as_root apt-get update
			as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
				g++ libgtest-dev pkg-config
			;;
		pacman)
			as_root pacman -S --noconfirm --needed gcc gtest pkgconf
			;;
		fedora | rhel)
			if [ "$package_family" = rhel ]; then
				case "$distribution_id" in
					rocky | almalinux | centos)
						as_root dnf -y install epel-release
						;;
					rhel)
						rhel_major=${distribution_version%%.*}
						case "$rhel_major" in
							8 | 9 | 10) ;;
							*) fail "unsupported RHEL version: ${distribution_version:-unknown}" ;;
						esac
						as_root dnf -y install \
							"https://dl.fedoraproject.org/pub/epel/epel-release-latest-${rhel_major}.noarch.rpm"
						;;
				esac
			fi
			as_root dnf -y install gcc-c++ gtest-devel pkgconf-pkg-config
			;;
		zypper)
			as_root zypper --non-interactive install gcc-c++ gtest pkgconf-pkg-config
			;;
	esac
}

start_docker() {
	if command -v systemctl >/dev/null 2>&1; then
		as_root systemctl enable --now docker
	elif command -v service >/dev/null 2>&1; then
		as_root service docker start
	else
		fail "Docker is installed, but no supported service manager was found"
	fi
}

install_docker_dependencies() {
	docker_ready=0
	docker_user_ready=0
	if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
		docker_ready=1
		docker_user_ready=1
	elif command -v docker >/dev/null 2>&1 && as_root docker info >/dev/null 2>&1; then
		docker_ready=1
	fi
	if [ "$docker_ready" -eq 1 ]; then
		printf 'hard installer: Docker is already installed and running.\n'
	else
		printf 'hard installer: installing Docker.\n'
		case "$package_family" in
			apt)
				as_root apt-get update
				as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends docker.io
				;;
			pacman)
				as_root pacman -S --noconfirm --needed docker
				;;
			fedora)
				as_root dnf -y install moby-engine
				;;
			rhel)
				as_root dnf -y install dnf-plugins-core
				as_root dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
				as_root dnf -y install \
					docker-ce \
					docker-ce-cli \
					containerd.io \
					docker-buildx-plugin \
					docker-compose-plugin
				;;
			zypper)
				as_root zypper --non-interactive install docker
				;;
		esac

		start_docker
		if ! as_root docker info >/dev/null 2>&1; then
			fail "Docker was installed, but its daemon is not available"
		fi
		if docker info >/dev/null 2>&1; then
			docker_user_ready=1
		fi
	fi

	if [ "$(id -u)" -ne 0 ] && [ "$docker_user_ready" -eq 0 ]; then
		install_user=$(id -un)
		user_groups=$(id -nG "$install_user")
		case " $user_groups " in
			*" docker "*) docker_group_changed=1 ;;
			*)
				as_root usermod -aG docker "$install_user"
				docker_group_changed=1
				;;
		esac
	fi
}

download_release() {
	download_directory=$(mktemp -d "${TMPDIR:-/tmp}/hard-download.XXXXXX")
	printf 'hard installer: downloading the portable Linux archive.\n'
	curl --fail --location --retry 3 \
		--output "$download_directory/$archive_name" \
		"$release_base_url/$archive_name"
	curl --fail --location --retry 3 \
		--output "$download_directory/$checksum_name" \
		"$release_base_url/$checksum_name"
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
	printf '%s\n' "$default_target" > "$install_stage/runtime/default-target"
	chmod 0644 "$install_stage/runtime/default-target"

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

case "$(uname -s)" in
	Linux) ;;
	*) fail "the portable archive currently supports Linux only" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) ;;
	*) fail "the portable archive currently supports x86_64 only" ;;
esac
[ -n "${HOME:-}" ] || fail "HOME is not set"

choose_mode "$@"
require_command curl
require_command tar
require_command sha256sum
require_command mktemp
detect_distribution
download_release
prepare_root_access

docker_group_changed=0
case "$mode" in
	host)
		install_host_dependencies
		default_target=host
		;;
	docker)
		install_docker_dependencies
		default_target=linux.v1
		;;
	both)
		install_host_dependencies
		install_docker_dependencies
		default_target=linux.v1
		;;
esac

install_release

printf '\nhard was installed in %s.\n' "$HOME/.local"
printf 'Default execution target: %s\n' "$default_target"
case ":${PATH:-}:" in
	*":$HOME/.local/bin:"*) ;;
	*) printf 'Add %s to PATH before running hard.\n' "$HOME/.local/bin" ;;
esac
if [ "$docker_group_changed" -eq 1 ]; then
	printf 'Sign out and back in before using Docker without sudo; membership in the docker group grants root-level access.\n'
fi
if [ "$mode" = both ]; then
	printf 'Use --target=host for native builds; linux.v1 is the default.\n'
elif [ "$mode" = docker ]; then
	printf 'Use --target=host only if you install the native host build dependencies separately.\n'
fi
