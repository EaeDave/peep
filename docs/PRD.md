# PRD — Peep

> App desktop minimalista para salas temporárias de compartilhamento de tela e câmera entre amigos, self-hosted numa VPS própria.

| Campo | Valor |
|---|---|
| Versão | 0.1 (MVP) |
| Data | 20/10/2026 |
| Público | Grupo fechado de amigos (≈ 2–8 pessoas por sala) |
| Plataformas | Windows 10/11 (tray app) · Linux Hyprland/Wayland com Quickshell · qualquer navegador Chromium (fallback) |
| Infra | VPS Oracle ARM64 (4 vCPU, 24 GB RAM) com Coolify |

---

## 1. Contexto e problema

Desde agosto de 2026 o compartilhamento de tela, o Go Live e as chamadas de vídeo do Discord estão suspensos no Brasil por ordem da ANPD. O grupo continua usando o Discord para voz e texto, mas perdeu o compartilhamento de tela e a câmera. O grupo não quer usar VPN.

**Solução:** um app próprio, leve e sem fricção, que faz uma coisa só: entrar numa sala por código e compartilhar tela e/ou câmera com boa qualidade. A voz continua no Discord.

## 2. Objetivos

1. Entrar numa sala em **≤ 3 cliques / ≤ 5 segundos** a partir da bandeja (Windows) ou da barra (Linux).
2. Compartilhamento de tela com qualidade de **1080p a 60fps** (ou 1440p a 30fps) e latência perceptível baixa (< 300 ms).
3. Visualização **grande e redimensionável**, com fullscreen. Nada de tela pequena quadradinha.
4. Zero manutenção de salas: salas são temporárias e morrem sozinhas.
5. Custo extra zero: tudo roda na VPS existente.

### Não-objetivos (MVP)

- Substituir a voz do Discord. O microfone existe, mas vem **desligado por padrão**.
- Chat de texto, gravação, contas de usuário, histórico, mobile, macOS.
- Escala pública ou multi-tenant.
- Controle remoto da tela de outra pessoa.

## 3. Personas e cenários

- **Host:** cria a sala, cola o código no Discord e compartilha a tela do jogo.
- **Viewer:** recebe o código, entra pela bandeja e assiste em tela cheia num segundo monitor.
- **Cenários típicos:** mostrar uma partida, ajudar a debugar código, assistir algo junto, mostrar a câmera rapidamente.

## 4. Arquitetura

### 4.1 Visão geral

```
                    ┌──────────── VPS Oracle (ARM64, Coolify) ──────────────┐
                    │                                                        │
 Windows (Tauri) ──►│  Traefik (Coolify) ─┬─► api.DOMINIO  → room-api (Go)   │
   tray + viewer    │                     ├─► call.DOMINIO → web (estático)  │
                    │                     └─► lk.DOMINIO   → LiveKit :7880   │
 Linux ────────────►│                                                        │
   Quickshell +     │  LiveKit SFU (network_mode: host)                      │
   Chromium --app   │    ├─ 7880/tcp  sinalização WS (via Traefik, WSS)      │
                    │    ├─ 7881/tcp  ICE/TCP fallback                       │
 Navegador ────────►│    ├─ 50000-60000/udp  mídia (direto, sem proxy)       │
   (fallback)       │    └─ TURN embutido: 3478/udp + 5349/tcp (TLS)         │
                    └────────────────────────────────────────────────────────┘
```

### 4.2 Princípios

- **Não reinventar WebRTC.** O LiveKit (SFU open source) cuida de mídia, NAT/TURN, simulcast e controle de banda.
- **Um único frontend web** concentra toda a lógica de chamada. Os clientes desktop são cascas finas.
- **Estado efêmero.** Sem banco no MVP. A sala vive no LiveKit e o mapeamento de código vive em memória com TTL.

### 4.3 Stack

| Camada | Tecnologia | Motivo |
|---|---|---|
| Mídia (SFU + TURN) | LiveKit Server (self-hosted, imagem ARM64) | Maduro, SDKs oficiais, simulcast, TURN embutido |
| API de salas/tokens | Go 1.23+ com `github.com/livekit/server-sdk-go/v2` | SDK oficial, binário pequeno, simples |
| Frontend | SvelteKit (`adapter-static`) + TypeScript + `livekit-client` | Leve, build estático reaproveitado no Tauri |
| Windows | Tauri v2 (Rust) + WebView2 | Tray nativo, instalador ~5–10 MB, WebView2 = Chromium |
| Linux | Widget QML no Quickshell + Chromium em modo `--app` | Integração nativa com a barra; Chromium captura bem no Wayland via PipeWire |
| Infra | Coolify (docker compose) | Já disponível |
| Banco | Nenhum no MVP | Postgres existente fica reservado para o futuro |

