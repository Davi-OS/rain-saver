# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Every dependency is public (the auth SDK is github.com/Davi-OS/go-auth-sdk),
# so the build needs no credentials.
COPY go.mod go.sum ./
RUN go mod download

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
