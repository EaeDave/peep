#!/usr/bin/env bash
# Opens a Peep room in a dedicated Chromium --app window.
# Usage: peep-launch.sh <CODE> [name]
# Reads webUrl / apiUrl / inviteSecret from ~/.config/peep/config.json.
set -euo pipefail

code=${1:?usage: peep-launch.sh <CODE> [name]}
name=${2:-}
cfg=${XDG_CONFIG_HOME:-$HOME/.config}/peep/config.json

[[ -r $cfg ]] || { echo "peep: missing $cfg" >&2; exit 1; }
read_cfg() { jq -r --arg k "$1" '.[$k] // empty' "$cfg"; }
web=$(read_cfg webUrl)
api=$(read_cfg apiUrl)
secret=$(read_cfg inviteSecret)
[[ -n $web ]] || { echo "peep: webUrl not set in $cfg" >&2; exit 1; }

for b in chromium google-chrome-stable google-chrome brave helium-browser; do
  command -v "$b" >/dev/null && { browser=$b; break; }
done
[[ -n ${browser:-} ]] || { echo "peep: no Chromium-based browser found" >&2; exit 1; }

enc() { jq -rn --arg v "$1" '$v|@uri'; }
# Secret/API travel in the #fragment: never sent to the server, stripped by the page.
url="${web%/}/r/$(enc "$code")?name=$(enc "$name")#secret=$(enc "$secret")&api=$(enc "$api")"

exec "$browser" --app="$url" --class=peep \
  --user-data-dir="${XDG_DATA_HOME:-$HOME/.local/share}/peep/chromium" \
  --ozone-platform-hint=auto >/dev/null 2>&1