### 4.4 Estrutura do repositório (monorepo)

```
peep/
├── infra/
│   ├── docker-compose.yml        # livekit + room-api + web
│   ├── livekit.yaml              # config do LiveKit (sem segredos)
│   └── README.md                 # passo a passo de deploy e firewall
├── apps/
│   ├── api/                      # room-api (Go)
│   │   ├── cmd/server/main.go
│   │   ├── internal/rooms/       # geração de códigos, store em memória c/ TTL
│   │   ├── internal/token/       # emissão de JWT LiveKit
│   │   ├── internal/http/        # handlers, middleware (auth, rate limit, CORS)
│   │   └── Dockerfile            # multi-stage, imagem final distroless/arm64
│   ├── web/                      # SvelteKit
│   │   ├── src/routes/+page.svelte          # tela Mini (join)
│   │   ├── src/routes/r/[code]/+page.svelte # tela Viewer
│   │   ├── src/lib/livekit/                 # wrapper de conexão/publicação
│   │   ├── src/lib/platform.ts              # detecta Tauri / browser
│   │   └── Dockerfile                        # nginx/caddy servindo o build
│   ├── desktop/                  # Tauri v2 (Windows)
│   │   ├── src-tauri/src/main.rs
│   │   ├── src-tauri/tauri.conf.json
│   │   └── src-tauri/capabilities/
│   └── quickshell/               # widget QML para Linux
│       ├── PeepButton.qml
│       ├── PeepPopup.qml
│       └── peep-launch.sh
└── docs/
    └── PRD.md
```

## 5. Componentes e requisitos

### 5.1 LiveKit Server

| ID | Requisito |
|---|---|
| LK-1 | Rodar via docker compose no Coolify com `network_mode: host` |
| LK-2 | Sinalização exposta em `wss://lk.DOMINIO` através do Traefik (proxy para `localhost:7880`) |
| LK-3 | Mídia UDP em `50000-60000` direto no host; `use_external_ip: true` |
| LK-4 | TURN embutido habilitado: `udp_port: 3478` e `tls_port: 5349` com domínio `turn.DOMINIO` e certificado válido |
| LK-5 | `room.empty_timeout: 300` (sala morre 5 min após esvaziar) e `room.max_participants: 10` |
| LK-6 | `API_KEY` / `API_SECRET` via variáveis de ambiente do Coolify, nunca commitados |
| LK-7 | Codecs habilitados: VP8, VP9, H.264, AV1 |

**Firewall (obrigatório, em dois lugares):**
1. Security List / NSG da VCN na Oracle: liberar `7881/tcp`, `3478/udp`, `5349/tcp`, `50000-60000/udp` (80/443 já devem estar abertos).
2. `iptables` do host (a imagem Ubuntu da Oracle bloqueia tudo por padrão): adicionar as mesmas regras **antes** da regra `REJECT` e persistir com `netfilter-persistent save`.

> Documentar isso em `infra/README.md`. É a causa nº 1 de "conecta mas não chega vídeo".

### 5.2 room-api (Go)

**Responsabilidades:** criar salas com código curto, validar o código e emitir tokens LiveKit.

#### Endpoints

| Método | Rota | Corpo | Resposta |
|---|---|---|---|
| `GET` | `/healthz` | — | `200 ok` |
| `POST` | `/rooms` | `{ "name": "Rafa" }` | `201 { "code": "KJ7-29F", "expiresAt": "..." }` |
| `POST` | `/rooms/{code}/join` | `{ "name": "Rafa" }` | `200 { "token": "<jwt>", "url": "wss://lk.DOMINIO", "room": "KJ7-29F" }` |
| `GET` | `/rooms/{code}` | — | `200 { "exists": true, "participants": 3 }` ou `404` |

#### Regras

