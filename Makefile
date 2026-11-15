# Toolbelt Makefile
# Comandos de desenvolvimento para o projeto

.PHONY: all build test lint bench-fake clean install help

# Variáveis
BINARY_NAME := tb
FAKE_AGENT := fake-agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/leonardo-moraes360/toolbelt/internal/version.Version=$(VERSION) \
	-X github.com/leonardo-moraes360/toolbelt/internal/version.Commit=$(COMMIT) \
	-X github.com/leonardo-moraes360/toolbelt/internal/version.BuildTime=$(BUILD_TIME)"

# Diretórios
BIN_DIR := bin
CMD_DIR := cmd

# Cores para output (respeita NO_COLOR)
ifndef NO_COLOR
	GREEN := \033[0;32m
	YELLOW := \033[0;33m
	CYAN := \033[0;36m
	RESET := \033[0m
endif

## all: Build everything
all: build

## build: Compila os binários
build: $(BIN_DIR)/$(BINARY_NAME) $(BIN_DIR)/$(FAKE_AGENT)

$(BIN_DIR)/$(BINARY_NAME): $(shell find . -name '*.go' -not -path './bench/*')
	@echo "$(CYAN)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)/tb

$(BIN_DIR)/$(FAKE_AGENT): $(shell find . -name '*.go' -not -path './bench/*')
	@echo "$(CYAN)Building $(FAKE_AGENT)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(FAKE_AGENT) ./$(CMD_DIR)/fake-agent

## test: Executa testes unitários
test:
	@echo "$(CYAN)Running tests...$(RESET)"
	go test -v -race -coverprofile=coverage.out $$(go list ./... | grep -v 'bench/fixtures')
	@echo "$(GREEN)Tests passed!$(RESET)"

## test-coverage: Mostra cobertura de testes
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(RESET)"

## lint: Executa linter
lint:
	@echo "$(CYAN)Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(RESET)"; \
		exit 1; \
	fi

## fmt: Formata código
fmt:
	@echo "$(CYAN)Formatting code...$(RESET)"
	go fmt ./...
	gofmt -s -w .

## vet: Executa go vet
vet:
	@echo "$(CYAN)Running go vet...$(RESET)"
	go vet ./...

## bench-fake: Executa benchmark com agente fake
bench-fake: build
	@echo "$(CYAN)Running benchmark with fake agent...$(RESET)"
	$(BIN_DIR)/$(BINARY_NAME) bench run --fake -r 1
	@echo "$(GREEN)Benchmark completed!$(RESET)"

## bench-list: Lista tarefas de benchmark disponíveis
bench-list: build
	$(BIN_DIR)/$(BINARY_NAME) bench list

## install: Instala binários em GOPATH/bin
install:
	@echo "$(CYAN)Installing...$(RESET)"
	go install $(LDFLAGS) ./$(CMD_DIR)/tb
	go install ./$(CMD_DIR)/fake-agent
	@echo "$(GREEN)Installed to $(shell go env GOPATH)/bin$(RESET)"

## clean: Remove arquivos gerados
clean:
	@echo "$(CYAN)Cleaning...$(RESET)"
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	rm -rf bench/results/*
	@echo "$(GREEN)Clean!$(RESET)"

## deps: Baixa dependências
deps:
	@echo "$(CYAN)Downloading dependencies...$(RESET)"
	go mod download
	go mod tidy
	@echo "$(GREEN)Dependencies ready!$(RESET)"

## check: Executa todas as verificações (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(GREEN)All checks passed!$(RESET)"

## version: Mostra versão que será compilada
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Build:   $(BUILD_TIME)"

## help: Mostra esta ajuda
help:
	@echo "Toolbelt - Comandos disponíveis:"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
	@echo ""
	@echo "Exemplos:"
	@echo "  make build        # Compila binários"
	@echo "  make test         # Executa testes"
	@echo "  make bench-fake   # Benchmark com agente fake"
	@echo "  make check        # Todas as verificações"

# Default target
.DEFAULT_GOAL := help
