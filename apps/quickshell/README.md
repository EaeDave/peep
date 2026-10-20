# Peep — Linux (Omarchy / Quickshell)

Plugin do shell do Omarchy: ícone na barra → popup com Nome/Código →
abre a sala num Chromium `--app` com perfil dedicado (captura via PipeWire).

## Pré-requisitos

`curl`, `tar`, `jq`, `wl-clipboard`, `pipewire`, `wireplumber`,
`xdg-desktop-portal-hyprland`, Omarchy (`omarchy` / `omarchy-shell`)
e um navegador Chromium (`chromium`, `google-chrome-stable` ou `brave`).

## Instalação

O instalador baixa `peep-omarchy-plugin.tar.gz` da release mais recente de
`EaeDave/peep`; não compila nem instala o cliente Tauri no Linux.
Substitua os dois domínios pelos endereços da sua implantação:

```sh
curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
  https://github.com/EaeDave/peep/releases/latest/download/install.sh \
  | bash -s -- --api-url https://api.DOMINIO --web-url https://call.DOMINIO
```

Para inspecionar antes de executar, baixe o mesmo `install.sh`, leia-o e
execute `bash install.sh --api-url https://api.DOMINIO --web-url https://call.DOMINIO`.
`bash install.sh --help` mostra o uso sem instalar.

O destino é `${XDG_CONFIG_HOME:-$HOME/.config}/omarchy/plugins/peep`.
Uma instalação existente é preservada: mova o diretório para um backup antes
de reinstalar. Se for um symlink de desenvolvimento, remova **apenas o link**
antes de instalar; o instalador recusa seguir esse link e não altera o checkout.
O plugin é habilitado com `omarchy-shell shell rescanPlugins` e
`omarchy plugin enable peep`.

(Desenvolvimento: `ln -s "$PWD/apps/quickshell" ~/.config/omarchy/plugins/peep` — salvar recarrega.)

Config em `${XDG_CONFIG_HOME:-$HOME/.config}/peep/config.json` (permissão 600).
O instalador preserva um arquivo existente; caso contrário, cria um arquivo
com os URLs informados e `inviteSecret` vazio. Edite o segredo **localmente**
depois da instalação; não o passe no comando `curl`, nem o publique no servidor:

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
