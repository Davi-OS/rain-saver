# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

ENV GOPRIVATE=github.com/Davi-OS/*
RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
# go.mod depends on the private github.com/Davi-OS/go-auth-service module
# (for pkg/authsdk). `go mod download` needs authenticated git access to
# it, provided here via a BuildKit secret-mounted netrc — never baked into
# a COPY'd file or a persisted ENV/ARG, so it never ends up in an image
# layer. Build with:
#   docker build --secret id=netrc,src=$HOME/.netrc -t rain-saver .
RUN --mount=type=secret,id=netrc,target=/root/.netrc go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/rain-saver ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/rain-saver ./rain-saver

USER appuser

EXPOSE 8081

ENTRYPOINT ["./rain-saver"]
