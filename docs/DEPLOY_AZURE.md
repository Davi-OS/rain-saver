# Guia de deploy do rain-saver na Azure

> **Estado (2026-10-25):** no ar em
> `https://ca-rainsaver.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io`
> (imagem `v0.1.0`), com o banco `psql-rainsaver-prod-f0eea6` e o job
> `job-inmet-ingestor`, que já carregou 2025 e o parcial de 2026 (até setembro).
> O sistema `rain-saver` está cadastrado no auth. As variáveis ficam em
> `~/.rainsaver-azure-env.sh` e os segredos no cofre do auth, em
> `~/.auth-azure-secrets/rainsaver/`.

## Visão geral

```
Front (GitHub Pages, https://davi-os.github.io)
   │ HTTPS + Bearer token (CORS)
   ▼
Container App "ca-rainsaver" (0–1 réplica)  ──JWKS + /revocations──▶  ca-auth (go-auth-service)
   │  secret: DATABASE_URL (sslmode=require)
   ▼
PostgreSQL Flexible Server "psql-rainsaver-prod-<hex>" (B1ms, 32 GB)
   ▲
Container Apps Job "job-inmet-ingestor" (cron anual) ── baixa <ano>.zip do INMET

Ambiente: cae-auth-prod (o mesmo do auth)   Logs: log-auth-prod
```

Decisões deste deploy:

