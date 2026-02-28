# syntax=docker/dockerfile:1.6
FROM golang:1.22-alpine

RUN apk add --no-cache git ca-certificates bash curl

WORKDIR /workspace

ENV CGO_ENABLED=0
ENV GOCACHE=/go-cache
ENV GOPATH=/go

CMD ["sh", "-lc", "go test ./... && go run ./cmd/api"]
