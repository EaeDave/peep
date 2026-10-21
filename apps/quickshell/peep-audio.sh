#!/usr/bin/env bash
# Screen-share audio for Omarchy: a PipeWire virtual source ("Peep: áudio da
# tela") fed with every app's output EXCEPT Peep's own Chromium, so sharing
# carries game/video sound without echoing the call. Your speakers are not
# touched: links are added alongside the existing ones.
# Usage: peep-audio.sh <peep chromium --user-data-dir>
# Started by peep-launch.sh; one instance per session; exits with the browser.
set -uo pipefail

profile=${1:?usage: peep-audio.sh <profile dir>}
sink=peep_screen_audio
for cmd in pactl pw-dump pw-link jq flock pgrep; do
  command -v "$cmd" >/dev/null || exit 0
done

exec 9>"${XDG_RUNTIME_DIR:-/tmp}/peep-audio.lock"
flock -n 9 || exit 0

browser=
for _ in $(seq 40); do
  browser=$(pgrep -o -f -- "--user-data-dir=$profile") && break
  sleep 0.5
done
[[ -n $browser ]] || exit 0

# Drop a source left behind by a crashed run, then create ours.
pactl list short modules | awk -v s="sink_name=$sink" 'index($0, s) {print $1}' |
  xargs -r -n1 pactl unload-module
module=$(pactl load-module module-null-sink "sink_name=$sink" \
  media.class=Audio/Source/Virtual channel_map=front-left,front-right \
  'device.description="Peep: audio da tela"') || exit 1  # pactl args are ASCII-only
trap 'pactl unload-module "$module" 2>/dev/null' EXIT

# PIDs of the Peep browser and all its children (renderers, audio service).
peep_pids() {
  ps -eo pid=,ppid= | awk -v root="$browser" '
    { parent[$1] = $2 }
    END {
      for (p in parent) {
        q = p
        while (q in parent && q != root && q > 1) q = parent[q]
        if (q == root) print p
      }
    }' | jq -R . | jq -sc .
}

# Output ports of app streams (minus Peep) -> our input ports, by channel
# (mono/surround go to both). Only links that do not exist yet.
missing_links='
  (map(select(.type == "PipeWire:Interface:Node"))) as $nodes
  | ($nodes | map(select(.info.props["node.name"] == $sink)) | .[0].id) as $target
  | ($nodes | map(select(.info.props["media.class"] == "Stream/Output/Audio"
      and ((.info.props["application.process.id"] | tostring) as $p | $skip | index($p) | not))) | map(.id)) as $streams
  | (map(select(.type == "PipeWire:Interface:Port"))) as $ports
  | ($ports | map(select(.info.props["node.id"] == $target and .info.props["port.direction"] == "in"))) as $ins
  | (map(select(.type == "PipeWire:Interface:Link") | "\(.info["output-port-id"])-\(.info["input-port-id"])")) as $links
  | $ports[]
  | select(.info.props["port.direction"] == "out" and (.info.props["node.id"] as $n | $streams | index($n)))
  | . as $out
  | ($out.info.props["audio.channel"]) as $ch
  | $ins[]
  | select(.info.props["audio.channel"] == $ch or ($ch != "FL" and $ch != "FR"))
  | select("\($out.id)-\(.id)" as $k | $links | index($k) | not)
  | "\($out.id) \(.id)"'

# pgrep ignores zombies and follows a restarted Peep browser.
while browser=$(pgrep -o -f -- "--user-data-dir=$profile"); do
  pw-dump 2>/dev/null | jq -r --arg sink "$sink" --argjson skip "$(peep_pids)" "$missing_links" |
    while read -r out in; do pw-link "$out" "$in" 2>/dev/null; done
  sleep 1
done
