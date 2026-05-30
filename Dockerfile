# syntax=docker/dockerfile:1.6
FROM golang:1.22-alpine

RUN apk add --no-cache \
    git \
    bash \
    curl \
    build-base

# Create vscode user (DevContainer standard)
RUN addgroup -S vscode && adduser -S vscode -G vscode
RUN mkdir -p /go /go-cache && chown -R vscode:vscode /go /go-cache

ENV CGO_ENABLED=0
ENV GOCACHE=/go-cache
ENV GOPATH=/go

WORKDIR /workspace/project
COPY project/go.mod project/go.sum ./

USER vscode

RUN /usr/local/go/bin/go mod download

WORKDIR /workspace

CMD ["sh", "-lc", "cd project && /usr/local/go/bin/go test ./... && /usr/local/go/bin/go run ./cmd/api"]
