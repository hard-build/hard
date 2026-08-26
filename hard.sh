#!/bin/sh

fail() {
	printf 'hard: %s\n' "$1" >&2
	exit 1
}

is_numeric_version() {
	version=$1
	case "$version" in
		*.*)
			version_major=${version%%.*}
			version_minor=${version#*.}
			;;
		*) return 1 ;;
	esac
	if [ -z "$version_major" ] || [ -z "$version_minor" ]; then
		return 1
	fi
	case "$version_minor" in
		*.*) return 1 ;;
	esac
	case "$version_major$version_minor" in
		*[!0-9]*) return 1 ;;
	esac
}

is_versioned_linux64_target() {
	case "$1" in
		linux64:v*-ubuntu.* | linux64:v*-alpine.*-static) ;;
		*) return 1 ;;
	esac
	linux64_version=${1#linux64:v}
	hard_version=${linux64_version%%-*}
	platform=${linux64_version#*-}
	case "$platform" in
		ubuntu.*)
			platform_version=${platform#ubuntu.}
			;;
		alpine.*-static)
			platform_version=${platform#alpine.}
			version_without_static=${platform_version%-static}
			if [ "$version_without_static" = "$platform_version" ]; then
				return 1
			fi
			platform_version=$version_without_static
			;;
		*) return 1 ;;
	esac
	is_numeric_version "$hard_version" && is_numeric_version "$platform_version"
}

is_versioned_windows64_target() {
	case "$1" in
		windows64:v*-llvm-mingw.*-ucrt) ;;
		*) return 1 ;;
	esac
	windows64_version=${1#windows64:v}
	hard_version=${windows64_version%%-*}
	toolchain=${windows64_version#*-}
	llvm_mingw_version=${toolchain#llvm-mingw.}
	llvm_mingw_version=${llvm_mingw_version%-ucrt}
	case "$llvm_mingw_version" in
		[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]) ;;
		*) return 1 ;;
	esac
	is_numeric_version "$hard_version"
}

complete_target() {
	completion_enabled=1
	completion_match=0
	completion_prefix=
	completion_previous=
	for completion_argument in "$@"; do
		if [ "$completion_enabled" -eq 0 ]; then
			completion_match=0
			completion_previous=$completion_argument
			continue
		fi
		case "$completion_argument" in
			--)
				completion_enabled=0
				completion_match=0
				;;
			--target=*)
				completion_match=1
				completion_prefix=${completion_argument#--target=}
				;;
			*)
				if [ "$completion_previous" = --target ]; then
					completion_match=1
					completion_prefix=$completion_argument
				else
					completion_match=0
				fi
				;;
		esac
		completion_previous=$completion_argument
	done
	if [ "$completion_match" -eq 0 ]; then
		return 1
	fi

	for completion_target in \
		host \
		linux64 \
		linux64:v3.0-ubuntu.22.04 \
		linux64:v3.0-alpine.3.22-static \
		windows64 \
		windows64:v4.0-llvm-mingw.20260616-ucrt \
		docker://; do
		case "$completion_target" in
			"$completion_prefix"*) printf '%s\n' "$completion_target" ;;
		esac
	done
	printf ':4\n'
}

installation_root=
resolve_installation_root() {
	if [ -n "$installation_root" ]; then
		return
	fi

	wrapper_path=$0
	case "$wrapper_path" in
		*/*) ;;
		*)
			wrapper_path=$(command -v "$wrapper_path") || fail "cannot determine wrapper path"
			;;
	esac
	wrapper_directory=$(CDPATH= cd -P "$(dirname "$wrapper_path")" 2>/dev/null && pwd) ||
		fail "cannot determine wrapper directory"
	installation_root=$(CDPATH= cd -P "$wrapper_directory/.." 2>/dev/null && pwd) ||
		fail "cannot determine installation root"
}

case "${1:-}" in
	__complete | __completeNoDesc)
		completion_request=$1
		shift
		if complete_target "$@"; then
			exit 0
		fi
		set -- "$completion_request" "$@"
		resolve_installation_root
		runtime_root=$installation_root/libexec/hard
		PATH=$runtime_root/bin${PATH:+:$PATH}
		export PATH
		exec "$runtime_root/hard" "$@"
		;;
esac

target=
target_seen=0
parse_target=1
remaining=$#
while [ "$remaining" -gt 0 ]; do
	argument=$1
	shift
	remaining=$((remaining - 1))

	if [ "$parse_target" -eq 0 ]; then
		set -- "$@" "$argument"
		continue
	fi

	case "$argument" in
		--)
			parse_target=0
			set -- "$@" "$argument"
			;;
		--target)
			if [ "$target_seen" -eq 1 ]; then
				fail "--target may only be specified once"
			fi
			if [ "$remaining" -eq 0 ]; then
				fail "--target requires a value"
			fi
			target=$1
			shift
			remaining=$((remaining - 1))
			if [ -z "$target" ]; then
				fail "--target requires a value"
			fi
			target_seen=1
			;;
		--target=*)
			if [ "$target_seen" -eq 1 ]; then
				fail "--target may only be specified once"
			fi
			target=${argument#--target=}
			if [ -z "$target" ]; then
				fail "--target requires a value"
			fi
			target_seen=1
			;;
		*)
			set -- "$@" "$argument"
			;;
	esac
done

if [ "$target_seen" -eq 0 ]; then
	target=host
	resolve_installation_root
	default_target=$installation_root/libexec/hard/default-target
	if [ -r "$default_target" ]; then
		IFS= read -r target < "$default_target" || fail "cannot read default target"
	fi
fi

case "$target" in
	host)
		resolve_installation_root
		runtime_root=$installation_root/libexec/hard
		PATH=$runtime_root/bin${PATH:+:$PATH}
		export PATH
		exec "$runtime_root/hard" "$@"
		;;
	linux64)
		image=ghcr.io/hard-build/linux64:latest
		pull=always
		;;
	windows64)
		image=ghcr.io/hard-build/windows64:latest
		pull=always
		;;
	docker://*)
		image=${target#docker://}
		case "$image" in
		"" | -*) fail "invalid Docker image target: $target" ;;
		esac
		pull=missing
		;;
	*)
		if is_versioned_linux64_target "$target" ||
			is_versioned_windows64_target "$target"; then
			image=ghcr.io/hard-build/$target
			pull=missing
		else
			fail "unknown target: $target"
		fi
		;;
esac

working_directory=$(pwd -P) || fail "cannot determine working directory"
if [ -n "${HARD_ROOT:-}" ]; then
	hard_root=$HARD_ROOT
elif [ -n "${HOME:-}" ]; then
	hard_root=$HOME/.local/share/hard
else
	fail "HOME is not set"
fi
case "$hard_root" in
	/*) ;;
	*) hard_root=$working_directory/$hard_root ;;
esac

user=$(id -u):$(id -g) || fail "cannot determine user identity"
exec docker run \
	--rm \
	--interactive \
	--pull="$pull" \
	--user "$user" \
	--mount "type=bind,source=$hard_root,target=/hard" \
	--mount "type=bind,source=$working_directory,target=$working_directory" \
	--workdir "$working_directory" \
	"$image" \
	"$@"
