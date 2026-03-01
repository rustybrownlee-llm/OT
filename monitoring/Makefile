BINARY     := monitor
CMD_PATH   := ./cmd/monitor
BIN_DIR    := bin
BINARY_OUT := $(BIN_DIR)/$(BINARY)

.PHONY: build test test-race fmt lint clean help

## build: compile the monitor binary to bin/monitor
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY_OUT) $(CMD_PATH)

## test: run all unit tests
test:
	go test ./...

## test-race: run all unit tests with race detector enabled
test-race:
	go test -race ./...

## fmt: format all Go source files
fmt:
	go fmt ./...

## lint: run go vet on all packages
lint:
	go vet ./...

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## help: display this help message
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