| ID | Requisito |
|---|---|
| API-1 | Código no formato `XXX-XXX`, alfabeto sem caracteres ambíguos (`ABCDEFGHJKMNPQRSTUVWXYZ23456789`). Normalizar a entrada (maiúsculas, aceitar sem hífen) |
| API-2 | Store em memória: `code → {createdAt, lastSeen}`. Expira em 12 h, ou quando o LiveKit reportar a sala vazia além do `empty_timeout` (consultar `ListRooms` periodicamente, a cada 60 s) |
| API-3 | Autenticação por **segredo de convite** compartilhado: header `Authorization: Bearer <INVITE_SECRET>` em todas as rotas exceto `/healthz`. O segredo fica configurado nos clientes |
| API-4 | Token LiveKit com TTL de 6 h, `identity = slug(name) + "-" + 4 chars aleatórios`, `name = name`, grants: `roomJoin`, `room = code`, `canPublish`, `canSubscribe`, `canPublishSources = [camera, microphone, screen_share, screen_share_audio]` |
| API-5 | O nome tem 1 a 24 caracteres. Rejeitar vazio e caracteres de controle |
| API-6 | Rate limit por IP: 30 req/min |
| API-7 | CORS liberado para `https://call.DOMINIO`, `tauri://localhost` e `http://tauri.localhost` |
| API-8 | Variáveis: `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `INVITE_SECRET`, `PUBLIC_LIVEKIT_WS_URL`, `PORT` |
| API-9 | Logs estruturados (`slog`). Nunca logar tokens ou segredos |
| API-10 | Imagem Docker multi-stage `linux/arm64`, rodando sem root |

### 5.3 Frontend web (SvelteKit)

Build estático único, usado em três lugares: no site `call.DOMINIO`, empacotado no Tauri e aberto pelo Chromium `--app` no Linux.

#### Tela Mini (`/`), cerca de 320 × 420 px

| ID | Requisito |
|---|---|
| WEB-1 | Campo **Nome** (persistido em `localStorage`, com try/catch) |
| WEB-2 | Campo **Código** com máscara `XXX-XXX` e colar do clipboard. Enter = entrar |
| WEB-3 | Botão **Criar sala**: chama `POST /rooms`, copia o código para o clipboard automaticamente e entra |
| WEB-4 | Ao entrar: no Tauri, chama o comando `open_viewer(code)` e esconde o popup; no navegador, navega para `/r/{code}` |
| WEB-5 | Erros claros: "Sala não existe", "Sem conexão com o servidor", "Segredo inválido" |
| WEB-6 | Tema escuro/claro seguindo o sistema. Visual minimalista, sem abas |

#### Tela Viewer (`/r/[code]`)

| ID | Requisito |
|---|---|
| VW-1 | Janela redimensionável. **Layout foco:** a tela compartilhada selecionada ocupa todo o espaço disponível (`object-fit: contain`) |
| VW-2 | Faixa lateral ou inferior recolhível com miniaturas (câmeras e outras telas). Clicar numa miniatura troca o foco |
| VW-3 | Ao surgir o primeiro compartilhamento de tela, ele vira o foco automaticamente |
| VW-4 | Fullscreen com duplo clique ou tecla `F`; `Esc` sai |
| VW-5 | Barra de controles (auto-oculta após 3 s sem mouse): **Tela**, **Câmera**, **Mic** (padrão off), **Copiar código**, **Sair** |
| VW-6 | Compartilhar tela usa `getDisplayMedia` com opção **"Áudio do sistema"** (Windows) |
| VW-7 | Seletor de qualidade ao compartilhar: **Jogo** (1080p60, `contentHint: 'motion'`) · **Texto/código** (1440p30, `contentHint: 'detail'`) · **Econômico** (720p30) |
| VW-8 | Publicação de tela com VP9 (fallback VP8/H.264), simulcast ligado e `degradationPreference` coerente com o modo escolhido |
| VW-9 | Câmera em 720p30 com simulcast |
| VW-10 | Lista de participantes com indicadores (🖥️ compartilhando, 📷 câmera, 🎙️ mic) |
| VW-11 | Indicador de qualidade de conexão por participante (`ConnectionQuality` do LiveKit) |
| VW-12 | Reconexão automática transparente. Mostrar "Reconectando…" |
| VW-13 | Volume por participante para o áudio de tela (o padrão do áudio de tela compartilhada é ligado) |
| VW-14 | Atalhos: `Ctrl+Shift+S` liga/desliga a tela, `Ctrl+Shift+C` liga/desliga a câmera |
| VW-15 | Acessível pelo navegador comum com o link `https://call.DOMINIO/r/{code}` (fallback universal) |

### 5.4 Cliente Windows — Tauri v2

