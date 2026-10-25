#!/usr/bin/env bash
# End-to-end checks of rain-saver (local stack, docker-compose.yml in this
# directory) against the go-auth-service deployed on Azure. Needs curl and jq.
#
#   ./build.sh && docker compose up --build -d
#   E2E_ADMIN_EMAIL=... E2E_ADMIN_PASSWORD=... E2E_USER_PASSWORD=... ./e2e.sh
#
# E2E_ADMIN_* is an admin of the rain-saver system (see deploy/auth-system.json
# and the README). The script works inside a test tenant ("E2E", created on
# the first run) and creates the test accounts maria and semacesso there,
# with E2E_USER_PASSWORD as their final password. Credentials only come from
# the environment, since these accounts live in production.
#
# Idempotent: can be re-run against the same stack and tenant.
set -uo pipefail

AUTH=${AUTH_URL:-https://ca-auth.lemonmoss-8726f7e4.northcentralus.azurecontainerapps.io}
AUTH_ISSUER=${AUTH_ISSUER:-$AUTH}
RS=${RAIN_SAVER_URL:-http://localhost:8081}
ADMIN_EMAIL=${E2E_ADMIN_EMAIL:-}
ADMIN_PASS=${E2E_ADMIN_PASSWORD:-}
USER_PASS=${E2E_USER_PASSWORD:-}
TENANT_NAME=${E2E_TENANT_NAME:-E2E}
NOACCESS_EMAIL=semacesso@rainsaver.local
MARIA_EMAIL=maria@rainsaver.local
# Password set by the admin when creating an account; the account must change
# it on first login (password_change_required).
TEMP_PASS=e2e-temporary-pass
# Revocations reach rain-saver via polling (SDK default: every 5s).
REVOCATION_WAIT_SECS=${REVOCATION_WAIT_SECS:-10}
# Front-end origins: the auth allows the system's web_origins
# (deploy/auth-system.json); rain-saver allows CORS_ALLOWED_ORIGINS.
AUTH_FRONT_ORIGIN=${AUTH_FRONT_ORIGIN:-https://davi-os.github.io}
FRONT_ORIGIN=${FRONT_ORIGIN:-http://localhost:5173}

if [[ -z $ADMIN_EMAIL || -z $ADMIN_PASS || -z $USER_PASS ]]; then
  echo "set E2E_ADMIN_EMAIL, E2E_ADMIN_PASSWORD and E2E_USER_PASSWORD" >&2
  exit 2
fi
if [[ $USER_PASS == "$TEMP_PASS" ]]; then
  echo "E2E_USER_PASSWORD must differ from the temporary password" >&2
  exit 2
fi

pass=0
fail=0

# req METHOD URL [TOKEN] [JSON] -> sets $status and $body
req() {
  local method=$1 url=$2 token=${3:-} data=${4:-}
  local args=(-s -o /tmp/e2e_body.$$ -w '%{http_code}' -X "$method" -H 'Content-Type: application/json')
  [[ -n $token ]] && args+=(-H "Authorization: Bearer $token")
  [[ -n $data ]] && args+=(-d "$data")
  status=$(curl "${args[@]}" "$url")
  body=$(cat /tmp/e2e_body.$$ 2>/dev/null)
  rm -f /tmp/e2e_body.$$
}

check() { # check "description" expected_status
  if [[ $status == "$2" ]]; then
    pass=$((pass + 1)); printf '  \033[32mok\033[0m   %s\n' "$1"
  else
    fail=$((fail + 1)); printf '  \033[31mFAIL\033[0m %s: status %s, want %s; body: %s\n' "$1" "$status" "$2" "$body"
  fi
}

assert() { # assert "description" <command...>
  local desc=$1; shift
  if "$@"; then
    pass=$((pass + 1)); printf '  \033[32mok\033[0m   %s\n' "$desc"
  else
    fail=$((fail + 1)); printf '  \033[31mFAIL\033[0m %s\n' "$desc"
  fi
}

jqok() { jq -e "$@" >/dev/null; } # jq as a quiet predicate for assert

b64url() { base64 -w0 | tr '/+' '_-' | tr -d '='; }

claims() { # decode a JWT payload
  local p
  p=$(cut -d. -f2 <<<"$1" | tr '_-' '/+')
  while (( ${#p} % 4 )); do p+='='; done
  base64 -d <<<"$p" 2>/dev/null
}

login() { # login EMAIL PASS SYSTEM [ORG] -> sets $access $refresh
  req POST "$AUTH/login" "" "$(jq -nc --arg e "$1" --arg p "$2" --arg s "$3" --arg o "${4:-}" \
    '{email: $e, password: $p, system: $s} + (if $o == "" then {} else {org: $o} end)')"
  access=$(jq -r '.access_token // empty' <<<"$body")
  refresh=$(jq -r '.refresh_token // empty' <<<"$body")
}

login_in_org() { # login_in_org EMAIL PASS SYSTEM ORG_SLUG -> like login, picking ORG_SLUG on 409 org_required
  login "$1" "$2" "$3"
  if [[ $status == 409 ]]; then
    local id
    id=$(jq -r --arg s "$4" '.orgs[]? | select(.slug == $s) | .id' <<<"$body")
    [[ -n $id ]] && login "$1" "$2" "$3" "$id"
  fi
}

change_temp_password() { # change_temp_password EMAIL -> TEMP_PASS becomes USER_PASS
  req POST "$AUTH/password/change" "" "$(jq -nc --arg e "$1" --arg c "$TEMP_PASS" --arg n "$USER_PASS" \
    '{email: $e, current_password: $c, new_password: $n}')"
}

ensure_user() { # ensure_user EMAIL SYSTEM -> creates EMAIL in the tenant (SYSTEM "" = auth only) and completes the forced password change
  req POST "$AUTH/orgs/$tenant/users" "$admin_access" "$(jq -nc --arg e "$1" --arg p "$TEMP_PASS" --arg s "$2" \
    '{email: $e, password: $p, role: "member"} + (if $s == "" then {} else {system: $s} end)')"
  if [[ $status == 409 ]]; then
    echo "  skip $1 already exists (re-run)"
    change_temp_password "$1" # finishes a change an interrupted run left pending; 401 once done
    return
  fi
  check "owner creates $1" 201
  login "$1" "$TEMP_PASS" auth; check "$1 first login -> 403" 403
  assert "...with error password_change_required" test "$(jq -r .error <<<"$body")" = password_change_required
  change_temp_password "$1"; check "$1 changes the temporary password" 204
}

wait_for_status() { # wait_for_status URL TOKEN EXPECTED -> polls until match or timeout
  local deadline=$((SECONDS + REVOCATION_WAIT_SECS))
  while (( SECONDS < deadline )); do
    req GET "$1" "$2"
    [[ $status == "$3" ]] && return 0
    sleep 0.5
  done
  return 1
}

echo "== health ($AUTH)"
req GET "$AUTH/healthz"; check "auth /healthz" 200
req GET "$AUTH/.well-known/jwks.json"; check "auth JWKS" 200
req GET "$RS/healthz"; check "rain-saver /healthz" 200

echo "== CORS for the front-end"
preflight() { # preflight URL ORIGIN METHOD -> prints the Access-Control-Allow-Origin value
  curl -s -o /dev/null -D - -X OPTIONS "$1" -H "Origin: $2" \
    -H "Access-Control-Request-Method: $3" -H 'Access-Control-Request-Headers: authorization,content-type' |
    tr -d '\r' | awk -F': ' 'tolower($1) == "access-control-allow-origin" {print $2}'
}
assert "auth preflight POST /login allows $AUTH_FRONT_ORIGIN" test "$(preflight "$AUTH/login" "$AUTH_FRONT_ORIGIN" POST)" = "$AUTH_FRONT_ORIGIN"
assert "rain-saver preflight POST /enderecos allows the front" test "$(preflight "$RS/enderecos" "$FRONT_ORIGIN" POST)" = "$FRONT_ORIGIN"
assert "rain-saver preflight refuses other origins" test -z "$(preflight "$RS/enderecos" https://evil.example POST)"
acao=$(curl -s -o /dev/null -D - "$RS/enderecos" -H "Origin: $FRONT_ORIGIN" | tr -d '\r' | awk -F': ' 'tolower($1) == "access-control-allow-origin" {print $2}')
assert "rain-saver 401 still carries CORS headers (front can read the error)" test "$acao" = "$FRONT_ORIGIN"

echo "== test tenant ($TENANT_NAME)"
login_in_org "$ADMIN_EMAIL" "$ADMIN_PASS" auth rain-saver; check "admin logs in for auth in the rain-saver owner org" 200
owner_access=$access
req GET "$AUTH/systems/rain-saver/orgs" "$owner_access"; check "admin lists rain-saver tenants" 200
tenant=$(jq -r --arg n "$TENANT_NAME" '[.orgs[] | select(.name == $n)][0].id // empty' <<<"$body")
if [[ -z $tenant ]]; then
  req POST "$AUTH/systems/rain-saver/orgs" "$owner_access" \
    "$(jq -nc --arg n "$TENANT_NAME" --arg e "$ADMIN_EMAIL" '{name: $n, owner_email: $e}')"
  check "admin creates the test tenant (owner: admin)" 201
  tenant=$(jq -r '.org.id // empty' <<<"$body")
fi
assert "tenant id known" test -n "$tenant"
login "$ADMIN_EMAIL" "$ADMIN_PASS" auth "$tenant"; check "admin logs in for auth in the tenant (admin API)" 200
admin_access=$access

echo "== login (rain-saver audience)"
login "$ADMIN_EMAIL" "$ADMIN_PASS" rain-saver "$tenant"; check "admin logs in for rain-saver in the tenant" 200
dev_access=$access; dev_refresh=$refresh
c=$(claims "$dev_access")
assert "iss = $AUTH_ISSUER" test "$(jq -r .iss <<<"$c")" = "$AUTH_ISSUER"
assert "aud = [rain-saver]" test "$(jq -c .aud <<<"$c")" = '["rain-saver"]'
assert "org claim = tenant" test "$(jq -r .org <<<"$c")" = "$tenant"
assert "perms carry endereco:write" jqok '.perms | index("endereco:write")' <<<"$c"
req GET "$AUTH/me" "$dev_access"; check "/me accepts a rain-saver token" 200

echo "== user management (admin creates users, forced password change)"
ensure_user "$MARIA_EMAIL" rain-saver
login "$MARIA_EMAIL" "$USER_PASS" rain-saver; check "maria logs in for rain-saver" 200
maria_access=$access
assert "maria's token has role member" test "$(claims "$maria_access" | jq -c .roles)" = '["member"]'
ensure_user "$NOACCESS_EMAIL" "" # auth-only member

echo "== permissions"
login "$NOACCESS_EMAIL" "$USER_PASS" rain-saver; check "account without rain-saver role still gets a token" 200
assert "...but it carries no org and no perms" test "$(claims "$access" | jq -c '[.org, (.perms // [] | length)]')" = '[null,0]'
req GET "$RS/enderecos" "$access"; check "rain-saver rejects it on reads (403)" 403
req POST "$RS/enderecos" "$access" '{"logradouro":"X","number":"1","latitude":0,"longitude":0}'; check "rain-saver rejects it on writes (403)" 403

echo "== protected routes and ownership"
req POST "$RS/enderecos" "$dev_access" \
  '{"logradouro":"Av. Paulista","number":"1000","cep":"01310-100","latitude":-23.5614,"longitude":-46.6559,"cidade":"Sao Paulo","estado":"SP"}'
check "dev creates endereco" 201
endereco=$(jq -r .id <<<"$body")
req GET "$RS/enderecos" "$dev_access"; check "dev lists enderecos" 200
assert "list contains the new endereco" jqok --arg id "$endereco" 'map(.id) | index($id)' <<<"$body"
req GET "$RS/enderecos/$endereco" "$maria_access"; check "maria cannot see dev's endereco (404)" 404
req GET "$RS/enderecos" "$maria_access"; check "maria lists her enderecos" 200
assert "maria's list excludes dev's endereco" jqok --arg id "$endereco" 'map(.id) | index($id) == null' <<<"$body"

echo "== address validation and geocoding"
req POST "$RS/enderecos" "$dev_access" '{"logradouro":"X","number":"1","latitude":-23.5}'
check "latitude without longitude -> 400" 400
req POST "$RS/enderecos" "$dev_access" '{"logradouro":"X","number":"1"}'
check "no coordinates and no cidade/estado -> 400" 400
if [[ -n ${E2E_SKIP_GEOCODING:-} ]]; then
  echo "  skip geocoding against Nominatim (E2E_SKIP_GEOCODING set)"
else
  req POST "$RS/enderecos" "$dev_access" \
    '{"logradouro":"Avenida Paulista","number":"1000","bairro":"Bela Vista","cep":"01310-100","cidade":"São Paulo","estado":"SP"}'
  check "address without coordinates is geocoded (Nominatim)" 201
  geocoded=$(jq -r .id <<<"$body")
  assert "geocoded point is on Av. Paulista (lat -23.56±0.02, lon -46.65±0.02)" \
    jqok '(.latitude + 23.56 | fabs) < 0.02 and (.longitude + 46.65 | fabs) < 0.02' <<<"$body"
  assert "bairro is stored" test "$(jq -r .bairro <<<"$body")" = "Bela Vista"
  req POST "$RS/enderecos/$geocoded/pluvial-economy" "$dev_access" '{"area_m2":50}'
  assert "geocoded address resolves to station A701" test "$(jq -r .station_code <<<"$body")" = A701
  req DELETE "$RS/enderecos/$geocoded" "$dev_access"; check "delete geocoded address" 204
  req POST "$RS/enderecos" "$dev_access" '{"logradouro":"Rua Que Nao Existe Xyzzy","number":"1","cidade":"Cidade Inexistente Qwerty","estado":"ZZ"}'
  check "unknown address -> 422" 422
fi

echo "== pluvial economy (dev station fixture)"
req POST "$RS/enderecos/$endereco/pluvial-economy" "$dev_access" '{"area_m2":120}'; check "calculate pluvial economy" 201
assert "nearest station is A701 (Sao Paulo)" test "$(jq -r .station_code <<<"$body")" = A701
req GET "$RS/enderecos/$endereco/pluvial-economy" "$dev_access"; check "read pluvial economy" 200

echo "== token rejection"
req GET "$RS/enderecos"; check "no token -> 401" 401
req GET "$RS/enderecos" "$admin_access"; check "token for another audience (auth) -> 401" 401
# Same header and signature, payload re-encoded with maria's sub: a forged
# identity swap must fail signature verification.
maria_sub=$(claims "$maria_access" | jq -r .sub)
forged_payload=$(claims "$dev_access" | jq -c --arg s "$maria_sub" '.sub = $s' | b64url)
forged="$(cut -d. -f1 <<<"$dev_access").$forged_payload.$(cut -d. -f3 <<<"$dev_access")"
req GET "$RS/enderecos" "$forged"; check "claims altered (sub swapped), original signature -> 401" 401
req GET "$RS/enderecos" "not.a.jwt"; check "garbage token -> 401" 401

echo "== refresh rotation"
req POST "$AUTH/refresh" "" "{\"refresh_token\":\"$dev_refresh\"}"; check "refresh" 200
new_access=$(jq -r .access_token <<<"$body"); new_refresh=$(jq -r .refresh_token <<<"$body")
req GET "$RS/enderecos" "$new_access"; check "refreshed token works on rain-saver" 200
req POST "$AUTH/refresh" "" "{\"refresh_token\":\"$dev_refresh\"}"; check "reusing rotated refresh token -> 401" 401

echo "== logout propagates to rain-saver (revocation polling)"
req POST "$AUTH/logout" "$new_access" "{\"system\":\"rain-saver\",\"refresh_token\":\"$new_refresh\"}"; check "logout" 204
assert "revoked access token rejected by rain-saver within ${REVOCATION_WAIT_SECS}s" wait_for_status "$RS/enderecos" "$new_access" 401
req POST "$AUTH/refresh" "" "{\"refresh_token\":\"$new_refresh\"}"; check "logged-out refresh token -> 401" 401

echo "== logout-all propagates to rain-saver"
req GET "$RS/enderecos" "$maria_access"; check "maria's token works before logout-all" 200
sleep 1 # keep the new login below in a later second than the cutoff (JWT iat has second precision)
req POST "$AUTH/logout-all" "$maria_access" '{"system":"rain-saver"}'; check "maria logout-all" 204
assert "maria's old token rejected by rain-saver within ${REVOCATION_WAIT_SECS}s" wait_for_status "$RS/enderecos" "$maria_access" 401
sleep 1
login "$MARIA_EMAIL" "$USER_PASS" rain-saver; check "maria logs in again" 200
req GET "$RS/enderecos" "$access"; check "maria's new token works" 200

echo
echo "passed: $pass  failed: $fail"
[[ $fail -eq 0 ]]
