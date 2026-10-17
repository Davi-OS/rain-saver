#!/usr/bin/env bash
# Builds the local-stack binaries on the host (static, linux) into .build/bin.
#
# Why on the host: Docker builds here would need network access for
# `go mod download`, and the private go-auth-service module would need GitHub
# credentials. The host already has both the toolchain and the module cache.
#
# rain-saver is built against the LOCAL go-auth-service checkout through a
# temporary modfile with a replace directive; the repo's go.mod/go.sum are
# never modified.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
rs_dir="$(cd "$here/../.." && pwd)"
auth_dir="$(cd "${AUTH_SERVICE_DIR:-$rs_dir/../go-auth-service}" && pwd)"
out="$here/.build"
mkdir -p "$out/bin"

export CGO_ENABLED=0 GOOS=linux

echo "building go-auth-service from $auth_dir"
(cd "$auth_dir" && go build -o "$out/bin/auth-service" ./cmd/api && go build -o "$out/bin/devseed" ./cmd/devseed)

echo "building rain-saver from $rs_dir (go-auth-service replaced by local checkout)"
cp "$rs_dir/go.mod" "$out/rain-saver.mod"
cp "$rs_dir/go.sum" "$out/rain-saver.sum"
(
  cd "$rs_dir"
  go mod edit -replace "github.com/Davi-OS/go-auth-service=$auth_dir" "$out/rain-saver.mod"
  go mod tidy -modfile="$out/rain-saver.mod"
  go build -modfile="$out/rain-saver.mod" -o "$out/bin/rain-saver" ./cmd/api
)

cp "$here/Dockerfile.bin" "$out/Dockerfile"
echo "ok: $(ls "$out/bin" | tr '\n' ' ')"