| ID | Requisito |
|---|---|
| WIN-1 | Ícone na bandeja. **Clique esquerdo** mostra/esconde o popup Mini; **clique direito** abre o menu (Abrir, Iniciar com o Windows, Sair) |
| WIN-2 | Popup sem borda, `alwaysOnTop`, posicionado junto à bandeja (`tauri-plugin-positioner`, `TrayBottomCenter`). Esconde ao perder o foco |
| WIN-3 | Comando Rust `open_viewer(code)`: cria ou foca a janela `viewer` (1280 × 720, redimensionável, com borda), carregando `/r/{code}` |
| WIN-4 | Frontend **empacotado** no app (build estático do `apps/web`), com URL da API e segredo de convite definidos em configuração local na primeira execução (tela de "setup" simples) ou em build-time |
| WIN-5 | Permissões de mídia do WebView2 concedidas automaticamente para o próprio app (tratar `PermissionRequested` para câmera, microfone e captura de tela), sem prompts repetidos |
| WIN-6 | `tauri-plugin-single-instance` (abrir de novo foca o existente) e `tauri-plugin-autostart` (opcional, desligado por padrão) |
| WIN-7 | Instalador `.msi` / NSIS gerado pelo `tauri build`. Atualização automática fica fora do MVP |
| WIN-8 | Fechar o viewer = sair da sala; o app continua na bandeja |

> **Spike obrigatório (fase 0):** validar no WebView2 o `getDisplayMedia` (tela inteira e janela), o áudio do sistema e o 1080p60 estável. **Plano B:** Electron com o mesmo frontend (troca só a casca).

### 5.5 Cliente Linux — Quickshell + Chromium

| ID | Requisito |
|---|---|
| LNX-1 | Componente `PeepButton.qml`: ícone na barra existente do Quickshell. Clique abre o `PeepPopup.qml` |
| LNX-2 | O popup tem campos Nome e Código e botões **Entrar** e **Criar sala**, no estilo do tema atual do shell |
| LNX-3 | "Criar sala" faz `POST /rooms` via `XMLHttpRequest` no QML e copia o código (`wl-copy`) |
| LNX-4 | "Entrar" executa `peep-launch.sh <code> <nome>` via `Process`, que roda: `chromium --app="https://call.DOMINIO/r/<code>?name=<nome>" --class=peep --user-data-dir="$HOME/.local/share/peep/chromium" --ozone-platform-hint=auto` |
| LNX-5 | Perfil Chromium dedicado (`--user-data-dir`) para persistir permissões e nome sem poluir o navegador principal |
| LNX-6 | `IpcHandler` com alvo `peep` e função `toggle()`, para permitir um keybind no Hyprland: `bind = SUPER, T, exec, qs ipc call peep toggle` |
| LNX-7 | Documentar a window rule opcional do Hyprland para a classe `peep` (ex.: abrir num workspace específico ou flutuando) |
| LNX-8 | Pré-requisitos documentados: `pipewire`, `wireplumber`, `xdg-desktop-portal-hyprland`, `chromium`, `wl-clipboard` |
| LNX-9 | Config (URL da API e segredo) em `~/.config/peep/config.json`, lida pelo QML |

> O frontend aceita `?name=` na URL para pular a tela de nome quando vier do Quickshell.

## 6. Fluxos principais

### 6.1 Criar e compartilhar (Windows)
1. Clica no ícone da bandeja e o popup abre.
2. Clica em "Criar sala": o código é gerado e copiado.
3. O viewer abre e o popup some.
4. Cola o código no Discord.
5. Clica em "Tela", escolhe o modo **Jogo**, seleciona a tela no picker e começa a transmitir.

### 6.2 Entrar e assistir (Linux)
1. Clica no ícone na barra (ou `SUPER+T`).
2. Cola o código e dá Enter.
3. O Chromium `--app` abre o viewer, e a tela compartilhada já está em foco.
4. Duplo clique entra em fullscreen.

### 6.3 Ciclo de vida da sala
Criação (API) → primeiro join cria a sala no LiveKit → todos saem → após `empty_timeout` o LiveKit encerra a sala → a API expira o código no próximo ciclo de limpeza.

## 7. Requisitos não-funcionais

| ID | Requisito |
|---|---|
| NF-1 | Latência ponta a ponta da tela < 300 ms na mesma região |
| NF-2 | Suportar 8 participantes com 2 telas 1080p60 simultâneas sem saturar a VPS (uso de CPU < 60%) |
| NF-3 | Tempo do clique no tray até a mídia aparecendo < 5 s |
| NF-4 | Uso de RAM do app Windows ocioso na bandeja < 80 MB |
| NF-5 | Todo tráfego cifrado: HTTPS/WSS na sinalização, DTLS-SRTP na mídia (padrão WebRTC) |
| NF-6 | Nenhuma gravação ou armazenamento de mídia no servidor |
| NF-7 | Segredos apenas em variáveis de ambiente do Coolify ou na config local do cliente |

## 8. Deploy

