#!/bin/sh

fail() {
	printf 'hard: %s\n' "$1" >&2
	exit 1
}

is_versioned_target() {
	case "$1" in
		linux64:* | windows64:*) ;;
		*) return 1 ;;
	esac

	tag=${1#*:}
	if [ "${#tag}" -gt 128 ]; then
		return 1
	fi
	case "$tag" in
		"" | [!A-Za-z0-9_]* | *[!A-Za-z0-9_.-]*) return 1 ;;
	esac
}

completion_cache_file=
completion_cached_targets=
completion_cache_is_fresh=0
resolve_completion_cache() {
	completion_cache_root=${XDG_CACHE_HOME:-}
	if [ -z "$completion_cache_root" ] && [ -n "${HOME:-}" ]; then
		completion_cache_root=$HOME/.cache
	fi
	if [ -n "$completion_cache_root" ]; then
		completion_cache_directory=$completion_cache_root/hard
		completion_cache_file=$completion_cache_directory/target-completion
	fi
}

read_completion_cache() {
	completion_cached_targets=
	completion_cache_is_fresh=0
	if [ -z "$completion_cache_file" ] ||
		[ ! -f "$completion_cache_file" ] ||
		[ -L "$completion_cache_file" ]; then
		return
	fi

	IFS=' ' read -r completion_cache_version completion_cache_expiry completion_cache_extra \
		< "$completion_cache_file" || return
	if [ "$completion_cache_version" != hard-target-completion-v1 ] ||
		[ -n "$completion_cache_extra" ]; then
		return
	fi
	case "$completion_cache_expiry" in
		"" | *[!0-9]*) return ;;
	esac
	if [ "${#completion_cache_expiry}" -gt 12 ]; then
		return
	fi

	completion_cached_targets=$(sed -n '2,$p' "$completion_cache_file" 2>/dev/null) || return
	completion_cache_now=$(date +%s 2>/dev/null) || return
	case "$completion_cache_now" in
		"" | *[!0-9]*) return ;;
	esac
	if [ "$completion_cache_now" -lt "$completion_cache_expiry" ]; then
		completion_cache_is_fresh=1
	fi
}

write_completion_cache() {
	completion_targets=$1
	if [ -z "$completion_cache_file" ]; then
		return
	fi
	completion_cache_now=$(date +%s 2>/dev/null) || return
	case "$completion_cache_now" in
		"" | *[!0-9]*) return ;;
	esac
	completion_cache_expiry=$((completion_cache_now + 300))

	(
		umask 077
		mkdir -p "$completion_cache_directory" 2>/dev/null || exit 0
		if [ -e "$completion_cache_file" ] || [ -L "$completion_cache_file" ]; then
			if [ ! -f "$completion_cache_file" ] || [ -L "$completion_cache_file" ]; then
				exit 0
			fi
		fi
		completion_cache_temporary=$completion_cache_file.$$
		{
			printf 'hard-target-completion-v1 %s\n' "$completion_cache_expiry"
			if [ -n "$completion_targets" ]; then
				printf '%s\n' "$completion_targets"
			fi
		} > "$completion_cache_temporary" || {
			rm -f "$completion_cache_temporary"
			exit 0
		}
		mv -f "$completion_cache_temporary" "$completion_cache_file" 2>/dev/null ||
			rm -f "$completion_cache_temporary"
	)
}

parse_registry_tags() {
	completion_repository=$1
	completion_response=$(printf '%s' "$2" | tr -d '[:space:]') || return 1
	case "$completion_response" in
		*"\"name\":\"hard-build/$completion_repository\""*'"tags":['*']}'*) ;;
		*) return 1 ;;
	esac

	completion_tags=$(
		printf '%s\n' "$completion_response" |
			sed -n 's/.*"tags":\[\([^]]*\)\].*/\1/p' |
			tr ',' '\n' |
			sed -n 's/^"\([A-Za-z0-9_][A-Za-z0-9_.-]*\)"$/\1/p'
	) || return 1
	for completion_tag in $completion_tags; do
		if [ "$completion_tag" != latest ] &&
			is_versioned_target "$completion_repository:$completion_tag"; then
			printf '%s:%s\n' "$completion_repository" "$completion_tag"
		fi
	done
}

fetch_completion_targets() {
	command -v curl >/dev/null 2>&1 || return 1
	completion_token_response=$(
		curl --fail --silent --location \
			--connect-timeout 2 --max-time 4 \
			'https://ghcr.io/token?service=ghcr.io&scope=repository%3Ahard-build%2Flinux64%3Apull&scope=repository%3Ahard-build%2Fwindows64%3Apull' \
			2>/dev/null
	) || return 1
	completion_token=$(printf '%s' "$completion_token_response" |
		tr -d '[:space:]' |
		sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
	if [ -z "$completion_token" ]; then
		return 1
	fi

	completion_linux_response=$(
		curl --fail --silent --location \
			--connect-timeout 2 --max-time 4 \
			--header "Authorization: Bearer $completion_token" \
			'https://ghcr.io/v2/hard-build/linux64/tags/list?n=10000' \
			2>/dev/null
	) || return 1
	completion_windows_response=$(
		curl --fail --silent --location \
			--connect-timeout 2 --max-time 4 \
			--header "Authorization: Bearer $completion_token" \
			'https://ghcr.io/v2/hard-build/windows64/tags/list?n=10000' \
			2>/dev/null
	) || return 1

	completion_linux_targets=$(parse_registry_tags linux64 "$completion_linux_response") || return 1
	completion_windows_targets=$(parse_registry_tags windows64 "$completion_windows_response") || return 1
	if [ -n "$completion_linux_targets" ]; then
		printf '%s\n' "$completion_linux_targets"
	fi
	if [ -n "$completion_windows_targets" ]; then
		printf '%s\n' "$completion_windows_targets"
	fi
}

print_completion_target() {
	case "$1" in
		"$completion_prefix"*) printf '%s\n' "$1" ;;
	esac
}

print_versioned_completion_targets() {
	completion_repository=$1
	while IFS= read -r completion_target; do
		case "$completion_target" in
			"$completion_repository":*)
				if is_versioned_target "$completion_target"; then
					print_completion_target "$completion_target"
				fi
				;;
		esac
	done <<EOF
$completion_dynamic_targets
EOF
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

	completion_dynamic_targets=
	completion_needs_registry=0
	case "$completion_prefix" in
		linux64:* | windows64:*) completion_needs_registry=1 ;;
		*)
			case linux64: in
				"$completion_prefix"*) completion_needs_registry=1 ;;
			esac
			case windows64: in
				"$completion_prefix"*) completion_needs_registry=1 ;;
			esac
			;;
	esac
	if [ "$completion_needs_registry" -eq 1 ]; then
		resolve_completion_cache
		read_completion_cache
		if [ "$completion_cache_is_fresh" -eq 1 ]; then
			completion_dynamic_targets=$completion_cached_targets
		elif completion_fetched_targets=$(fetch_completion_targets); then
			completion_dynamic_targets=$completion_fetched_targets
			write_completion_cache "$completion_dynamic_targets"
		else
			completion_dynamic_targets=$completion_cached_targets
		fi
	fi

	print_completion_target host
	print_completion_target linux64
	print_versioned_completion_targets linux64
	print_completion_target windows64
	print_versioned_completion_targets windows64
	print_completion_target docker://
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
		if is_versioned_target "$target"; then
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
