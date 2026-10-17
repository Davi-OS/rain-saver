# rain-saver — Documentação Técnica

Backend principal da solução Rain Saver: gerencia endereços de usuários e calcula a economia de água obtida com captação pluvial, usando dados de chuva de estações meteorológicas do INMET. Não possui tabela local de usuários — identidade é delegada inteiramente ao [go-auth-service](../../go-auth-service/docs/ARCHITECTURE.md).

## Visão geral

- **Módulo:** `github.com/Davi-OS/rain-saver`, Go `1.25.0`.
- **Camadas:** `cmd/api` (composição) → `internal/http` (Gin) → `internal/service` (regra de negócio) → `internal/repository` (interfaces) ← `internal/repository/postgres` (implementação, pgx/v5, sem ORM).
- Services dependem só das interfaces em `internal/repository/repository.go`, nunca do pacote `postgres` diretamente — inversão de dependência clássica, permitindo testar a camada de service com fakes em memória sem banco.

```
cmd/api/main.go
internal/
  config/config.go
  domain/{endereco,pluvial_economy,weather_station}.go
  http/{server,middleware,docs,json,endereco_handler,pluvial_handler,healthz_handler}.go
  http/openapi.yaml            (embutido no binário via go:embed)
  repository/repository.go     (interfaces + ErrNotFound)
  repository/postgres/{postgres,endereco_repository,pluvial_economy_repository,weather_station_repository}.go
  service/{endereco_service,pluvial_service,errors}.go
migrations/                    (golang-migrate)
scripts/seed_dev_stations.sql  (fixture de dev, NÃO é migration)
```

### Dependências diretas (go.mod)

| Módulo | Propósito |
|---|---|
| `github.com/Davi-OS/go-auth-service` | SDK (`pkg/authsdk`) para validar JWTs emitidos pelo go-auth-service |
| `github.com/gin-gonic/gin` | Router/framework HTTP |
| `github.com/golang-jwt/jwt/v5` | Primitivas de JWT (usado nos testes para forjar tokens; usado transitivamente pelo authsdk) |
| `github.com/jackc/pgx/v5` | Driver/pool Postgres |
| `github.com/joho/godotenv` | Carrega `.env` no boot |

### Wiring (`cmd/api/main.go`)

1. `config.Load()` — lê env (com fallback `.env`).
2. `postgres.NewPool(ctx, cfg.DSN())` — abre e faz ping no pool pgx.
3. `authsdk.New(ctx, authsdk.Config{BaseURL: cfg.AuthServiceBaseURL, Audience: "rain-saver"})` — busca o JWKS do go-auth-service e inicia o polling de revogação em background.
4. Três repositórios Postgres (`Endereco`, `PluvialEconomy`, `WeatherStation`), todos sobre o mesmo pool.
5. Dois services: `EnderecoService(enderecoRepo)` e `PluvialService(enderecoRepo, economyRepo, stationRepo)`.
6. `rainhttp.NewServer(enderecoSvc, pluvialSvc, authClient)` monta o router Gin.
7. `net/http.Server` (ReadTimeout 5s, WriteTimeout 10s), com graceful shutdown em SIGINT/SIGTERM (timeout de 10s).

## Domain model

```go
// internal/domain/endereco.go
type Endereco struct {
	ID         string
	UserID     string
	Logradouro string
	Number     string
	Cep        string
	Latitude   float64
	Longitude  float64
	Cidade     string
	Estado     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

```go
// internal/domain/pluvial_economy.go
type PluvialEconomy struct {
	ID          string
	EnderecoID  string
	UserID      string
	AreaM2      float64
	PeriodStart time.Time
	PeriodEnd   time.Time
	CreatedAt   time.Time

	// StationCode e DistanceKm registram qual estação alimentou o cálculo
	// e a que distância, para auditabilidade.
	StationCode string
	DistanceKm  float64

	MonthlyCaptation []PluvialMonthlyCaptation
}

type PluvialMonthlyCaptation struct {
	ID               string
	PluvialEconomyID string
	Year             int
	Month            int
	RainfallMM       float64
	CapturedLiters   float64
}
```

```go
// internal/domain/weather_station.go
type WeatherStation struct {
	Code      string
	Name      string
	Latitude  float64
	Longitude float64
	UF        string
	CreatedAt time.Time
}

