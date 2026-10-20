# Peep — Linux (Omarchy / Quickshell)

Plugin do shell do Omarchy: ícone na barra → popup com Nome/Código →
abre a sala num Chromium `--app` com perfil dedicado (captura via PipeWire).

## Pré-requisitos

`pipewire`, `wireplumber`, `xdg-desktop-portal-hyprland`, `jq`, `wl-clipboard`
e um navegador Chromium (`chromium`, `google-chrome-stable` ou `brave`).

## Instalação

```sh
mkdir -p ~/.config/omarchy/plugins
tar -xzf peep-omarchy-plugin.tar.gz -C ~/.config/omarchy/plugins   # cria …/plugins/peep
omarchy-shell shell rescanPlugins
omarchy plugin enable peep
```

(Desenvolvimento: `ln -s "$PWD/apps/quickshell" ~/.config/omarchy/plugins/peep` — salvar recarrega.)

Config em `~/.config/peep/config.json` (permissão 600):

```json
{
  "apiUrl": "https://api.DOMINIO",
  "webUrl": "https://call.DOMINIO",
  "inviteSecret": "o-segredo-do-grupo"
}
```

O popup salva o último nome usado no mesmo arquivo. O segredo vai para o
Chromium no fragmento da URL (`#secret=…`), que não é enviado ao servidor.

## Atalho e window rule (Hyprland)

Omarchy com config Lua (`~/.config/hypr/bindings.lua` / `hyprland.lua`):

```lua
o.bind("SUPER + ALT + P", "Peep", "omarchy-shell peep toggle")
-- opcional: viewer num workspace próprio, ou flutuando
o.window("^peep$", { workspace = "9" })
-- o.window("^peep$", { float = true })
```

Hyprland com config `.conf` clássica:

```ini
bind = SUPER ALT, P, exec, omarchy-shell peep toggle
windowrule = workspace 9, class:^peep$
```
