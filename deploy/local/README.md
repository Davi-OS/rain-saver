# Stack local: rain-saver + auth na Azure

Sobe o rain-saver com o Postgres dele, as migrations e as estações de dev, e
roda um teste ponta a ponta contra o go-auth-service implantado na Azure.

```sh
cd deploy/local
./build.sh                    # compila o binário no host (ver abaixo)
docker compose up --build -d
E2E_ADMIN_EMAIL=... E2E_ADMIN_PASSWORD=... E2E_USER_PASSWORD=... ./e2e.sh
docker compose down -v        # -v apaga o banco
```

Pré-requisitos: Go, Docker Compose v2, `curl`, `jq`, o sistema `rain-saver`
cadastrado no auth (README do repo, seção 5) e o auth **ligado** na Azure. Em
pausa, o rain-saver encerra no boot, porque não consegue buscar o JWKS. Para
retomar o auth, veja o passo 10 do `docs/DEPLOY_AZURE.md` do go-auth-service.
A geocodificação usa o Nominatim público, então precisa de internet (o
serviço usa DNS 1.1.1.1/8.8.8.8 para isso). Para pular essa parte no
`e2e.sh`, use `E2E_SKIP_GEOCODING=1`.

| Serviço | URL |
|---|---|
| go-auth-service | https://ca-auth.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io (`AUTH_SERVICE_BASE_URL`, `AUTH_ISSUER`) |
| rain-saver | http://localhost:8081 (`RAIN_SAVER_PORT`), com Swagger em `/docs` |

O banco não é exposto no host, então não conflita com o `docker-compose.yml`
do repo (5433).

## Dados

- `scripts/seed_dev_stations.sql` carrega duas estações INMET com chuva
  sintética.
- Contas e orgs vivem no auth e são criadas por API, sem seeds. O `e2e.sh`
  faz isso num tenant de teste.

## O `e2e.sh`

Variáveis (só pelo ambiente, nunca no repo, porque as contas são de produção):

| Variável | Uso |
|---|---|
| `E2E_ADMIN_EMAIL` / `E2E_ADMIN_PASSWORD` | admin do sistema `rain-saver` (o `admin_email` do cadastro) |
| `E2E_USER_PASSWORD` | senha final das contas de teste `maria@rainsaver.local` e `semacesso@rainsaver.local` |
| `E2E_TENANT_NAME` | nome do tenant de teste (default `E2E`) |
| `AUTH_URL`, `AUTH_ISSUER`, `RAIN_SAVER_URL` | endereços, com default na Azure e em `localhost:8081` |
| `AUTH_FRONT_ORIGIN`, `FRONT_ORIGIN` | origens testadas no CORS do auth (`web_origins`) e do rain-saver (`CORS_ALLOWED_ORIGINS`) |

Na primeira rodada, o admin cria o tenant `E2E` (`POST /systems/rain-saver/orgs`)
como owner e, dentro dele, cria a maria (member do rain-saver) e a semacesso
(sem role no rain-saver), com uma senha temporária. O script confirma o
`403 password_change_required` e troca a senha em `POST /password/change`. Nas
rodadas seguintes, ele reaproveita o tenant e as contas.

Cobre: health do auth e do rain-saver, CORS dos dois, login com `aud=rain-saver`
e claims (`iss`, `aud`, `org`, `perms`), `/me`, troca obrigatória de senha,
403 para conta sem permissão no rain-saver, validação e geocodificação de
endereço, isolamento de dados entre usuários, cálculo pluvial, rejeição de
token (sem token, outra audience, claims adulteradas, lixo), rotação de
refresh e reuso, e propagação de `logout` e `logout-all` até o rain-saver
(polling de revogações, até `REVOCATION_WAIT_SECS`). O `logout-all` é feito
só na maria.

Não cobre: restart do auth, expiração natural do access token (TTL de 15 min)
nem Google Sign-In.

## Por que o binário é compilado no host

Builds Docker precisam de DNS para o `go mod download`, que falhou de forma
intermitente com o DNS padrão do Docker. O `build.sh` compila com o toolchain
e o cache de módulos do host, e a imagem só copia o binário.
Por isso, o `Dockerfile` de produção não é exercitado aqui. Rode `./build.sh`
de novo sempre que mudar código Go.
