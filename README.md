# rain-saver

Backend em Go que gerencia endereços e calcula/entrega economia **mensal** de captação de água de chuva para o front-end do projeto Rain Saver, usando a chuva histórica da **estação INMET mais próxima** de cada endereço. Login e contas de usuário vivem num serviço separado, o [go-auth-service](https://github.com/Davi-OS/go-auth-service) — este backend não tem cadastro/senha próprios, só valida tokens emitidos por ele.

Os dados de chuva (`weather_stations`, `station_monthly_rainfall`) são povoados por um processo de ingestão do INMET separado (fora deste repositório, ainda não construído) — este backend só **lê** dessas tabelas, já compiladas por mês/estação. Veja a seção de setup local para semear dados de teste enquanto essa ingestão não existe.

## Stack

- Go 1.25+, [Gin](https://github.com/gin-gonic/gin)
- Postgres via [pgx/v5](https://github.com/jackc/pgx) (sem ORM)
- Autenticação via [`pkg/authsdk`](https://github.com/Davi-OS/go-auth-service) do go-auth-service (JWT ES256, validado contra o JWKS do auth service)
- Docs da API: OpenAPI (`internal/http/openapi.yaml`), Swagger UI em `/docs`

## Setup local

### 1. Dependência privada (`go-auth-service`)

`go.mod` depende de `github.com/Davi-OS/go-auth-service` (repositório privado). Antes de rodar `go build`/`go mod download`/`go get`:

```bash
export GOPRIVATE=github.com/Davi-OS/*
# confirme que o git consegue acessar o repo privado (ex: via gh auth setup-git,
# ou uma chave SSH/PAT configurada)
git ls-remote https://github.com/Davi-OS/go-auth-service
```

Isso também vale para builds Docker (veja o `Dockerfile`) e para qualquer CI futuro — sem acesso git ao repositório privado, o build falha ao resolver essa dependência.

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

### 5. go-auth-service local + pré-requisito de audience

Este serviço só aceita tokens cuja audience seja `"rain-saver"`. Isso exige uma linha na tabela `systems` do banco do go-auth-service — não existe endpoint HTTP para isso, é um passo manual:

```sql
INSERT INTO systems (name, slug) VALUES ('Rain Saver', 'rain-saver');
```

Depois, com o go-auth-service rodando localmente (porta padrão `8080`), autentique um usuário de teste via `POST /login` com `"system": "rain-saver"` para obter um `access_token`.

### 6. Rodar

```bash
go run ./cmd/api
```

Serve em `http://localhost:8081`. Docs em `/docs`, spec em `/openapi.yaml`, healthcheck em `/healthz`.

## Testes

```bash
go test ./...
```

Os testes de `internal/repository/postgres` precisam de um Postgres real (mesmo do passo 3 acima, ou aponte `TEST_DATABASE_URL` para outra instância) — sem ele, esses testes são pulados automaticamente (`t.Skip`), não falham.

## Rotas

Ver `internal/http/openapi.yaml` (ou `/docs` com o servidor rodando) para a especificação completa. Resumo:

| Método | Rota | Auth |
|---|---|---|
| GET | `/healthz` | não |
| GET | `/openapi.yaml`, `/docs` | não |
| POST, GET | `/enderecos` | sim |
| GET, DELETE | `/enderecos/:id` | sim |
| POST, GET | `/enderecos/:id/pluvial-economy` | sim |

Todas as rotas protegidas exigem `Authorization: Bearer <token>` emitido pelo go-auth-service para a audience `rain-saver`; o `user_id` de cada recurso vem sempre do claim `sub` do token validado, nunca do corpo da requisição.

A resposta de `pluvial-economy` inclui `station_code` e `distance_km` — qual estação INMET foi usada e a que distância do endereço, pra rastreabilidade. Se nenhuma estação estiver cadastrada ainda, o cálculo retorna `503`.
