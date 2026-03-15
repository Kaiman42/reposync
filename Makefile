# Detecção de OS e Variáveis de Caminhos
ifeq ($(OS),Windows_NT)
    PLATFORM := windows
    PREFIX ?= $(USERPROFILE)\.local
    BINDIR = $(PREFIX)\bin
    SHAREDIR = $(PREFIX)\share\reposync
    DESKTOP = $(USERPROFILE)\Desktop
    STARTMENU = $(APPDATA)\Microsoft\Windows\Start Menu\Programs
    RM := del /Q /F
    RMDIR := rmdir /S /Q
    CP = copy /y
    MKDIR = powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path"
    TAGS := 
    FIXPATH = $(subst /,\,$1)
    
    SETUP_ICONS = @$(CP) build\windows\reposync.ico build\windows\icon.ico >nul && \
                  $(CP) build\reposync.png build\appicon.png >nul && \
                  $(CP) build\linux\reposync.svg frontend\icon.svg >nul
    CLEAN_ICONS = -@powershell -NoProfile -Command "\
        $$files = @('build\windows\icon.ico', 'build\appicon.png', 'frontend\icon.svg'); \
        foreach ($$f in $$files) { if (Test-Path $$f) { Remove-Item $$f -Force } }"
else
    PLATFORM := linux
    PREFIX ?= $(HOME)/.local
    BINDIR = $(PREFIX)/bin
    APPDIR = $(PREFIX)/share/applications
    ICONDIR = $(PREFIX)/share/icons/hicolor
    SCALABLE_DIR = $(ICONDIR)/scalable/apps
    PNGICON_DIR = $(ICONDIR)/512x512/apps
    SHAREDIR = $(PREFIX)/share/reposync
    RM := rm -f
    RMDIR := rm -rf
    TAGS := -tags webkit2_41
    CP = cp
    FIXPATH = $1
    
    SETUP_ICONS = @$(CP) build/reposync.png build/appicon.png
    CLEAN_ICONS = -@$(RM) build/appicon.png
endif

.PHONY: all help build build-linux build-windows clean dev dev-internal install uninstall

all: help

help:
	@echo "========================================="
	@echo "        RepoSync Build System            "
	@echo "========================================="
	@echo "Sistema detectado: $(PLATFORM)"
	@echo "Comandos disponíveis:"
	@echo "  make build    - Compila para todas as plataformas"
	@echo "  make dev      - Inicia modo desenvolvimento (com auto-elevação)"
	@echo "  make install  - Instala no sistema e cria atalhos"
	@echo "  make clean    - Limpa arquivos de build"

build: build-linux build-windows

build-linux:
	@echo "Construindo para Linux..."
	$(SETUP_ICONS)
	wails build -platform linux/amd64 -tags webkit2_41 -clean
	$(CLEAN_ICONS)

build-windows:
	@echo "Construindo para Windows..."
	$(SETUP_ICONS)
	wails build -platform windows/amd64 -clean
	$(CLEAN_ICONS)

dev:
	$(MAKE) dev-internal

dev-internal:
	@echo "Iniciando servidor de desenvolvimento..."
	$(SETUP_ICONS)
	-wails dev $(TAGS)
	$(CLEAN_ICONS)

clean:
	@$(RMDIR) build/bin frontend/wailsjs 2>nul || true
	$(CLEAN_ICONS)

install:
ifeq ($(OS),Windows_NT)
	@echo "Instalando RepoSync no Windows..."
	$(MAKE) build-windows
	@$(MKDIR) "$(BINDIR)"
	@$(MKDIR) "$(SHAREDIR)"
	@$(CP) build\bin\reposync.exe "$(BINDIR)\reposync.exe"
	@$(CP) build\windows\reposync.ico "$(SHAREDIR)\reposync.ico"
	
	@echo "Limpando atalhos antigos..."
	@powershell -NoProfile -Command "\
		$$desktop = [Environment]::GetFolderPath('Desktop'); \
		$$startMenu = [System.IO.Path]::Combine([Environment]::GetFolderPath('StartMenu'), 'Programs'); \
		if (Test-Path \"$$desktop\RepoSync.lnk\") { Remove-Item \"$$desktop\RepoSync.lnk\" -Force }; \
		if (Test-Path \"$$startMenu\RepoSync.lnk\") { Remove-Item \"$$startMenu\RepoSync.lnk\" -Force };"
	
	@echo "Criando atalhos..."
	@powershell -NoProfile -Command "\
		$$ws = New-Object -ComObject WScript.Shell; \
		$$desktop = [Environment]::GetFolderPath('Desktop'); \
		$$startMenu = [System.IO.Path]::Combine([Environment]::GetFolderPath('StartMenu'), 'Programs'); \
		\
		foreach ($$path in @(\"$$desktop\RepoSync.lnk\", \"$$startMenu\RepoSync.lnk\")) { \
			$$s = $$ws.CreateShortcut($$path); \
			$$s.TargetPath = '$(BINDIR)\reposync.exe'; \
			$$s.Arguments = 'dashboard'; \
			$$s.IconLocation = '$(SHAREDIR)\reposync.ico,0'; \
			$$s.Save(); \
		}"
	@echo "Instalação concluída!"
else
	$(MAKE) build-linux
	@echo "Instalando RepoSync no Linux..."
	@mkdir -p $(BINDIR) $(SHAREDIR) $(ICONDIR)
	install -m 755 build/bin/reposync $(BINDIR)/reposync
	@echo "Instalação concluída!"
endif

uninstall:
ifeq ($(OS),Windows_NT)
	@echo "Desinstalando RepoSync..."
	-@if exist "$(BINDIR)\reposync.exe" $(RM) "$(BINDIR)\reposync.exe"
	-@powershell -NoProfile -Command "\
		$$desktop = [Environment]::GetFolderPath('Desktop'); \
		$$startMenu = [System.IO.Path]::Combine([Environment]::GetFolderPath('StartMenu'), 'Programs'); \
		if (Test-Path \"$$desktop\RepoSync.lnk\") { Remove-Item \"$$desktop\RepoSync.lnk\" -Force }; \
		if (Test-Path \"$$startMenu\RepoSync.lnk\") { Remove-Item \"$$startMenu\RepoSync.lnk\" -Force };"
	@echo "Desinstalação concluída!"
else
	@echo "Desinstalando RepoSync..."
	rm -f $(BINDIR)/reposync
	@echo "Desinstalação concluída!"
endif
