APP_NAME := commu-bin
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
MAIN_PKG := ./cmd

.PHONY: help build run swagger test test-unit test-integration clean

help:
	@echo "Available targets:"
	@echo "  make build            Build single binary -> $(BIN_PATH)"
	@echo "  make run              Build and run binary"
	@echo "  make swagger          Generate OpenAPI docs (docs/swagger)"
	@echo "  make test             Run all tests"
	@echo "  make test-unit        Run unit tests"
	@echo "  make test-integration Run integration tests"
	@echo "  make clean            Remove build artifacts"

build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN_PATH) $(MAIN_PKG)

run: build
	./$(BIN_PATH)

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/main.go -o docs/swagger --parseInternal

test:
	go test ./...

test-unit:
	go test ./internal/application/service ./internal/delivery ./internal/domain/entity ./internal/infrastructure/persistence/inmemory

test-integration:
	go test ./internal/integration

clean:
	rm -rf $(BIN_DIR)
