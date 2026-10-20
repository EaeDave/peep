# Peep

Salas temporárias de compartilhamento de tela/câmera entre amigos, self-hosted
(LiveKit + Go + SvelteKit). Especificação: [`docs/PRD.md`](docs/PRD.md).

| Caminho | O quê |
|---|---|
| `apps/api` | room-api (Go): códigos de sala e tokens LiveKit |
| `apps/web` | frontend SvelteKit (site, Tauri e Chromium `--app`) |
| `apps/desktop` | cliente Windows (Tauri v2) |
| `apps/quickshell` | widget Linux (Quickshell) |
| `infra` | docker-compose do Coolify, config LiveKit, Traefik — ver [`infra/README.md`](infra/README.md) |

Deploy: push na `main` → Coolify builda `infra/docker-compose.yml`.

Testes da API: `cd apps/api && go test ./...`
