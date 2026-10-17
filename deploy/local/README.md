# Stack local: go-auth-service + rain-saver

Sobe os dois serviços, cada um com seu Postgres, migrations e dados de dev, e
roda um teste ponta a ponta da integração.

```sh
cd deploy/local
./build.sh                    # compila os binários no host (ver abaixo)
docker compose up --build -d
./e2e.sh                      # 56 checks; idempotente
docker compose down -v        # -v apaga bancos e a chave de assinatura
```

Pré-requisitos: Go, Docker Compose v2, `curl`, `jq` e o checkout do
go-auth-service ao lado deste repo (ou `AUTH_SERVICE_DIR=/caminho`). A
geocodificação de endereços usa o Nominatim público, então precisa de
internet (o serviço `rain-saver` usa DNS 1.1.1.1/8.8.8.8 para isso); para
rodar o `e2e.sh` offline, use `E2E_SKIP_GEOCODING=1`.

| Serviço | URL no host |
|---|---|
| go-auth-service | http://localhost:8080 (`AUTH_PORT`) — Swagger em `/docs` |
| rain-saver | http://localhost:8081 (`RAIN_SAVER_PORT`) — Swagger em `/docs` |

Os bancos não são expostos no host, então não conflitam com os
`docker-compose.yml` de cada repo (5432/5433).

## Dados de dev

- `cmd/devseed` (go-auth-service) registra o sistema `rain-saver`
  (permissões `endereco:read`, `endereco:write`, `pluvial:calculate`; roles
  `owner` e `member`), a org `demo` e o usuário **dev@rainsaver.local /
  rainsaver-dev-pass**, owner da org nos sistemas `auth` e `rain-saver`.
  Altere com `DEVSEED_EMAIL` / `DEVSEED_PASSWORD`.
- `scripts/seed_dev_stations.sql` carrega duas estações INMET com chuva
  sintética.
- Outros usuários são criados pela API, como o `e2e.sh` faz com a maria:

```sh
ORG=...   # claim "org" do token
ADMIN=$(curl -s localhost:8080/login -d '{"email":"dev@rainsaver.local","password":"rainsaver-dev-pass","system":"auth","org":"'$ORG'"}' | jq -r .access_token)
curl -s localhost:8080/orgs/$ORG/users -H "Authorization: Bearer $ADMIN" \
  -d '{"email":"fulano@rainsaver.local","password":"senha-com-10+","role":"member","system":"rain-saver"}'
```

## Por que os binários são compilados no host

- O go-auth-service é um módulo privado: o `Dockerfile` do rain-saver precisa
  de um `~/.netrc` com token do GitHub. O `build.sh` usa o checkout local via
  um `-modfile` temporário com `replace` (o `go.mod` do repo não é alterado),
  então o rain-saver é testado contra o SDK do working tree.
- Builds Docker precisam de DNS para `go mod download`/`apk add`, o que falha
  em hosts cujo resolvedor não é alcançável de dentro dos containers.

Consequência: os `Dockerfile`s de produção de cada repo não são exercitados
aqui. Rode `./build.sh` de novo sempre que mudar código Go.

## O que o `e2e.sh` cobre

CORS do front (`http://localhost:5173`, configurável por `CORS_ALLOWED_ORIGINS`), login com `aud=rain-saver` e claims (`iss`, `aud`, `org`, `perms`), `/me`,
criação de usuário por admin, 403 para conta sem permissão no rain-saver,
validação e geocodificação de endereço (Nominatim, 422 para endereço
inexistente), isolamento de dados entre usuários, cálculo
pluvial, rejeição de token (sem token, outra audience, claims adulteradas,
lixo), rotação de refresh e reuso, propagação de `logout` e `logout-all` até o
rain-saver (polling de revogações, até `REVOCATION_WAIT_SECS`) e restart do
auth mantendo a chave.

Não cobre: expiração natural do access token (TTL fixo de 15 min) nem Google
Sign-In.
