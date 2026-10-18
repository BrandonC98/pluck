BINARY := pluck
BIN_DIR := bin

.PHONY: all deps build test test-integration vet fmt fmt-check tidy check clean

all: check build

## deps: install tooling from the Brewfile
deps:
	brew bundle --file=Brewfile

## build: compile the binary into bin/
build:
	go build -o $(BIN_DIR)/$(BINARY) .

## test: run the unit tests
test:
	go test -race ./...

## test-integration: build the binary and run it against testdata/vault
test-integration:
	go test -race -tags integration -count=1 .

## vet: run go vet, including the integration tests
vet:
	go vet -tags integration ./...

## fmt: format the code
fmt:
	gofmt -w .

## fmt-check: fail if any file is not gofmt'd
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Run 'make fmt' on:"; gofmt -l .; exit 1)

## tidy: tidy go.mod and go.sum
tidy:
	go mod tidy

## check: everything CI runs
check: fmt-check vet test test-integration

clean:
	rm -rf $(BIN_DIR)
