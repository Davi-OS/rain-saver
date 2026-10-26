#!/usr/bin/env bash
# Builds the local-stack binary on the host (static, linux) into .build/bin.
#
# Why on the host: a Docker build here would need network access for
# `go mod download`, which failed intermittently on Docker's default DNS.
# The host already has both the toolchain and the module cache.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
rs_dir="$(cd "$here/../.." && pwd)"
out="$here/.build"
mkdir -p "$out/bin"
rm -f "$out/bin/auth-service" "$out/bin/devseed" "$out/rain-saver.mod" "$out/rain-saver.sum" # leftovers of the old stack

export CGO_ENABLED=0 GOOS=linux

echo "building rain-saver from $rs_dir"
(cd "$rs_dir" && go build -o "$out/bin/rain-saver" ./cmd/api)

cp "$here/Dockerfile.bin" "$out/Dockerfile"
echo "ok: $(ls "$out/bin" | tr '\n' ' ')"
