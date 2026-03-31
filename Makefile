.PHONY: build install test lint update deps

VERSION ?= 0.5.4
BIN_NAME = omnix

build:
	go build -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BIN_NAME) .

install: build
	go install -ldflags "-s -w -X main.Version=$(VERSION)" .

test:
	go test ./...

lint:
	gofmt -s -w .
	go vet ./...

update:
	go get -u ./...
	go mod tidy
	go mod vendor

deps:
	go mod download
	go mod tidy

clean:
	rm -f $(BIN_NAME)
	rm -rf vendor/
	rm -rf .nix/

nix-build:
	nix build .#$(BIN_NAME)

nix-install:
	nix profile install .
