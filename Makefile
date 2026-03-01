PLANT_DIR      := plant
MONITORING_DIR := monitoring

.PHONY: build test test-race fmt lint clean help

## build: build both plant and monitoring binaries
build:
	$(MAKE) -C $(PLANT_DIR) build
	$(MAKE) -C $(MONITORING_DIR) build

## test: run all tests across both modules
test:
	$(MAKE) -C $(PLANT_DIR) test
	$(MAKE) -C $(MONITORING_DIR) test

## test-race: run all tests with race detector across both modules
test-race:
	$(MAKE) -C $(PLANT_DIR) test-race
	$(MAKE) -C $(MONITORING_DIR) test-race

## fmt: format all Go source files in both modules
fmt:
	$(MAKE) -C $(PLANT_DIR) fmt
	$(MAKE) -C $(MONITORING_DIR) fmt

## lint: run go vet on all packages in both modules
lint:
	$(MAKE) -C $(PLANT_DIR) lint
	$(MAKE) -C $(MONITORING_DIR) lint

## clean: remove build artifacts from both modules
clean:
	$(MAKE) -C $(PLANT_DIR) clean
	$(MAKE) -C $(MONITORING_DIR) clean

## help: display this help message
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
