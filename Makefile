VERSION := 0.1.0
CLI_NAME := cli
SERVER_NAME := server

build-cli:
	go build -o build/cli/$(CLI_NAME) ./cmd/cli/cli.go

build-server:
	go build -o build/server/$(SERVER_NAME) ./cmd/server/*.go

build-all: build-cli build-server

clean:
	rm -rf build/*


client:
	./build/cli/$(CLI_NAME)
