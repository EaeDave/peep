# Deploy — Peep

Stack: `livekit` (host network) + `room-api` + `web` em um único recurso
**Docker Compose** do Coolify, com auto-deploy a cada push na `main`.
Os hostnames abaixo são exemplos; use os seus (são configurados por env).

| Host | Destino |
|---|---|
| `call.DOMINIO` | web (serviço `web`, porta 8080) |
| `api.DOMINIO` | room-api (serviço `room-api`, porta 8080) |
| `lk.DOMINIO` | LiveKit 7880 (Traefik dynamic config) |
| `turn.DOMINIO` | TURN/TLS via Traefik :443 → 5349 · TURN/UDP 3478 direto |

## 1. DNS

Registros A para o IP da VPS. Com Cloudflare, `call`, `api` e `lk` podem ficar
atrás do proxy, mas **`turn` precisa ser DNS-only (nuvem cinza)**: TURN (UDP
3478 e TLS com SNI) não passa pelo proxy da Cloudflare.

## 2. Firewall (dois lugares — causa nº 1 de "conecta mas não chega vídeo")

### 2a. Oracle VCN (Security List / NSG), ingress `0.0.0.0/0`

| Porta | Proto | Uso |
|---|---|---|
| 7881 | TCP | ICE/TCP fallback |
| 3478 | UDP | TURN/UDP |
| 50000-60000 | UDP | mídia WebRTC |

80/443 já estão abertos. 5349 **não** precisa ser público: o Traefik termina
TLS em :443 (SNI `turn.DOMINIO`) e encaminha para 5349 local.

### 2b. iptables do host (a imagem Ubuntu da Oracle rejeita tudo por padrão)

O LiveKit roda em `network_mode: host`, então o tráfego cai na chain `INPUT`.
Regras inseridas **antes** do `REJECT` e persistidas:

```sh
sudo iptables -I INPUT 1 -p tcp --dport 7881 -m comment --comment peep -j ACCEPT
sudo iptables -I INPUT 1 -p udp --dport 3478 -m comment --comment peep -j ACCEPT
sudo iptables -I INPUT 1 -p udp --dport 50000:60000 -m comment --comment peep -j ACCEPT
# containers (Traefik, room-api) → LiveKit no host: sinalização e TURN/TLS
sudo iptables -I INPUT 1 -i br-+ -p tcp -m multiport --dports 7880,5349 -m comment --comment peep -j ACCEPT
sudo iptables -I INPUT 1 -i docker0 -p tcp -m multiport --dports 7880,5349 -m comment --comment peep -j ACCEPT
sudo netfilter-persistent save
```

7880 e 5349 ficam acessíveis só pelas bridges do Docker, nunca pela internet.

## 3. Traefik (rotas para o LiveKit em host network)

Copie `traefik-peep.yml` (trocando `DOMINIO`) para
`/data/coolify/proxy/dynamic/peep.yaml` (ou Coolify → Servers → Proxy →
Dynamic Configurations). O Traefik recarrega sozinho.

## 4. Coolify

- Recurso: **GitHub App / Public Repository** → este repo, branch `main`.
- Build pack: **Docker Compose**, Base Directory `/infra`, Docker Compose Location `/docker-compose.yml`.
- Domínios: `room-api` → `https://api.DOMINIO:8080`; `web` → `https://call.DOMINIO:8080`.
- Variáveis: ver `.env.example`. Gere as chaves com
  `docker run --rm livekit/livekit-server generate-keys`.
- Auto-deploy: ligado (push na `main` dispara o deploy pelo GitHub App).

## 5. Smoke test

```sh
curl https://api.DOMINIO/healthz                     # ok
curl -XPOST -H "Authorization: Bearer $INVITE_SECRET" \
  -d '{"name":"teste"}' https://api.DOMINIO/rooms     # {"code":"…"}
```

Depois, LiveKit Meet (`meet.livekit.io` → custom server `wss://lk.DOMINIO`
com um token de `POST /rooms/{code}/join`) em duas redes diferentes (uma no 4G).
Para validar TURN, force relay (`iceTransportPolicy: 'relay'`).
