# Makefile for Distributed Key-Value Store (DKV)

.PHONY: all build test clean proto docker-up docker-down

all: build test

build:
	go build -o bin/kvsrvd ./cmd/kvsrvd
	go build -o bin/kvraftd ./cmd/kvraftd
	go build -o bin/shardctrlrd ./cmd/shardctrlrd
	go build -o bin/dkv-client ./cmd/dkv-client

test:
	go test -v -race ./internal/...

clean:
	rm -rf bin/
	go clean

# Docker helper targets
docker-up:
	docker compose -f ../docker-compose.yml up --build -d

docker-down:
	docker compose -f ../docker-compose.yml down -v
