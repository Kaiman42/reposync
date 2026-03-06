.PHONY: all help build dev shortcut

help:
	@echo "========================================="
	@echo "        RepoSync Build System            "
	@echo "========================================="
	@echo "Comandos disponíveis:"
	@echo "  make build    - Compila o executável oficial em build/bin/reposync"
	@echo "  make dev      - Inicia o modo de desenvolvimento ao vivo (wails dev)"
	@echo "  make shortcut - Compila o projeto e cria o atalho no Desktop"
	@echo ""
	@echo "AVISO: O comando 'go build' foi desencorajado para este projeto."
	@echo "Sempre utilize o 'make build' ou 'wails build' para garantir a versão correta."

build:
	@echo "Compilando o RepoSync usando o compilador oficial do Wails (com suporte a WebKit 4.1)..."
	wails build -tags webkit2_41

dev:
	@echo "Iniciando o modo de desenvolvimento..."
	wails dev

shortcut: build
	@echo "Criando os atalhos no sistema..."
	./build/bin/reposync create-shortcut
