#!/usr/bin/env bash
# One-off setup so the "release" job of .github/workflows/docker-publish.yml
# can log in to Azure without a stored password (GitHub OIDC), and nothing
# else can: the federated credentials only match the job that runs under the
# "production" environment (i.e. after the manual approval), and the role is
# scoped to the single Container App, not the resource group.
#
# Run it yourself, logged in with `az login` as someone who may create app
# registrations and role assignments. It prints the repository variables to
# set at the end. Re-running is safe: existing pieces are reused.
set -euo pipefail

RG=${RG:-rg-rainsaver-prod}
APP=${APP:-ca-rainsaver}
GH_REPO=${GH_REPO:-Davi-OS/rain-saver}
ENVIRONMENT=${ENVIRONMENT:-production}
SP_NAME=${SP_NAME:-gh-rain-saver-deploy}

SUB=$(az account show --query id -o tsv)
TENANT=$(az account show --query tenantId -o tsv)
APP_SCOPE=$(az containerapp show -g "$RG" -n "$APP" --query id -o tsv)

CLIENT_ID=$(az ad app list --display-name "$SP_NAME" --query '[0].appId' -o tsv)
if [ -z "$CLIENT_ID" ]; then
  CLIENT_ID=$(az ad app create --display-name "$SP_NAME" --query appId -o tsv)
fi
az ad sp show --id "$CLIENT_ID" >/dev/null 2>&1 || az ad sp create --id "$CLIENT_ID" >/dev/null

# GitHub sends the subject in one of two shapes depending on the repository
# (names, or names plus numeric ids: repo:OWNER@ID/NAME@ID:environment:ENV).
# Azure needs an exact match, so both are registered.
OWNER_ID=$(gh api "repos/$GH_REPO" --jq .owner.id)
REPO_ID=$(gh api "repos/$GH_REPO" --jq .id)
OWNER=${GH_REPO%%/*}
NAME=${GH_REPO#*/}
add_credential() { # name subject
  if [ -z "$(az ad app federated-credential list --id "$CLIENT_ID" --query "[?subject=='$2'].name | [0]" -o tsv)" ]; then
    az ad app federated-credential create --id "$CLIENT_ID" --parameters "$(jq -nc --arg n "$1" --arg s "$2" '{
      name: $n,
      issuer: "https://token.actions.githubusercontent.com",
      subject: $s,
      audiences: ["api://AzureADTokenExchange"]
    }')" >/dev/null
  fi
}
add_credential github-production "repo:$GH_REPO:environment:$ENVIRONMENT"
add_credential github-production-by-id "repo:$OWNER@$OWNER_ID/$NAME@$REPO_ID:environment:$ENVIRONMENT"

az role assignment create --assignee "$CLIENT_ID" --role Contributor --scope "$APP_SCOPE" >/dev/null 2>&1 || true

cat <<OUT

Set these repository variables (they are identifiers, not secrets):

  gh variable set AZURE_CLIENT_ID       -R $GH_REPO -b $CLIENT_ID
  gh variable set AZURE_TENANT_ID       -R $GH_REPO -b $TENANT
  gh variable set AZURE_SUBSCRIPTION_ID -R $GH_REPO -b $SUB
  gh variable set AZURE_RESOURCE_GROUP  -R $GH_REPO -b $RG
  gh variable set AZURE_CONTAINER_APP   -R $GH_REPO -b $APP
OUT