1. **Servidor Postgres próprio**, separado do auth. O isolamento é total, e o custo é de ~US$ 16/mês, porque a conta gratuita só cobre um B1ms, o do auth.
2. **Mesmo ambiente do auth (`cae-auth-prod`).** A assinatura só aceita **um** Container Apps Environment por região (`MaxNumberOfRegionalEnvironmentsInSubExceeded`). Por isso a app e o job ficam no `rg-rainsaver-prod`, mas rodam no ambiente do `rg-auth-prod`, o que a Azure permite entre resource groups. Os logs vão para o `log-auth-prod`. Para ter um ambiente separado, seria preciso pedir aumento de cota (Quotas no portal) ou usar outra região.
3. **Imagens públicas no Docker Hub.** Elas não têm segredos, e o SDK do auth é público ([`go-auth-sdk`](https://github.com/Davi-OS/go-auth-sdk)). Assim o build não precisa de credencial do GitHub e a Azure não precisa de token do registry.
4. **Escala a zero** (`minReplicas: 0`). Parada, a app não custa nada, e a primeira requisição leva ~2 s a mais. A réplica é uma só (`maxReplicas: 1`) porque o Nominatim público aceita no máximo 1 req/s.
5. **Firewall em vez de VNet** e **secrets direto na app**, pelos mesmos motivos do auth (veja o `docs/DEPLOY_AZURE.md` do go-auth-service): as migrations rodam com `psql` da sua máquina.

## 0. Pré-requisitos

- O auth no ar, porque o rain-saver busca o JWKS no boot e encerra se não conseguir. Se ele estiver pausado, retome-o antes (passo 10 do guia do auth).
- Ferramentas: `az` (com a extensão `containerapp`), Docker e `jq`. O `psql` roda via Docker.
- O cofre do auth montado em `~/.auth-azure-secrets`.
- `docker login -u davios18` com um token **Read & Write**, só para o push.

Variáveis (`~/.rainsaver-azure-env.sh`; num terminal novo, rode `source` nele):

```sh
RG=rg-rainsaver-prod
LOC=northcentralus
PG=psql-rainsaver-prod-$(openssl rand -hex 3)   # único na Azure; anote o valor gerado
ENV_ID=$(az containerapp env show -n cae-auth-prod -g rg-auth-prod --query id -o tsv)
APP=ca-rainsaver
JOB=job-inmet-ingestor
DOCKERHUB_USER=davios18
IMAGE=docker.io/$DOCKERHUB_USER/rain-saver:v0.1.0
INGESTOR_IMAGE=docker.io/$DOCKERHUB_USER/inmet-rain-ingestor:v0.1.0
AUTH_URL=https://ca-auth.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io
FRONT_ORIGIN=https://davi-os.github.io
INMET_BASE_URL=https://portal.inmet.gov.br/uploads/dadoshistoricos
SECRETS=~/.auth-azure-secrets/rainsaver
psql() { docker run --rm -i --network host -v "$PWD:/work:ro" -w /work postgres:16-alpine psql "$@"; }
```

## 1. Cadastrar o sistema no auth

Isso é feito uma vez, por um platform admin, com o `deploy/auth-system.json` (os curls estão na seção 5 do README). O cadastro também libera o CORS do auth para as `web_origins` do spec (`https://davi-os.github.io`). Para conferir sem credenciais:

```sh
curl -s -o /dev/null -D - -X OPTIONS $AUTH_URL/login -H "Origin: $FRONT_ORIGIN" \
  -H 'Access-Control-Request-Method: POST' | grep -i access-control-allow-origin
```

## 2. Grupo de recursos e banco

```sh
mkdir -p $SECRETS; umask 077
openssl rand -base64 24 | tr -d '/+=' > $SECRETS/pg-admin-pass
openssl rand -base64 24 | tr -d '/+=' > $SECRETS/app-db-pass
az group create -n $RG -l $LOC
az postgres flexible-server create -g $RG -n $PG -l $LOC \
  --tier Burstable --sku-name Standard_B1ms --storage-size 32 --version 16 \
  --admin-user pgadmin --admin-password "$(cat $SECRETS/pg-admin-pass)" \
  --public-access $(curl -s https://api.ipify.org) --backup-retention 7 --zonal-resiliency Disabled
az postgres flexible-server firewall-rule create -g $RG -s $PG \
  -n allow-azure-services --start-ip-address 0.0.0.0 --end-ip-address 0.0.0.0
az postgres flexible-server parameter set -g $RG -s $PG --name azure.extensions --value PGCRYPTO
```

Crie o orçamento no portal (Cost Management → Budgets): escopo `rg-rainsaver-prod`, US$ 30/mês.

Usuário e banco da aplicação (o `ALTER SCHEMA` é necessário na Azure, como no auth):

```sh
H=$PG.postgres.database.azure.com
ADMIN="host=$H port=5432 user=pgadmin password=$(cat $SECRETS/pg-admin-pass) sslmode=require"
psql "$ADMIN dbname=postgres" -v ON_ERROR_STOP=1 <<SQL
CREATE ROLE rainsaver LOGIN PASSWORD '$(cat $SECRETS/app-db-pass)';
GRANT rainsaver TO pgadmin;
CREATE DATABASE rainsaver OWNER rainsaver;
REVOKE CONNECT ON DATABASE rainsaver FROM PUBLIC;
SQL
psql "$ADMIN dbname=rainsaver" -c 'ALTER SCHEMA public OWNER TO rainsaver;'
echo "postgres://rainsaver:$(cat $SECRETS/app-db-pass)@$H:5432/rainsaver?sslmode=require" > $SECRETS/database-url
```

## 3. Migrations

Rode da raiz do repo:

```sh
for f in migrations/*.up.sql; do echo "== $f"; psql "$(cat $SECRETS/database-url)" -q -1 -v ON_ERROR_STOP=1 -f "$f" || break; done
psql "$(cat $SECRETS/database-url)" -c '\dt'   # 5 tabelas
```

O `scripts/seed_dev_stations.sql` **não** roda em produção: as estações vêm do job (passo 6).

## 4. Imagem

```sh
docker build -t $IMAGE .     # sem --secret: todas as dependências são públicas
docker push $IMAGE
```

Confira em hub.docker.com se o repositório está **público**; sem isso a Azure não consegue baixar a imagem sem credencial.

## 5. Container App

O YAML tem o secret do banco, então fica no cofre e é apagado logo depois:

```sh
Y=$SECRETS/ca-rainsaver.yaml
cat > $Y <<EOF
location: $LOC
properties:
  managedEnvironmentId: $ENV_ID
  configuration:
    activeRevisionsMode: Single
    ingress: { external: true, targetPort: 8081, transport: auto, allowInsecure: false }
    secrets:
      - name: database-url
        value: "$(cat $SECRETS/database-url)"
  template:
    containers:
      - name: rain-saver
        image: $IMAGE
        resources: { cpu: 0.25, memory: 0.5Gi }
        env:
          - { name: PORT, value: "8081" }
          - { name: DATABASE_URL, secretRef: database-url }
          - { name: AUTH_SERVICE_BASE_URL, value: "$AUTH_URL" }
          - { name: AUTH_ISSUER, value: "$AUTH_URL" }
          - { name: CORS_ALLOWED_ORIGINS, value: "$FRONT_ORIGIN" }
          - { name: GEOCODER_URL, value: "https://nominatim.openstreetmap.org" }
          - { name: GEOCODER_USER_AGENT, value: "rain-saver (+https://github.com/Davi-OS/rain-saver)" }
        probes:
          - { type: Startup, httpGet: { path: /healthz, port: 8081 }, periodSeconds: 3, failureThreshold: 10 }
          - { type: Liveness, httpGet: { path: /healthz, port: 8081 }, periodSeconds: 10 }
    scale: { minReplicas: 0, maxReplicas: 1 }
EOF
az containerapp create -n $APP -g $RG --yaml $Y
shred -u $Y
RS_URL=https://$(az containerapp show -n $APP -g $RG --query properties.configuration.ingress.fqdn -o tsv)
```

- **Startup:** o binário conecta no banco e busca o JWKS e as revogações do auth **antes** de abrir a porta. O startup probe dá 30 s. Se o auth estiver fora, os logs mostram `authsdk: fetch jwks` e a réplica não sobe.
- **Sem `/readyz`:** o rain-saver só tem `/healthz`, que não toca no banco. Uma queda do Postgres aparece como 5xx nas rotas, sem reiniciar o container.

## 6. Job do ingestor

Siga o `docs/DEPLOY_AZURE.md` do `inmet-rain-ingestor`, com `RG=rg-rainsaver-prod` e `--environment "$ENV_ID"`. Como a imagem é pública, não há `--registry-*`. O `DATABASE_URL` é o do cofre (`$SECRETS/database-url`).

**Backfill obrigatório:** o cálculo pluvial usa os **últimos 12 meses**, então, depois de criar o job, carregue o ano anterior e o parcial do ano corrente:

```sh
for y in 2025 2026; do
  az containerapp job start -n $JOB -g $RG --image $INGESTOR_IMAGE --container-name ingestor --cpu 1 --memory 2Gi \
    --env-vars DATABASE_URL=secretref:database-url INMET_BASE_URL=$INMET_BASE_URL --args="-year=$y"
done
```

Use `--args="-year=..."`, com `=`: na forma `--args "-year=..."`, o `az` lê o valor como uma flag e responde `unrecognized arguments`. Para mais de um argumento (por exemplo, `-stations`), use o YAML descrito no guia do ingestor. Em 2026-10-25, o carregamento deu 594 estações, 2025 completo e 2026 de janeiro a setembro.

**Lacuna:** o cron é anual (1º de janeiro). Os meses do ano corrente só entram quando você roda o job à mão. Se a janela de 12 meses precisar estar sempre atualizada, troque por um cron mensal com `-year=<ano corrente>`.

## 7. Verificar

```sh
curl -s $RS_URL/healthz                     # {"status":"ok"} (a 1ª chamada acorda a réplica)
curl -s $RS_URL/enderecos -w ' %{http_code}\n'   # 401: exige token
curl -s -o /dev/null -D - -X OPTIONS $RS_URL/enderecos -H "Origin: $FRONT_ORIGIN" \
  -H 'Access-Control-Request-Method: GET' -H 'Access-Control-Request-Headers: authorization' | grep -i access-control
```

Depois, faça o fluxo completo pelo front: login, cadastro de endereço e cálculo pluvial.

## 8. Atualizações

```sh
docker build -t docker.io/$DOCKERHUB_USER/rain-saver:<nova-tag> . && docker push docker.io/$DOCKERHUB_USER/rain-saver:<nova-tag>
az containerapp update -n $APP -g $RG --image docker.io/$DOCKERHUB_USER/rain-saver:<nova-tag>
```

Migrations novas: aplique com o `psql` (passo 3) **antes** do `update`.

## 9. Pausar e retomar

A app e o job não cobram parados. Só o banco custa:

```sh
az postgres flexible-server stop -g $RG -n $PG     # pausar (a Azure religa sozinha após 7 dias)
az postgres flexible-server start -g $RG -n $PG    # retomar
```

Com o auth pausado, a app não consegue subir uma réplica nova, porque não acha o JWKS. Retome o auth primeiro.

## Custo estimado

| Item | US$/mês |
|---|---|
| PostgreSQL B1ms + 32 GB | ~16 (fora da conta gratuita, que já cobre o do auth) |
| Container App (escala a zero) | ~0 a 2 |
| Job (alguns minutos por ano) | ~0 |
| **Pausado** | **~4** (armazenamento do banco) |
