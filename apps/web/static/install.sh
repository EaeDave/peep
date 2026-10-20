#!/usr/bin/env bash
# Usage: curl -fsSL https://YOUR-PEEP-HOST/install.sh | bash -s -- \
#   --api-url https://YOUR-API-HOST --web-url https://YOUR-PEEP-HOST
# Downloads the latest EaeDave/peep release; never reads secrets from stdin.
set -euo pipefail

fail() { printf 'peep: %s\n' "$*" >&2; exit 1; }
api_url= web_url= update=0 version=
while (($#)); do
  case $1 in
    --api-url|--web-url)
      (($# >= 2)) || fail "Missing value for $1"
      [[ $2 == https://* && $2 != *[$'\r\n\t ']* ]] || fail 'URLs must use HTTPS and contain no whitespace.'
      if [[ $1 == --api-url ]]; then api_url=$2; else web_url=$2; fi
      shift 2 ;;
    --update) update=1; shift ;;
    --version)
      (($# >= 2)) && [[ $2 =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'Use --version vX.Y.Z.'
      version=$2
      shift 2 ;;
    --help|-h)
      printf 'Usage: bash install.sh [--api-url https://HOST] [--web-url https://HOST] [--version vX.Y.Z] [--update]\nExisting config is preserved. Set inviteSecret locally after installation.\n--update replaces an existing plugin installation with the selected release.\n'
      exit 0 ;;
    *) fail "Unknown option: $1 (use --help)" ;;
  esac
done
[[ $(uname -s) == Linux ]] || fail 'This installer is for Linux Omarchy only.'
for cmd in curl tar jq mktemp mkdir mv rm chmod omarchy omarchy-shell wl-copy; do
  command -v "$cmd" >/dev/null || fail "Missing dependency: $cmd. Install Omarchy and the prerequisites in apps/quickshell/README.md."
done
browser=
for cmd in chromium google-chrome-stable google-chrome brave helium-browser; do
  if command -v "$cmd" >/dev/null; then browser=$cmd; break; fi
done
[[ -n $browser ]] || fail 'Install a Chromium-based browser first.'
config_root=${XDG_CONFIG_HOME:-$HOME/.config}
plugins=$config_root/omarchy/plugins
destination=$plugins/peep
[[ ! -L $destination ]] || fail "Existing plugin is a symlink: $destination. Remove only that symlink yourself to switch from a development checkout; its target was not changed."
if ((update)); then
  [[ -d $destination ]] || fail "Peep is not installed at $destination; run the installer without --update."
else
  [[ ! -e $destination ]] || fail "Existing installation at $destination. Move it aside before reinstalling; no files were changed."
fi
config=$config_root/peep/config.json
[[ ! -L $config && ! -L $config_root/peep ]] || fail 'Peep config is a symlink; manage it manually before installing.'
umask 077
[[ ! -L $plugins && ! -L $config_root/omarchy ]] || fail 'Omarchy plugin parent is a symlink; install manually to avoid changing its target.'
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
# Resolve the asset via the API: GitHub's latest/download redirect can cache
# the previous release even after a new one is published.
release=latest
[[ -z $version ]] || release=tags/$version
asset=$(curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
  "https://api.github.com/repos/EaeDave/peep/releases/$release" \
  | jq -er '[.assets[] | select(.name == "peep-omarchy-plugin.tar.gz")] | if length == 1 then .[0].browser_download_url else error("Missing or ambiguous plugin asset") end')
[[ $asset == https://github.com/EaeDave/peep/releases/download/*/peep-omarchy-plugin.tar.gz && $asset != *[$'\r\n\t ']* ]] \
  || fail 'Release returned an unexpected plugin URL.'
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
  "$asset" --output "$tmp/plugin.tar.gz"
# Reject paths outside peep/ and links before extracting the release archive.
tar -tzf "$tmp/plugin.tar.gz" > "$tmp/members"
while IFS= read -r member; do
  [[ $member == peep || $member == peep/* ]] || fail 'Unexpected archive root.'
  [[ /$member/ != */../* && /$member/ != */./* ]] || fail 'Unsafe archive path.'
done < "$tmp/members"
tar -tvzf "$tmp/plugin.tar.gz" > "$tmp/types"
while IFS= read -r member; do
  [[ $member == -* || $member == d* ]] || fail 'Archive contains links or special files.'
done < "$tmp/types"
tar -xzf "$tmp/plugin.tar.gz" --no-same-owner --no-same-permissions -C "$tmp"
for file in manifest.json PeepButton.qml PeepPopup.qml peep-launch.sh; do
  [[ -f $tmp/peep/$file ]] || fail "Release is missing $file."
done
jq -e '.id == "peep"' "$tmp/peep/manifest.json" >/dev/null || fail 'Invalid plugin manifest.'
# Public release defaults prefill a fresh client; explicit URLs always win.
if [[ -f $tmp/peep/server-defaults.json ]]; then
  api_url=${api_url:-$(jq -er '.apiUrl // ""' "$tmp/peep/server-defaults.json")}
  web_url=${web_url:-$(jq -er '.webUrl // ""' "$tmp/peep/server-defaults.json")}
fi
chmod +x "$tmp/peep/peep-launch.sh"
mkdir -p "$plugins" "$config_root/peep"
if ((update)); then
  # Stage next to the plugin (same filesystem), then swap with two renames.
  stage=$(mktemp -d "$plugins/.peep-update.XXXXXX")
  trap 'rm -rf -- "$tmp" "$stage"' EXIT
  mv -T "$tmp/peep" "$stage/new"
  mv -T "$destination" "$stage/old"
  if ! mv -T -n "$stage/new" "$destination" || [[ -e $stage/new ]]; then
    [[ -e $destination ]] || mv -T "$stage/old" "$destination"
    fail 'Could not replace the plugin; the previous version was kept.'
  fi
else
  # -T refuses a concurrently-created directory; -n never overwrites an existing path.
  mv -T -n "$tmp/peep" "$destination"
  [[ ! -e $tmp/peep ]] || fail 'Installation path appeared during download; nothing was overwritten.'
fi
if [[ ! -e $config ]]; then
  # noclobber also preserves a config created while the download was running.
  (set -o noclobber; jq -n --arg api "$api_url" --arg web "$web_url" \
    '{apiUrl: $api, webUrl: $web, inviteSecret: ""}' > "$config")
fi
if ((update)); then
  printf 'Peep updated to %s. Restart the Omarchy shell to load it.\n' "$(jq -r .version "$destination/manifest.json")"
  exit 0
fi
omarchy-shell shell rescanPlugins
omarchy plugin enable peep
printf 'Peep installed. Open its gear to configure the server access key. Existing configuration was preserved.\n'
