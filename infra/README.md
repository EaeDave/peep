# Deploy — Peep

Dois recursos **Docker Compose** no Coolify, com auto-deploy a cada push na `main`:
`room-api` + `web` (`/infra`) e `livekit` em host network (`/infra/livekit`).
Separados para que deploys da API/site nunca reiniciem o servidor de mídia.
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

### 2c. Buffers UDP do kernel

O padrão do Linux (212 KB) descarta pacotes de vídeo em rajadas
(`RcvbufErrors` em `/proc/net/snmp`). O LiveKit usa até `rmem_max`/`wmem_max`:

```sh
printf '%s\n' 'net.core.rmem_max = 8388608' 'net.core.wmem_max = 8388608' \
  'net.core.rmem_default = 1048576' 'net.core.wmem_default = 1048576' \
  'net.core.netdev_max_backlog = 5000' | sudo tee /etc/sysctl.d/99-peep-udp.conf
sudo sysctl --system   # depois, reinicie o LiveKit (redeploy) para aplicar
```

`livekit.yaml` limita os candidatos ICE à interface `enp0s6`; troque pelo nome
da placa de rede do seu servidor (`ip -br a`).

## 3. Traefik (rotas para o LiveKit em host network)

Copie `traefik-peep.yml` (trocando `DOMINIO`) para
`/data/coolify/proxy/dynamic/peep.yaml` (ou Coolify → Servers → Proxy →
Dynamic Configurations). O Traefik recarrega sozinho.

## 4. Coolify

- Recursos: **GitHub App / Public Repository** → este repo, branch `main`, build
  pack **Docker Compose**, Docker Compose Location `/docker-compose.yml`:
  - **API + site:** Base Directory `/infra`. Domínios: `room-api` →
    `https://api.DOMINIO:8080`; `web` → `https://call.DOMINIO:8080`. Watch paths:
    `apps/api/**`, `apps/web/**`, `infra/docker-compose.yml`.
  - **LiveKit:** Base Directory `/infra/livekit`, sem domínio (Traefik dynamic
    config). Watch paths: `infra/livekit/**`. Variáveis: `LIVEKIT_API_KEY`,
    `LIVEKIT_API_SECRET`, `TURN_DOMAIN`. Mexer nele derruba chamadas em andamento:
    só faça deploy com as salas vazias (o painel `/admin` mostra).
- Variáveis: ver `.env.example`. Gere as chaves com
  `docker run --rm livekit/livekit-server generate-keys`.
- Se a API reiniciar, salas ainda vivas no LiveKit são reconhecidas na próxima
  entrada (o código é recuperado consultando o LiveKit); sem banco de dados.

### Notificações do Discord

Na aplicação API/site, configure `DISCORD_WEBHOOK_URL` no Coolify. A URL é secreta
e não vai para o cliente. As mensagens incluem nome e código na criação, entrada
e saída de participantes, e encerramento da sala (inclusive pelo administrador).
Criação e expiração de códigos nunca usados são notificações de melhor esforço.
Entradas/saídas vêm de conexões reais, não da emissão de tokens.

O LiveKit envia eventos assinados para `POST /webhooks/livekit`; ajuste a URL em
`livekit/livekit.yaml` se mudar o domínio da API. `LIVEKIT_WEBHOOK_API_KEY` usa a
mesma chave do servidor. Publique primeiro a API e só depois o LiveKit, com salas
vazias. O endpoint verifica assinatura e checksum, ignora eventos irrelevantes
e suprime IDs repetidos por 24 horas em memória. Sem fila durável: reiniciar a API
pode repetir uma mensagem reenviada; eventos durante indisponibilidade do Discord
são tentados novamente pelo LiveKit dentro dos limites de sua fila.

## 5. Smoke test

```sh
curl https://api.DOMINIO/healthz                     # ok
curl -XPOST -H "Authorization: Bearer $INVITE_SECRET" \
  -d '{"name":"teste"}' https://api.DOMINIO/rooms     # {"code":"…"}
```

Depois, LiveKit Meet (`meet.livekit.io` → custom server `wss://lk.DOMINIO`
com um token de `POST /rooms/{code}/join`) em duas redes diferentes (uma no 4G).
Para validar TURN, force relay (`iceTransportPolicy: 'relay'`).
