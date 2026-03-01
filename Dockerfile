# syntax=docker/dockerfile:1.6
FROM golang:1.22-alpine

RUN apk add --no-cache \
    git \
    bash \
    curl \
    build-base

# Create vscode user (DevContainer standard)
RUN addgroup -S vscode && adduser -S vscode -G vscode

WORKDIR /workspace

ENV CGO_ENABLED=0
ENV GOCACHE=/go-cache
ENV GOPATH=/go

USER vscode

CMD ["sh", "-lc", "go test ./... && go run ./cmd/api"]
