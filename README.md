# Peep

Salas temporárias para compartilhar tela e câmera entre amigos, self-hosted.
Entra por um código curto (`KJ7-29F`), compartilha em até 1080p60, e a sala
some sozinha quando todo mundo sai. A voz continua no Discord.

Stack: [LiveKit](https://livekit.io) (SFU + TURN) · room-api em Go ·
frontend SvelteKit · cliente Windows em Tauri v2 · plugin Linux para o
shell do Omarchy (Quickshell) + Chromium `--app`.
Especificação completa: [`docs/PRD.md`](docs/PRD.md).

| Caminho | O quê |
|---|---|
| [`apps/api`](apps/api) | room-api (Go): códigos de sala e tokens LiveKit |
| [`apps/web`](apps/web) | frontend SvelteKit (site, embutido no Tauri, Chromium `--app`) |
| [`apps/desktop`](apps/desktop) | cliente Windows (Tauri v2, bandeja) |
| [`apps/quickshell`](apps/quickshell) | plugin Linux (Omarchy/Quickshell) — ver README |
| [`infra`](infra) | docker-compose para Coolify, LiveKit, Traefik — ver [`infra/README.md`](infra/README.md) |

## Segredos

Nada sensível fica no repositório. Servidor: variáveis de ambiente
(`infra/.env.example`). A **chave de acesso ao servidor** é fornecida por quem
hospeda o Peep; não é o código de uma sala. A engrenagem permite alterar a chave
e a URL da API no Windows/web, além da URL do site no Omarchy, sem reinstalar.
Somente **Salvar** aplica as alterações; **Cancelar** preserva a configuração.
Os nomes técnicos `INVITE_SECRET`, `peep.secret` e `inviteSecret` continuam
compatíveis com instalações existentes.

## Deploy e releases

- **Servidor:** push na `main` → Coolify builda `infra/docker-compose.yml`
  (só quando muda `apps/api`, `apps/web` ou `infra`).
- **Clientes:** `scripts/release.sh patch|minor|major|X.Y.Z` cria a tag `vX.Y.Z`;
  o GitHub Actions gera os instaladores Windows (`.msi`/`.exe`) no
  `windows-latest` e anexa o plugin Linux à release. Commits comuns não geram build.
  A URL pública da API embutida no app vem da variável de repositório `PEEP_API_URL`.
  O plugin recebe também `PEEP_WEB_URL`. As URLs públicas vêm preenchidas por
  padrão; uma configuração personalizada salva no dispositivo tem prioridade.

## Instalação dos clientes

A página inicial apresenta o projeto e mostra o comando para o seu sistema.
`/join` abre a tela de criar/entrar em uma sala; links `/r/{code}` continuam diretos.

Na sala, o cabeçalho reúne código, conexão e configurações; os controles de tela,
câmera e microfone ficam separados das miniaturas. O painel de participantes
permite ajustar o áudio de cada tela compartilhada. Em janelas pequenas, ele
abre sobre o vídeo sem empurrar os controles. Alterar o servidor durante uma
sessão só afeta a próxima entrada; não derruba a conexão atual.

- **Linux / Omarchy:** o comando `curl` instala o plugin Quickshell da última
  release. A URL do servidor vem da página, e o segredo é configurado localmente.
- **Windows:** o comando PowerShell baixa o instalador x64 da última release.
  Releases sem assinatura digital exigem confirmação explícita; nenhuma política
  de segurança do Windows é desativada.
- Prefere inspecionar antes? `install.sh` e `install.ps1` estão disponíveis no
  site e como anexos da release. A instalação não apaga configurações existentes.
  Os instaladores resolvem a release pela API do GitHub e usam o endereço
  versionado do artefato, evitando redirecionamentos `latest/download` em cache.

### Atualizações

Os dois clientes mostram a versão atual no rodapé do popup e procuram uma nova
release ao iniciar e a cada 5 horas. O ícone de recarregar força a busca. Quando
há versão nova, o rodapé oferece **Atualizar**; nada é instalado sem esse clique.

- **Windows:** usa o updater do Tauri. A release publica `latest.json` e o setup
  NSIS assinado com a chave do updater (secret `TAURI_SIGNING_PRIVATE_KEY`; a
  chave pública fica em `tauri.conf.json`). Pacotes sem assinatura válida são
  recusados. Não atualiza com uma sala aberta.
- **Omarchy:** baixa o `install.sh` da release nova e roda
  `install.sh --update --version vX.Y.Z`, que troca o plugin preservando a
  configuração e depois reinicia a barra. Instalações em modo desenvolvimento
  (link simbólico) não são substituídas.

Guarde a chave privada do updater fora do repositório: sem ela, versões futuras
não conseguem atualizar os apps Windows já instalados.

## Desenvolvimento

```sh
cd apps/api && go test ./...                 # API
cd apps/web && npm ci && npm run check       # frontend
cd apps/web && VITE_API_URL=http://localhost:8080 npm run dev
```

### Skills de interface

As skills ficam versionadas no próprio repositório em `.agents/skills/`:

- [`frontend-design`](.agents/skills/frontend-design/SKILL.md), da Anthropic:
  direção visual, tipografia e hierarquia.
- [`web-design-guidelines`](.agents/skills/web-design-guidelines/SKILL.md), da Vercel:
  revisão de acessibilidade, foco, formulários e interação.

`skills-lock.json` registra as origens e os hashes instalados. O frontend
continua em Svelte, com CSS escopado e ícones SVG compartilhados.

Licença: [MIT](LICENSE).
