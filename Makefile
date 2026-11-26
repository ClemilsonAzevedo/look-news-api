GO ?= go
BINARY_NAME ?= look-news
MAIN_PKG ?= ./cmd/web
BIN_DIR ?= bin
BINARY := $(BIN_DIR)/$(BINARY_NAME)

.DEFAULT_GOAL := help

.PHONY: help run build test clean fmt fmt-check vet lint check tidy download

help: ## Exibe os comandos disponíveis
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Inicia a aplicação localmente
	$(GO) run $(MAIN_PKG)

build: ## Compila a aplicação no diretório configurado
	@mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(BINARY)" $(MAIN_PKG)

test: ## Executa os testes de todos os pacotes
	$(GO) test ./...

clean: ## Remove o binário gerado pelo build
	@rm -f "$(BINARY)"

fmt: ## Formata o código Go
	$(GO) fmt ./...

fmt-check: ## Verifica a formatação sem alterar arquivos
	@files="$$(gofmt -l $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .XTestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...))"; \
	if [ -n "$$files" ]; then \
		printf 'Arquivos Go precisam de formatação:\n%s\n' "$$files"; \
		exit 1; \
	fi

vet: ## Executa a análise estática com go vet
	$(GO) vet ./...

lint: fmt-check vet ## Verifica a formatação e executa go vet

check: test lint ## Executa testes e verificações estáticas

tidy: ## Organiza as dependências do módulo
	$(GO) mod tidy

download: ## Baixa as dependências do módulo
	$(GO) mod download
