VERSION := 0.1.0
CLI_NAME := cinder
SERVER_NAME := server

build-cli:
	go build -o build/cli/$(CLI_NAME) ./cmd/cli/

build-server:
	go build -o build/server/$(SERVER_NAME) ./cmd/server/

build-all: build-cli build-server

clean:
	rm -rf build/*

client:
	./build/cli/$(CLI_NAME)

