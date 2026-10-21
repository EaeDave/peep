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
profile="${XDG_DATA_HOME:-$HOME/.local/share}/peep/chromium"
# Screen-share audio via PipeWire (see peep-audio.sh); the page uses it when marked.
audio=
if command -v pw-dump >/dev/null && command -v pactl >/dev/null; then
  setsid bash "$(dirname "$0")/peep-audio.sh" "$profile" >/dev/null 2>&1 &
  audio="&audio=pipewire"
fi
# Secret/API travel in the #fragment: never sent to the server, stripped by the page.
url="${web%/}/r/$(enc "$code")?name=$(enc "$name")$audio#secret=$(enc "$secret")&api=$(enc "$api")"

exec "$browser" --app="$url" --class=peep --wayland-app-id=peep \
  --user-data-dir="$profile" \
  --ozone-platform-hint=auto >/dev/null 2>&1
