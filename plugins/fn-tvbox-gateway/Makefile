.PHONY: run test build

VERSION ?= $(shell cat ../../VERSION 2>/dev/null || echo dev)

run:
	go run -ldflags "-X main.version=$(VERSION)" ./cmd/gateway

test:
	go test ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/fn-tvbox-gateway ./cmd/gateway
