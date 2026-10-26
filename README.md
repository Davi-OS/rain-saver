# rain-saver

Backend em Go que gerencia endereços e calcula/entrega economia **mensal** de captação de água de chuva para o front-end do projeto Rain Saver, usando a chuva histórica da **estação INMET mais próxima** de cada endereço. Login e contas de usuário vivem num serviço separado, o [go-auth-service](https://github.com/Davi-OS/go-auth-service) — este backend não tem cadastro/senha próprios, só valida tokens emitidos por ele.

Os dados de chuva (`weather_stations`, `station_monthly_rainfall`) são povoados por um processo de ingestão do INMET separado (fora deste repositório, ainda não construído) — este backend só **lê** dessas tabelas, já compiladas por mês/estação. Veja a seção de setup local para semear dados de teste enquanto essa ingestão não existe.

## Stack

- Go 1.25+, [Gin](https://github.com/gin-gonic/gin)
- Postgres via [pgx/v5](https://github.com/jackc/pgx) (sem ORM)
- Autenticação via [`pkg/authsdk`](https://github.com/Davi-OS/go-auth-service) do go-auth-service (JWT ES256, validado contra o JWKS do auth service)
- Docs da API: OpenAPI (`internal/http/openapi.yaml`), Swagger UI em `/docs`

## Setup local

### 1. Dependências

Todas são públicas. O SDK do auth é o [`github.com/Davi-OS/go-auth-sdk`](https://github.com/Davi-OS/go-auth-sdk), então `go build` e `docker build -t rain-saver .` não precisam de credencial do GitHub.

### 2. Variáveis de ambiente

```bash
cp .env.example .env
# edite .env se necessário
```

O Go não carrega `.env` automaticamente fora do processo — `config.Load()` usa [`godotenv`](https://github.com/joho/godotenv), que lê o arquivo `.env` do diretório de trabalho no boot. Isso é suficiente para `go run`/`go test` locais; em produção prefira variáveis de ambiente reais.

### 3. Banco de dados

```bash
docker compose up -d postgres
```

Sobe um Postgres local na porta `5433` (porta diferente da usada pelo go-auth-service, `5432`, para os dois rodarem juntos sem conflito).

Rodar as migrações (usando [golang-migrate](https://github.com/golang-migrate/migrate)):

```bash
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate -database "postgres://rainsaver:rainsaver@localhost:5433/rainsaver?sslmode=disable" -path migrations up
```

### 4. Dados de estação/chuva (dev)

Como a ingestão real do INMET ainda não existe, semeie estações e chuva mensal sintéticas pra testar localmente:

```bash
psql "postgres://rainsaver:rainsaver@localhost:5433/rainsaver?sslmode=disable" -f scripts/seed_dev_stations.sql
```

Isso não é uma migration — é só um fixture de desenvolvimento (`scripts/seed_dev_stations.sql`), nunca roda em produção.

### 5. go-auth-service (Azure) + sistema `rain-saver`

Este serviço só aceita tokens com audience `"rain-saver"` e issuer igual ao `AUTH_ISSUER` do auth, e exige as permissões `endereco:read`, `endereco:write` e `pluvial:calculate`. O auth roda na Azure (`https://ca-auth.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io`, veja o `docs/DEPLOY_AZURE.md` do go-auth-service). Configure `AUTH_SERVICE_BASE_URL` e `AUTH_ISSUER` com essa URL, como no `.env.example`. No boot, o rain-saver busca o JWKS e encerra se o auth estiver fora do ar (por exemplo, pausado).

O sistema é descrito em [`deploy/auth-system.json`](deploy/auth-system.json) e cadastrado uma única vez por um platform admin, sem seeds:

```bash
AUTH=https://ca-auth.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io
read -rp 'e-mail: ' EMAIL; read -rsp 'senha: ' PASS; echo
login() { jq -nc --arg e "$EMAIL" --arg p "$PASS" --arg o "${1:-}" '{email:$e, password:$p, system:"auth"} + (if $o == "" then {} else {org:$o} end)' | curl -s "$AUTH/login" -d @-; }
PLATFORM=$(login | jq -r '.orgs[]? | select(.slug == "platform") | .id')   # 409 org_required lista as orgs
ROOT=$(login "$PLATFORM" | jq -r .access_token)
jq --arg e "$EMAIL" '. + {admin_email: $e}' deploy/auth-system.json |
  curl -s "$AUTH/systems" -H "Authorization: Bearer $ROOT" -d @- | jq
```

Se a conta só pertence à org `platform`, o primeiro `/login` já devolve o token; nesse caso, use `ROOT=$(login | jq -r .access_token)`. O `admin_email` vira admin do sistema (na org dona `rain-saver`). Uma conta que já existe mantém a senha; uma conta nova entra com a senha inicial (`INITIAL_PASSWORD_PREFIX` + `rain-saver`) e precisa trocá-la em `POST /password/change`. Mudanças posteriores no spec: `PUT /systems/rain-saver` com o mesmo JSON (sem `admin_email`), feito por um admin do sistema.

Depois, `POST /login` com `"system": "rain-saver"` devolve o `access_token` aceito aqui. Para subir o rain-saver com banco e estações de dev e rodar o teste ponta a ponta contra a Azure, use a stack em [`deploy/local`](deploy/local/README.md).

### 6. Rodar

```bash
go run ./cmd/api
```

Serve em `http://localhost:8081`. Docs em `/docs`, spec em `/openapi.yaml`, healthcheck em `/healthz`.

## Testes

```bash
go test ./...
```

Os testes de `internal/repository/postgres` precisam de um Postgres real e fazem `TRUNCATE` de todas as tabelas, então só rodam contra um banco **descartável** indicado em `TEST_DATABASE_URL`, cujo nome termine em `_test`:

```bash
# uma vez: cria o banco de teste no container do passo 3 e aplica as migrations
docker exec rain-saver-postgres psql -U rainsaver -d postgres -c "CREATE DATABASE rainsaver_test"
for f in migrations/*.up.sql; do docker exec -i rain-saver-postgres psql -U rainsaver -d rainsaver_test -v ON_ERROR_STOP=1 -q < "$f"; done

TEST_DATABASE_URL="postgres://rainsaver:rainsaver@localhost:5433/rainsaver_test?sslmode=disable" go test ./...
```

Sem a variável esses testes são pulados (`t.Skip`, visível com `-v`); com um banco cujo nome não termina em `_test` eles falham de propósito, para nunca apagar o banco de dev.

## Rotas

Ver `internal/http/openapi.yaml` (ou `/docs` com o servidor rodando) para a especificação completa. Resumo:

| Método | Rota | Permissão |
|---|---|---|
| GET | `/healthz` | — (público) |
| GET | `/openapi.yaml`, `/docs` | — (público) |
| POST / GET | `/enderecos` | `endereco:write` / `endereco:read` |
| GET / DELETE | `/enderecos/:id` | `endereco:read` / `endereco:write` |
| POST / GET | `/enderecos/:id/pluvial-economy` | `pluvial:calculate` / `endereco:read` |

`POST /enderecos` aceita `latitude`/`longitude` opcionais: sem elas o endereço é geocodificado via Nominatim (OpenStreetMap) a partir de logradouro, número, cidade e UF — `422` se não for localizado, `503` se o geocodificador estiver fora.

Sem a permissão da rota a resposta é `403`. Todas as rotas protegidas exigem `Authorization: Bearer <token>` emitido pelo go-auth-service para a audience `rain-saver`; o `user_id` de cada recurso vem sempre do claim `sub` do token validado, nunca do corpo da requisição.

A resposta de `pluvial-economy` inclui `station_code` e `distance_km` — qual estação INMET foi usada e a que distância do endereço, pra rastreabilidade. Se nenhuma estação estiver cadastrada ainda, o cálculo retorna `503`.
