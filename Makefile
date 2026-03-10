.PHONY: all help build dev shortcut

# Detecção de Sistema Operacional
ifeq ($(OS),Windows_NT)
    EXECUTAVEL = build/bin/reposync.exe
    COMANDO_BUILD = wails build
    SISTEMA = Windows
else
    EXECUTAVEL = build/bin/reposync
    COMANDO_BUILD = wails build -tags webkit2_41
    SISTEMA = Linux
endif

help:
	@echo "========================================="
	@echo "        RepoSync Build System            "
	@echo "========================================="
	@echo "Sistema detectado: $(SISTEMA)"
	@echo "Comandos disponíveis:"
	@echo "  make build    - Compila o executável oficial"
	@echo "  make dev      - Inicia o modo de desenvolvimento (wails dev)"
	@echo "  make shortcut - Compila e cria o atalho no Desktop"
	@echo ""
	@echo "AVISO: No Windows, certifique-se de usar o Git Bash ou Mingw para rodar o 'make'."

build:
	@echo "Compilando o RepoSync para $(SISTEMA)..."
	$(COMANDO_BUILD)

dev:
	@echo "Iniciando o modo de desenvolvimento..."
	wails dev

shortcut: build
	@echo "Criando os atalhos no sistema $(SISTEMA)..."
	./$(EXECUTAVEL) create-shortcut
