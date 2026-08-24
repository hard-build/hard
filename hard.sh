#!/bin/sh

fail() {
	printf 'hard: %s\n' "$1" >&2
	exit 1
}

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

runtime_root=$HOME/.local/libexec/hard
if [ "$target_seen" -eq 0 ]; then
	target=host
	if [ -r "$runtime_root/default-target" ]; then
		IFS= read -r target < "$runtime_root/default-target" || fail "cannot read default target"
	fi
fi

case "$target" in
	host)
		PATH=$runtime_root/bin${PATH:+:$PATH}
		export PATH
		exec "$runtime_root/hard" "$@"
		;;
	linux.v1)
		image=ghcr.io/hard-build/hard:linux.v1
		;;
	*)
		fail "unknown target: $target"
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
	--pull=missing \
	--user "$user" \
	--mount "type=bind,source=$hard_root,target=/hard" \
	--mount "type=bind,source=$working_directory,target=$working_directory" \
	--workdir "$working_directory" \
	"$image" \
	"$@"
