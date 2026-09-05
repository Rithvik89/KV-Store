VERSION := 0.1.0
CLI_NAME := cinder
SERVER_NAME := server
BENCH_NAME := bench

build-cli:
	go build -o build/cli/$(CLI_NAME) ./cmd/cli/

build-server:
	go build -o build/server/$(SERVER_NAME) ./cmd/server/

build-bench:
	go build -o build/bench/$(BENCH_NAME) ./cmd/bench/

build-all: build-cli build-server build-bench

clean:
	rm -rf build/*

client:
	./build/cli/$(CLI_NAME)

# Requires a running server (default 127.0.0.1:9573).
bench: build-bench
	./build/bench/$(BENCH_NAME) -n 2000 -c 4 -workload mixed
