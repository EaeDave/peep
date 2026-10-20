#!/usr/bin/env bash
# Cuts a client release: bumps versions, commits, tags vX.Y.Z and pushes.
# The tag triggers .github/workflows/release.yml (Windows installers + Omarchy plugin).
# Usage: scripts/release.sh <X.Y.Z | patch | minor | major>
set -euo pipefail
cd "$(dirname "$0")/.."

current=$(jq -r .version apps/desktop/package.json)
IFS=. read -r MA MI PA <<<"$current"
case ${1:-} in
  patch) next="$MA.$MI.$((PA + 1))" ;;
  minor) next="$MA.$((MI + 1)).0" ;;
  major) next="$((MA + 1)).0.0" ;;
  [0-9]*.[0-9]*.[0-9]*) next=$1 ;;
  *) echo "usage: $0 <X.Y.Z|patch|minor|major>   (current: $current)" >&2; exit 1 ;;
esac
# MSI only accepts numeric X.Y.Z.
[[ $next =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "invalid version: $next" >&2; exit 1; }

[[ $(git branch --show-current) == main ]] || { echo "release from main only" >&2; exit 1; }
[[ -z $(git status --porcelain) ]] || { echo "working tree not clean" >&2; exit 1; }
git fetch -q origin main
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || { echo "main is not in sync with origin" >&2; exit 1; }
git rev-parse -q --verify "refs/tags/v$next" >/dev/null && { echo "tag v$next exists" >&2; exit 1; }

set_json() { jq --arg v "$next" '.version = $v' "$1" >"$1.tmp" && mv "$1.tmp" "$1"; }
set_json apps/desktop/package.json
set_json apps/quickshell/manifest.json
(cd apps/desktop && npm install --package-lock-only --silent)
sed -i "0,/^version = \".*\"/s//version = \"$next\"/" apps/desktop/src-tauri/Cargo.toml
# keep Cargo.lock's own entry in sync without touching dependencies
sed -i "/^name = \"peep\"$/{n;s/^version = \".*\"/version = \"$next\"/}" apps/desktop/src-tauri/Cargo.lock

git add apps/desktop apps/quickshell/manifest.json
git commit -qm "release: v$next"
git tag -a "v$next" -m "Peep v$next"
git push -q origin main "v$next"
echo "v$next pushed → https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/actions"
