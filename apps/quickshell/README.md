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
com os URLs informados e `inviteSecret` vazio. Abra o Peep na barra e use a
engrenagem **Configurações do servidor** para editar **URL da API**, **URL do site**
e **Chave de acesso ao servidor**. Essa chave é fornecida por quem hospeda o
Peep; não é o código da sala. Não a passe no comando `curl` nem a publique no
servidor. O formato do arquivo continua compatível:

```json
{
  "apiUrl": "https://api.DOMINIO",
  "webUrl": "https://call.DOMINIO",
  "inviteSecret": "o-segredo-do-grupo"
}
```

As alterações de servidor só são gravadas em **Salvar**; **Cancelar** ou
**Escape** descarta o rascunho. A chave fica mascarada, com opção de mostrá-la.
As releases incluem `server-defaults.json` com os URLs públicos definidos no
build. A engrenagem usa esses valores apenas quando não há URL salvo; uma
implantação personalizada existente sempre tem prioridade. Esse arquivo
não contém a chave e pode estar ausente em um checkout de desenvolvimento.
Os URLs devem usar HTTPS; HTTP é permitido somente em `localhost`,
`127.0.0.1` ou `[::1]`, para desenvolvimento. Use URLs sem query ou fragmento.
O popup confirma o salvamento somente após a gravação terminar.

O popup salva o último nome usado no mesmo arquivo antes de abrir uma sala.
Chaves desconhecidas do JSON são preservadas. Antes de salvar, o arquivo é
relido: alterações externas nos dados do servidor interrompem o salvamento
até cancelar e reabrir as configurações; alterações em outros campos são
preservadas. JSON inválido ou falha de permissão não são sobrescritos.
Arquivos existentes mantêm as permissões na gravação atômica; um arquivo
ausente é criado com permissão 600. Links simbólicos de configuração não são
aceitos para gravação. A chave vai para o
Chromium no fragmento da URL (`#secret=…`), que não é enviado ao servidor.

### Verificação manual do popup

Com o plugin instalado (ou o symlink de desenvolvimento acima):

1. Abra a engrenagem, altere os três campos e pressione **Cancelar** ou
   **Escape**. Reabra e confira que os valores salvos não mudaram.
2. Tente salvar uma chave vazia, um URL inválido e HTTP em um domínio remoto:
   devem aparecer mensagens sem alterar o arquivo.
3. Salve URLs válidos e a chave; aguarde **Configurações salvas**. Confira
   `stat -c '%a' ~/.config/peep/config.json` (600), o nome e quaisquer campos
   adicionais já existentes no JSON, sem copiar a chave para logs.
4. Com as configurações abertas, altere externamente um URL do arquivo:
   **Salvar** deve recusar a edição antiga. Cancele e reabra para recarregar.
5. Entre com um código existente e crie uma sala; teste o atalho, Tab,
   **Escape**, foco visível e rolagem em uma tela pequena.

## Atalho e window rule (Hyprland)

Omarchy com config Lua (`~/.config/hypr/bindings.lua` / `hyprland.lua`):

```lua
o.bind("SUPER + ALT + P", "Peep", "omarchy-shell peep toggle")
-- opcional: viewer num workspace próprio, ou flutuando
o.window({ title = "^Peep$" }, { workspace = "9" })
-- o.window({ title = "^Peep$" }, { float = true })
```

Hyprland com config `.conf` clássica:

```ini
bind = SUPER ALT, P, exec, omarchy-shell peep toggle
windowrule = workspace 9, title:^Peep$
```

Em Chromium nativo Wayland, a classe de janelas `--app` pode ser gerada pelo
navegador (`chrome-<host>__r_<code>-Default`), mesmo com `--class=peep`.
Por isso os exemplos usam o título. Confira o valor local com `hyprctl clients`.
