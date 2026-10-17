#!/usr/bin/env bash
# End-to-end checks of go-auth-service <-> rain-saver on the local stack
# (docker-compose.yml in this directory). Needs curl and jq.
#
#   ./build.sh && docker compose up --build -d && ./e2e.sh
#
# Idempotent: can be re-run against the same stack.
set -uo pipefail

AUTH=${AUTH_URL:-http://localhost:8080}
RS=${RAIN_SAVER_URL:-http://localhost:8081}
DEV_EMAIL=${DEVSEED_EMAIL:-dev@rainsaver.local}
DEV_PASS=${DEVSEED_PASSWORD:-rainsaver-dev-pass}
MARIA_EMAIL=maria@rainsaver.local
MARIA_PASS=maria-pass-123
# Revocations reach rain-saver via polling (SDK default: every 5s).
REVOCATION_WAIT_SECS=${REVOCATION_WAIT_SECS:-10}
# Front-end origin expected in CORS_ALLOWED_ORIGINS (Vite dev server).
FRONT_ORIGIN=${FRONT_ORIGIN:-http://localhost:5173}

here="$(cd "$(dirname "$0")" && pwd)"
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
  local org_field=""
  [[ -n ${4:-} ]] && org_field=",\"org\":\"$4\""
  req POST "$AUTH/login" "" "{\"email\":\"$1\",\"password\":\"$2\",\"system\":\"$3\"$org_field}"
  access=$(jq -r '.access_token // empty' <<<"$body")
  refresh=$(jq -r '.refresh_token // empty' <<<"$body")
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

echo "== health"
req GET "$AUTH/.well-known/jwks.json"; check "auth JWKS" 200
req GET "$RS/healthz"; check "rain-saver /healthz" 200

echo "== CORS for the front-end ($FRONT_ORIGIN)"
preflight() { # preflight URL ORIGIN METHOD -> prints the Access-Control-Allow-Origin value
  curl -s -o /dev/null -D - -X OPTIONS "$1" -H "Origin: $2" \
    -H "Access-Control-Request-Method: $3" -H 'Access-Control-Request-Headers: authorization,content-type' |
    tr -d '\r' | awk -F': ' 'tolower($1) == "access-control-allow-origin" {print $2}'
}
assert "auth preflight POST /login allows the front" test "$(preflight "$AUTH/login" "$FRONT_ORIGIN" POST)" = "$FRONT_ORIGIN"
assert "rain-saver preflight POST /enderecos allows the front" test "$(preflight "$RS/enderecos" "$FRONT_ORIGIN" POST)" = "$FRONT_ORIGIN"
assert "rain-saver preflight refuses other origins" test -z "$(preflight "$RS/enderecos" https://evil.example POST)"
acao=$(curl -s -o /dev/null -D - "$RS/enderecos" -H "Origin: $FRONT_ORIGIN" | tr -d '\r' | awk -F': ' 'tolower($1) == "access-control-allow-origin" {print $2}')
assert "rain-saver 401 still carries CORS headers (front can read the error)" test "$acao" = "$FRONT_ORIGIN"

echo "== login (rain-saver audience)"
login "$DEV_EMAIL" "$DEV_PASS" rain-saver; check "dev logs in for rain-saver" 200
dev_access=$access; dev_refresh=$refresh
c=$(claims "$dev_access")
org=$(jq -r .org <<<"$c")
assert "iss = auth-service" test "$(jq -r .iss <<<"$c")" = auth-service
assert "aud = [rain-saver]" test "$(jq -c .aud <<<"$c")" = '["rain-saver"]'
assert "org claim present" test -n "$org" -a "$org" != null
assert "perms carry endereco:write" jqok '.perms | index("endereco:write")' <<<"$c"
req GET "$AUTH/me" "$dev_access"; check "/me accepts a rain-saver token" 200

echo "== user management (admin creates user, no Google)"
login "$DEV_EMAIL" "$DEV_PASS" auth "$org"; check "dev logs in for auth (admin API)" 200
admin_access=$access
req POST "$AUTH/orgs/$org/users" "$admin_access" \
  "{\"email\":\"$MARIA_EMAIL\",\"password\":\"$MARIA_PASS\",\"role\":\"member\",\"system\":\"rain-saver\"}"
if [[ $status == 409 ]]; then status=201; fi # re-run: already created
check "owner creates maria (rain-saver member)" 201
login "$MARIA_EMAIL" "$MARIA_PASS" rain-saver; check "maria logs in for rain-saver" 200
maria_access=$access
assert "maria's token has role member" test "$(claims "$maria_access" | jq -c .roles)" = '["member"]'

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
login "$MARIA_EMAIL" "$MARIA_PASS" rain-saver; check "maria logs in again" 200
req GET "$RS/enderecos" "$access"; check "maria's new token works" 200

echo "== auth restart keeps tokens valid (stable signing key volume)"
login "$DEV_EMAIL" "$DEV_PASS" rain-saver
before_restart=$access
(cd "$here" && docker compose restart auth >/dev/null 2>&1)
deadline=$((SECONDS + 30))
until curl -sf "$AUTH/.well-known/jwks.json" >/dev/null || (( SECONDS > deadline )); do sleep 0.5; done
req GET "$RS/enderecos" "$before_restart"; check "pre-restart token still accepted by rain-saver" 200
login "$DEV_EMAIL" "$DEV_PASS" rain-saver; check "login after restart" 200
req GET "$RS/enderecos" "$access"; check "post-restart token accepted by rain-saver" 200

echo
echo "passed: $pass  failed: $fail"
[[ $fail -eq 0 ]]