// StationMonthlyRainfall é uma linha já compilada pelo inmet-rain-ingestor —
// um valor por mês/estação. rain-saver nunca agrega leituras brutas.
type StationMonthlyRainfall struct {
	StationCode string
	Year        int
	Month       int
	RainfallMM  float64
}
```

Não existe struct `User`: identidade vive inteiramente no go-auth-service; `enderecos.user_id` é um `uuid` sem FK local (documentado na própria migration).

## Endpoints HTTP

Middleware global: `gin.Recovery()`, CORS (`CORS_ALLOWED_ORIGINS`) + limite de 1MB no corpo da requisição. Todas as rotas exceto `/openapi.yaml`, `/docs` e `/healthz` exigem `RequireAuth` **e** uma permissão do sistema `rain-saver` (`RequirePermission`, 403 sem ela).

| Método | Rota | Permissão | Descrição |
|---|---|---|---|
| GET | `/openapi.yaml` | — (público) | Spec OpenAPI 3.0.3 embutida no binário |
| GET | `/docs` | — (público) | Swagger UI apontando para `/openapi.yaml` |
| GET | `/healthz` | — (público) | `{"status":"ok"}` |
| POST | `/enderecos` | `endereco:write` | Cria endereço do usuário autenticado (geocodifica se faltar lat/lng) |
| GET | `/enderecos` | `endereco:read` | Lista endereços do usuário autenticado |
| GET | `/enderecos/:id` | `endereco:read` | Busca um endereço (ownership obrigatório) |
| DELETE | `/enderecos/:id` | `endereco:write` | Remove um endereço (ownership obrigatório) |
| POST | `/enderecos/:id/pluvial-economy` | `pluvial:calculate` | Calcula e persiste uma nova economia pluvial |
| GET | `/enderecos/:id/pluvial-economy` | `endereco:read` | Retorna o cálculo mais recente para o endereço |

Contas sem acesso ao rain-saver (sem organização habilitada para o sistema) recebem tokens sem `org` e **sem permissões**, então `RequirePermission` também as barra. Os dados continuam escopados por usuário (`sub`); o claim `org` ainda não é usado para escopo.

**POST /enderecos** — request:
```go
type enderecoCreateRequest struct {
	Logradouro string   `json:"logradouro" binding:"required"`
	Number     string   `json:"number" binding:"required"`
	Bairro     string   `json:"bairro"`
	Cep        string   `json:"cep"`
	Latitude   *float64 `json:"latitude" binding:"omitempty,gte=-90,lte=90"`
	Longitude  *float64 `json:"longitude" binding:"omitempty,gte=-180,lte=180"`
	Cidade     string   `json:"cidade"`
	Estado     string   `json:"estado"`
}
```
`latitude`/`longitude` são opcionais e vão juntas; sem elas o endereço é geocodificado e `cidade` + `estado` passam a ser obrigatórios. 201 no sucesso; 400 body inválido; 401 sem auth; 403 sem `endereco:write`; **422** endereço não localizado; **503** geocodificador indisponível. Não há edição (PUT): cálculos guardam a estação escolhida para a localização original, então corrigir = excluir e criar de novo. `user_id` na resposta vem sempre do claim `sub` do JWT, nunca do body.

`binding.EnableDecoderDisallowUnknownFields = true` está setado globalmente — campos JSON desconhecidos em qualquer request são rejeitados com 400.

**GET/DELETE /enderecos/:id** — 404 `{"error":"endereco not found"}` tanto para "não existe" quanto para "não pertence ao usuário" (mesma mensagem nos dois casos, para não vazar enumeração de IDs).

**POST /enderecos/:id/pluvial-economy** — request:
```go
type pluvialCalculationRequest struct {
	AreaM2 float64 `json:"area_m2" binding:"required,gt=0"`
}
```
Resposta inclui `station_code`, `distance_km` e `monthly_captation: []{year, month, rainfall_mm, captured_liters}`. Status: 201; 400; 401; 404 endereco not found; **503** `{"error":"no weather station available"}` se não houver nenhuma estação cadastrada.

**GET /enderecos/:id/pluvial-economy** — 200; 401; 404 `{"error":"no pluvial economy calculated for this endereco"}`.

Envelope de erro padrão: `{"error": "<mensagem>"}`.

## Lógica de negócio (`internal/service/`)

### `EnderecoService`
CRUD fino sobre `repository.EnderecoRepository`, mais geocodificação no `Create`: com `Latitude`/`Longitude` nil, chama o `geocoding.Geocoder` injetado (`ErrNotFound` → `ErrAddressNotFound`, falha do provedor → `ErrGeocodingUnavailable`; nada é gravado nesses casos). Normaliza espaços e UF em maiúsculas. `Get`/`Delete` traduzem `repository.ErrNotFound` → `service.ErrEnderecoNotFound`; `Get` também verifica `e.UserID != userID` na própria service (não no banco); `Delete` delega o ownership para a query SQL (`WHERE id=$1 AND user_id=$2`), evitando janela TOCTOU.

### `PluvialService` — o núcleo da lógica de negócio

`Calculate(ctx, enderecoID, userID, areaM2)`:
1. Carrega o endereço e verifica ownership (`ErrEnderecoNotFound` se não achar ou não for do usuário).
2. `stations.Nearest(lat, lon)` — busca a estação mais próxima (Haversine, ver seção de repositórios); `ErrNotFound` → `service.ErrNoStationAvailable`.
3. `stations.MonthlyRainfall(stationCode, 12)` — busca até os últimos 12 meses já agregados (populados pelo `inmet-rain-ingestor`); **rain-saver nunca agrega dado bruto**.
4. Monta `PluvialEconomy` com janela móvel de 1 ano (`PeriodStart = now - 1 ano`, `PeriodEnd = now`), `StationCode`, `DistanceKm` arredondado a 2 casas.
5. **Fórmula de captação**: `CapturedLiters = round(areaM2 × rainfall_mm, 2)` por mês — 1mm de chuva sobre 1m² = 1 litro (`Volume(L) = Área(m²) × Chuva(mm)`), sem coeficiente de eficiência/perda — corresponde à física padrão usada no backend antigo (GreenTech).
6. Persiste tudo transacionalmente via `economies.Create`.

`Latest(ctx, enderecoID, userID)`: mesmo ownership check, delega para `economies.GetLatestByEnderecoID`.

### Erros sentinela
```go
var (
	ErrEnderecoNotFound     = errors.New("service: endereco not found")
	ErrNoStationAvailable   = errors.New("service: no weather station available")
	ErrAddressNotFound      = errors.New("service: address could not be located")
	ErrGeocodingUnavailable = errors.New("service: geocoding unavailable")
)
```

### Geocodificação (`internal/geocoding/`)
`Geocoder` é uma interface; a implementação é `Nominatim` (OpenStreetMap). Usa a busca estruturada (`street`, `city`, `state`, `countrycodes=br`) do mais preciso para o menos: rua com número → rua sem número → centro da cidade, parando no primeiro resultado. O centro da cidade é impreciso, mas basta para escolher a estação INMET mais próxima. O **CEP não é enviado**: a cobertura de CEP do Nominatim no Brasil é ruim e incluí-lo faz consultas boas voltarem vazias. Respeita a política de uso da instância pública — no máximo 1 requisição/s (limitador no processo) e `User-Agent` identificável (`GEOCODER_USER_AGENT`); timeout de 5s.

## Repositórios (`internal/repository/postgres/`)

- **`EnderecoRepository`** (tabela `enderecos`): `Create`/`GetByID`/`ListByUserID` diretos; `Delete` com ownership embutido no `WHERE` (single round-trip, TOCTOU-safe).
- **`PluvialEconomyRepository`** (`pluvial_economy` + `pluvial_monthly_captation`): `Create` roda em transação explícita (`Begin`/`defer Rollback`/`Commit`) — insere o pai, depois cada mês filho; se algum mês violar o `UNIQUE(pluvial_economy_id, year, month)`, a transação inteira reverte. `GetLatestByEnderecoID` é um padrão *append-only log*: `ORDER BY created_at DESC LIMIT 1` — cada cálculo é uma linha nova, nunca um upsert.
- **`WeatherStationRepository`** (`weather_stations`, `station_monthly_rainfall`, somente leitura — populadas pelo `inmet-rain-ingestor`):
  ```sql
  -- Haversine em SQL puro: ~1000 estações torna um full scan + ORDER BY
  -- barato o suficiente para não justificar PostGIS.
  SELECT code, name, latitude, longitude, uf, created_at,
      (6371 * acos(cos(radians($1)) * cos(radians(latitude))
        * cos(radians(longitude) - radians($2))
        + sin(radians($1)) * sin(radians(latitude)))) AS distance_km
  FROM weather_stations
  ORDER BY distance_km ASC
  LIMIT 1
  ```
  `MonthlyRainfall(code, n)`: subquery pega as `n` linhas mais recentes (`ORDER BY ... DESC`), a query externa reordena cronologicamente — não assume que a tabela tem exatamente N linhas, mantendo o rain-saver desacoplado da política de retenção do lado de ingestão.

## Schema do banco (migrations, em ordem)

**`000001_init_schema.up.sql`** — endereços + log de cálculos:
```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- user_id referencia um usuário de outro serviço (go-auth-service) — sem
-- FK local, por design (fronteira de serviço/banco separada).
CREATE TABLE enderecos (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL,
    logradouro text NOT NULL,
    number     text NOT NULL,
    cep        text,
    latitude   double precision NOT NULL,
    longitude  double precision NOT NULL,
    cidade     text,
    estado     text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_enderecos_user_id ON enderecos(user_id);

-- Log append-only: cada cálculo insere uma linha nova em vez de upsert,
-- então um histórico de cálculos sai "de graça" no futuro.
CREATE TABLE pluvial_economy (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endereco_id  uuid NOT NULL REFERENCES enderecos(id) ON DELETE CASCADE,
    user_id      uuid NOT NULL,
    area_m2      double precision NOT NULL,
    period_start date NOT NULL,
    period_end   date NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_pluvial_economy_endereco_id ON pluvial_economy(endereco_id);
CREATE INDEX idx_pluvial_economy_endereco_created_at ON pluvial_economy(endereco_id, created_at DESC);

CREATE TABLE pluvial_monthly_captation (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pluvial_economy_id uuid NOT NULL REFERENCES pluvial_economy(id) ON DELETE CASCADE,
    year               int NOT NULL,
    month              int NOT NULL CHECK (month BETWEEN 1 AND 12),
    rainfall_mm        double precision NOT NULL,
    captured_liters    double precision NOT NULL,
    UNIQUE (pluvial_economy_id, year, month)
);
CREATE INDEX idx_pluvial_monthly_captation_economy_id ON pluvial_monthly_captation(pluvial_economy_id);
```

**`000002_weather_stations.up.sql`** — dados de referência do INMET + link com `pluvial_economy`:
```sql
-- weather_stations e station_monthly_rainfall são populadas por um
-- processo de ingestão externo (inmet-rain-ingestor) — este serviço só lê.
CREATE TABLE weather_stations (
    code       text PRIMARY KEY,
    name       text NOT NULL,
    latitude   double precision NOT NULL,
    longitude  double precision NOT NULL,
    uf         text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE station_monthly_rainfall (
    station_code text NOT NULL REFERENCES weather_stations(code) ON DELETE CASCADE,
    year         int NOT NULL,
    month        int NOT NULL CHECK (month BETWEEN 1 AND 12),
    rainfall_mm  double precision NOT NULL,
    PRIMARY KEY (station_code, year, month)
);

ALTER TABLE pluvial_economy
    ADD COLUMN station_code text REFERENCES weather_stations(code),
    ADD COLUMN distance_km  double precision;
```

**`000003_endereco_bairro.up.sql`** — `ALTER TABLE enderecos ADD COLUMN bairro text NOT NULL DEFAULT ''` (bairro vindo do ViaCEP, só exibição).

`scripts/seed_dev_stations.sql` **não é uma migration** — fixture de dev que insere 2 estações reais do INMET (`A701` São Paulo - Mirante, `A652` Belo Horizonte - Cercadinho, **atenção**: no catálogo real do INMET o código `A652` na verdade corresponde a "Rio de Janeiro - Forte de Copacabana", não Belo Horizonte — o seed de dev usa esse código com um nome/cidade fictícios só para teste) com chuva mensal sintética, para exercitar o read path sem depender do `inmet-rain-ingestor`.

## Integração de autenticação

1. `authsdk.Client` é construído no boot com `Audience: "rain-saver"`. Ele busca o JWKS do go-auth-service, monta um verificador local (issuer + chaves), faz um fetch inicial de `/revocations` e inicia um polling em background (padrão 5s) que mantém dois mapas em memória: `jti` revogados e `userInvalidAfter` (invalidação "sair de todos os lugares").
2. `RequireAuth` (middleware Gin, `internal/http/middleware.go`): extrai `Authorization: Bearer <token>`, chama `client.Verify(token)` — que valida assinatura ES256, issuer, **audience exatamente `"rain-saver"`** (exige um registro `systems.slug = 'rain-saver'` no banco do go-auth-service), expiração, e por fim a revogação local (cache de polling). Falha → 401 `{"error":"unauthorized: <erro>"}`.
3. `RequirePermission(perm)` (após `RequireAuth`) exige o permission string no claim `perms` → 403 `{"error":"forbidden: missing permission <perm>"}`. O catálogo (`endereco:read`, `endereco:write`, `pluvial:calculate`) é registrado no go-auth-service (em dev, pelo `cmd/devseed`).
4. **`user_id` nunca vem do body** — sempre extraído de `claims.Subject` (`sub` do JWT) via `UserIDFromContext`, e é essa a única fonte de verdade para ownership em todos os handlers/services.
5. rain-saver não guarda nenhuma chave/segredo de JWT localmente — a confiança é inteiramente delegada às chaves públicas do go-auth-service, buscadas no boot.

## Configuração (`internal/config/config.go`)

| Env var | Default |
|---|---|
| `PORT` | `8081` |
| `DATABASE_URL` | `""` (se setado, usado verbatim, tem prioridade) |
| `POSTGRES_HOST` | `localhost` |
| `POSTGRES_PORT` | `5433` (propositalmente diferente do 5432 do go-auth-service, para rodar os dois lado a lado) |
| `POSTGRES_USER` | `rainsaver` |
| `POSTGRES_PASSWORD` | `rainsaver` |
| `POSTGRES_DB` | `rainsaver` |
| `AUTH_SERVICE_BASE_URL` | `http://localhost:8080` |
| `CORS_ALLOWED_ORIGINS` | `""` (CORS desligado); lista separada por vírgula, ex. `http://localhost:5173` |
| `GEOCODER_URL` | `https://nominatim.openstreetmap.org` |
| `GEOCODER_USER_AGENT` | `rain-saver (+https://github.com/Davi-OS/rain-saver)` — obrigatório pela política do Nominatim |

## Build & Deploy

- **Dockerfile** (multi-stage): builder `golang:1.25-alpine` com `GOPRIVATE=github.com/Davi-OS/*` (módulo `go-auth-service` é privado); `go mod download` usa um **secret do BuildKit montado como netrc** (`--mount=type=secret,id=netrc,...`) para nunca gravar credenciais git na imagem:
  ```
  docker build --secret id=netrc,src=$HOME/.netrc -t rain-saver .
  ```
  Runtime: `alpine:3.20`, usuário não-root `appuser` (uid 10001), `EXPOSE 8081`.
- **docker-compose.yml**: só o serviço `postgres` (a API roda via `go run`/binário, fora do compose), porta `5433:5432`.
- **CI/CD**: não existe `.github/workflows` neste repositório — nenhuma pipeline configurada até o momento.

## Testes

- `go test ./...`. Testes de repositório contra Postgres real usam `testutil_test.go`: pool via `TEST_DATABASE_URL`, sem default (antes caía no banco de dev): **skip** se a variável não estiver definida ou o banco não estiver acessível, e **falha** se o nome do banco não terminar em `_test`; usa um **advisory lock** (`573_918_264`, escolhido para não colidir com o `918_273_645` do go-auth-service) para serializar os testes contra um banco compartilhado, e faz `TRUNCATE ... RESTART IDENTITY CASCADE` antes de cada rodada.
- Testes de service usam fakes em memória (`internal/service/fakes_test.go`) — sem necessidade de banco.
- `internal/http/middleware_test.go` sobe um **servidor JWKS/revocations fake** (`httptest`), assina tokens ES256 com `golang-jwt/jwt/v5` compatíveis com o esquema do go-auth-service, e testa `RequireAuth` fim a fim.

## Setup local (resumo do README)

1. `export GOPRIVATE=github.com/Davi-OS/*` e garantir acesso git ao `go-auth-service` privado.
2. Copiar `.env.example` → `.env`.
3. `docker compose up -d postgres`; rodar as migrations com `golang-migrate`.
4. Popular dados de estação: `psql ... -f scripts/seed_dev_stations.sql`.
5. No banco do go-auth-service: `INSERT INTO systems (name, slug) VALUES ('Rain Saver', 'rain-saver');` (passo manual, sem endpoint HTTP para isso) — depois obter um token via `POST /login` do go-auth-service com `"system": "rain-saver"`.
6. `go run ./cmd/api` → `http://localhost:8081` (`/docs`, `/openapi.yaml`, `/healthz`).
