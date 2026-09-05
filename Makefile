.PHONY: all build build-cli build-server build-bench build-all \
	test race run smoke bench clean client help

VERSION := 0.1.0
CLI_NAME := cinder
SERVER_NAME := server
BENCH_NAME := bench

help:
	@echo "Targets: build-all test race run smoke bench clean client"

all: test

build-cli:
	go build -o build/cli/$(CLI_NAME) ./cmd/cli/

build-server:
	go build -o build/server/$(SERVER_NAME) ./cmd/server/

build-bench:
	go build -o build/bench/$(BENCH_NAME) ./cmd/bench/

build-all: build-cli build-server build-bench

# Default gates
test:
	go test ./...

race:
	go test -race ./...

# Run the server in the foreground (override with CINDER_* env vars).
run: build-server
	./build/server/$(SERVER_NAME)

# Brief CSP PING against an ephemeral server (self-contained).
smoke: build-server build-cli
	@bash scripts/smoke.sh

# Requires a running server (default 127.0.0.1:9573), or start one via `make run`.
bench: build-bench
	./build/bench/$(BENCH_NAME) -n 2000 -c 4 -workload mixed

clean:
	rm -rf build/*

client: build-cli
	./build/cli/$(CLI_NAME)
