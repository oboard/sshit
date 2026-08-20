#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release.sh <version>

Create and push an annotated release tag, triggering the GitHub Release workflow.

Arguments:
  version  Release version, with or without the leading "v" (for example: 0.1.0 or v0.1.0).

Examples:
  scripts/release.sh 0.1.0
  scripts/release.sh v0.1.0
EOF
}

if [[ $# -ne 1 || "$1" == "-h" || "$1" == "--help" ]]; then
  usage
  [[ $# -eq 1 ]] && [[ "$1" == "-h" || "$1" == "--help" ]] && exit 0
  exit 1
fi

version="$1"
tag="${version#v}"
tag="v${tag}"

if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "Invalid version: $version (expected SemVer, for example v0.1.0)" >&2
  exit 1
fi

repo_root="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "Run this command inside a Git repository." >&2
  exit 1
}
cd "$repo_root"

branch="$(git branch --show-current)"
if [[ "$branch" != "main" ]]; then
  echo "Releases must be created from the main branch (current: ${branch:-detached HEAD})." >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree has uncommitted changes. Commit or stash them before releasing." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "Tag already exists locally: $tag" >&2
  exit 1
fi

if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
  echo "Tag already exists on origin: $tag" >&2
  exit 1
fi

git fetch origin main --tags
local_head="$(git rev-parse HEAD)"
remote_head="$(git rev-parse origin/main)"
if [[ "$local_head" != "$remote_head" ]]; then
  echo "Local main does not match origin/main. Pull or push changes before releasing." >&2
  exit 1
fi

git tag --annotate "$tag" --message "Release $tag"
git push origin "$tag"

printf 'Pushed %s. The GitHub Release workflow is now building release assets.\n' "$tag"