1. DNS: `call.`, `api.`, `lk.` e `turn.` apontando para o IP da VPS.
2. Firewall Oracle (Security List + iptables), conforme a seção 5.1.
3. Coolify: um projeto com o `docker-compose.yml` do `infra/`:
   - `livekit`: `network_mode: host`, com config montada e segredos via env.
   - `room-api`: atrás do Traefik em `api.DOMINIO`.
   - `web`: estático atrás do Traefik em `call.DOMINIO`.
   - Rota Traefik `lk.DOMINIO` → `host.docker.internal:7880` (ou IP do host), com suporte a WebSocket.
4. Certificado TLS do TURN (`turn.DOMINIO`): reaproveitar o do Let's Encrypt via Traefik/certbot e montar no LiveKit.
5. Smoke test com o LiveKit Meet (`meet.livekit.io`, opção custom server) antes de qualquer cliente próprio.

## 9. Plano de entrega e critérios de aceite

### Fase 0: Infra e spike (1–2 dias)
- [ ] LiveKit no ar. Duas máquinas em redes diferentes (uma no 4G) conseguem vídeo via LiveKit Meet.
- [ ] TURN validado, forçando relay (`iceTransportPolicy: 'relay'` num teste).
- [ ] Spike Tauri: o WebView2 captura tela inteira e janela em 1080p60, com áudio do sistema.

### Fase 1: room-api (1 dia)
- [ ] Endpoints da seção 5.2 implementados, com testes unitários de geração e normalização de código e de emissão de token.
- [ ] Rejeita requisições sem `INVITE_SECRET` válido.
- [ ] Deploy no Coolify com `/healthz` verde.

### Fase 2: Frontend web (3–4 dias)
- [ ] Criar sala, entrar e compartilhar tela/câmera funcionando no Chrome comum.
- [ ] Layout foco, miniaturas, fullscreen e auto-oculta dos controles.
- [ ] Os três modos de qualidade aplicam resolução e fps corretos (verificar via `getStats`).
- [ ] Reconexão após derrubar o Wi-Fi por 5 s.

### Fase 3: Cliente Windows (2–3 dias)
- [ ] Tray com popup ancorado; o popup esconde ao perder o foco.
- [ ] O viewer abre em janela própria e grande; fechar o viewer sai da sala.
- [ ] Sem prompts de permissão repetidos.
- [ ] Instalador gerado e testado numa máquina limpa.

### Fase 4: Cliente Linux (1–2 dias)
- [ ] Botão na barra do Quickshell com popup funcional.
- [ ] `qs ipc call peep toggle` funcionando via keybind.
- [ ] Captura de tela no Hyprland via portal (picker do Hyprland aparece) e transmissão chegando no Windows.

### Fase 5: Polimento (contínuo)
- [ ] Atalhos globais, lembrar último código, indicador de quem está compartilhando no tooltip do tray.
- [ ] Ajuste fino de bitrate após uso real.

## 10. Riscos e mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| WebView2 com limitação de captura/áudio | Alto | Spike na fase 0; plano B Electron com o mesmo frontend |
| Portas bloqueadas na Oracle (Security List/iptables) | Alto | Checklist em `infra/README.md` e teste com LiveKit Meet antes de codar |
| Amigo atrás de NAT restritivo/CGNAT | Médio | TURN embutido (UDP 3478 + TLS 5349) |
| Upload doméstico baixo de quem transmite | Médio | Modo Econômico + simulcast + adaptive stream |
| Captura no Wayland quebrar após update do Hyprland/portal | Baixo | Usar Chromium (suporte PipeWire maduro); documentar os pacotes |
| Segredo de convite vazar | Baixo | Rotacionar via env no Coolify; salas efêmeras sem dados sensíveis |
| Abrir para público no futuro | — | Exige revisão regulatória (proteção de menores/ECA Digital, LGPD) antes de qualquer abertura |

## 11. Futuro (pós-MVP)

- Contas e convites individuais (aí sim usando o Postgres existente).
- Salas fixas/favoritas e notificação "fulano começou a transmitir".
- Auto-update do app Windows (`tauri-plugin-updater`).
- Pacote AUR para o lado Linux.
- Controle de qualidade por viewer (escolher a camada do simulcast manualmente).
- Anotações/desenho sobre a tela compartilhada.

## 12. Notas para implementação (Claude Code)

- Implementar **fase por fase**, na ordem da seção 9, validando os critérios de aceite antes de seguir.
- Placeholders a substituir: `DOMINIO`, `INVITE_SECRET`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`.
- Nenhum segredo no repositório: usar `.env.example` com valores fictícios.
- Todas as imagens Docker precisam buildar para `linux/arm64`.
- Preferir as libs oficiais do LiveKit (`livekit-client`, `server-sdk-go`) e consultar a documentação atual para nomes exatos de opções de publicação (presets de screenshare, `contentHint`, `degradationPreference`).
