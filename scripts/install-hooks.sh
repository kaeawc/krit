#!/usr/bin/env bash
# Install the repository's git hooks (.githooks/pre-commit, .githooks/pre-push)
# for this clone and all its worktrees: `make hooks`.
#
# It writes a small wrapper into the shared hooks directory
# ($(git rev-parse --git-common-dir)/hooks) instead of setting
# core.hooksPath, which would silently disable every hook already there
# (husky, tool-installed hooks such as commit trailers). A hook that was
# already installed under the same name is kept as <name>.local and runs
# first. The wrapper runs the checked-out worktree's .githooks/<name>, so
# each worktree uses its own branch's hook, and branches without one skip.
#
# Usage: scripts/install-hooks.sh [--uninstall]
set -euo pipefail

marker='# krit-hook-wrapper'
hooks=(pre-commit pre-push)

if [ -n "$(git config --get core.hooksPath || true)" ]; then
	path=$(git config --get core.hooksPath)
	cat >&2 <<EOF
core.hooksPath is set to '$path', so git ignores the shared hooks directory.
Either unset it (git config --unset core.hooksPath) and rerun make hooks, or
call the repository hooks from your own:
  $path/pre-push:   "\$(git rev-parse --show-toplevel)/.githooks/pre-push" "\$@"
  $path/pre-commit: "\$(git rev-parse --show-toplevel)/.githooks/pre-commit" "\$@"
EOF
	exit 1
fi

dir=$(git rev-parse --git-common-dir)/hooks
mkdir -p "$dir"

is_wrapper() { [ -f "$1" ] && grep -qF "$marker" "$1"; }

if [ "${1:-}" = --uninstall ]; then
	for name in "${hooks[@]}"; do
		target="$dir/$name"
		is_wrapper "$target" || continue
		rm -f "$target"
		if [ -e "$target.local" ]; then
			mv "$target.local" "$target"
		fi
		echo "removed the $name wrapper"
	done
	exit 0
fi

for name in "${hooks[@]}"; do
	target="$dir/$name"
	if [ -e "$target" ] && ! is_wrapper "$target"; then
		if [ -e "$target.local" ]; then
			echo "$target and $target.local both exist; move one aside and rerun." >&2
			exit 1
		fi
		mv "$target" "$target.local"
		echo "kept the existing $name hook as $name.local (it runs first)"
	fi
	cat > "$target" <<'EOF'
#!/usr/bin/env bash
# krit-hook-wrapper: installed by `make hooks` (scripts/install-hooks.sh).
# Runs the hook that was here before (<name>.local), then the checked-out
# worktree's .githooks/<name>. Remove with scripts/install-hooks.sh --uninstall.
set -euo pipefail
name=$(basename "$0")
here=$(cd "$(dirname "$0")" && pwd)
repo_hook="$(git rev-parse --show-toplevel)/.githooks/$name"
input=""
if [ "$name" = pre-push ]; then
	# Both hooks read the pushed refs from stdin.
	input=$(cat)
fi
feed() {
	if [ -n "$input" ]; then printf '%s\n' "$input"; fi
}
if [ -x "$here/$name.local" ]; then
	feed | "$here/$name.local" "$@"
fi
if [ -x "$repo_hook" ]; then
	feed | "$repo_hook" "$@"
fi
EOF
	chmod +x "$target"
	echo "installed $name -> .githooks/$name"
done
echo "Bypass once with git push --no-verify; remove with scripts/install-hooks.sh --uninstall."
