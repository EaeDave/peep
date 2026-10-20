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
(`infra/.env.example`). Clientes: o segredo de convite é digitado na primeira
execução (Windows/web) ou lido de `~/.config/peep/config.json` (Linux).

## Deploy e releases

- **Servidor:** push na `main` → Coolify builda `infra/docker-compose.yml`
  (só quando muda `apps/api`, `apps/web` ou `infra`).
- **Clientes:** `scripts/release.sh patch|minor|major|X.Y.Z` cria a tag `vX.Y.Z`;
  o GitHub Actions gera os instaladores Windows (`.msi`/`.exe`) no
  `windows-latest` e anexa o plugin Linux à release. Commits comuns não geram build.
  A URL pública da API embutida no app vem da variável de repositório `PEEP_API_URL`.

## Desenvolvimento

```sh
cd apps/api && go test ./...                 # API
cd apps/web && npm ci && npm run check       # frontend
cd apps/web && VITE_API_URL=http://localhost:8080 npm run dev
```

Licença: [MIT](LICENSE).
