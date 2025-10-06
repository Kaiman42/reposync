# RepoSync

Script para exibir ícones no Dolphin indicando o status Git dos repositórios nas pastas `/home/kaiman/Repos/Meus/` e `/home/kaiman/Repos/Terceiros/`.

## Como usar

Execute o script `reposync.py` para atualizar os ícones das pastas baseados no status Git.

```bash
./reposync.py
```

## Status e Ícones

- **not_init**: Pasta não é um repositório Git. Ícone: `folder-black`.
- **clean**: Repositório inicializado, sem mudanças pendentes. Ícone: `folder-green`.
- **commit**: Há arquivos para commit (staged ou modificados). Ícone: `folder-yellow`.
- **untracked**: Há arquivos não rastreados. Ícone: `folder-red`.
- **synced**: Repositório sincronizado com remoto (sem ahead/behind e limpo). Ícone: `folder-green`.
- **pending_sync**: Repositório com divergência (ahead/behind) ou mudanças locais. Ícone: `folder-violet`.

Modo de sincronização opcional:
Execute com `--sync-mode` para usar somente `synced` (verde) e `pending_sync` (violeta) ignorando os demais detalhamentos internos. Use `--fetch-remotes` junto se quiser atualizar referências remotas antes de classificar.

Exemplos:
```bash
./bin/reposync.py --sync-mode
./bin/reposync.py --sync-mode --fetch-remotes
```

O Dolphin lerá os arquivos `.directory` criados em cada pasta e exibirá os ícones correspondentes.

### Atualização automática sem F5

Para atualizar ícones automaticamente ao salvar arquivos em qualquer editor nos diretórios monitorados, use o watcher baseado em inotify:

```bash
./bin/watch_repos.py
```

Requer `inotify-tools` (instalado). Ele observa os repositórios e dispara `reposync` com debounce (padrão 400ms) agregando múltiplos saves rápidos.

Variáveis opcionais:
```bash
DEBOUNCE_MS=800 ./bin/watch_repos.py
```

Exemplo systemd (usuário) `~/.config/systemd/user/reposync-watcher.service`:
```
[Unit]
Description=RepoSync Watcher (inotify)
After=graphical-session.target

[Service]
Type=simple
Environment=DEBOUNCE_MS=500
WorkingDirectory=%h/Repos/Meus/reposync/bin
ExecStart=%h/Repos/Meus/reposync/bin/watch_repos.py
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
```
Ativar:
```bash
cp reposync-watcher.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now reposync-watcher.service
```

Isso monitora mudanças em arquivos dentro dos repos e atualiza ícones automaticamente.

Para VS Code específico, configure "Run On Save" como acima, mas o watcher cobre qualquer editor.

## Automação

Para executar automaticamente, adicione ao cron ou crie um serviço systemd.

Exemplo para cron (a cada 5 minutos):

```bash
*/5 * * * * /home/kaiman/Repos/Meus/reposync/reposync.py
```

### Reinstalação automática de hooks

Os hooks do Git (post-commit, post-merge, etc.) chamam o `reposync.py` para atualizar o ícone logo após ações relevantes.
Agora há um controle de versão dos hooks:

- A versão atual está em `version.py` (`HOOK_VERSION`).
- Cada hook gerado contém uma linha `# hook-version: X.Y.Z`.
- Se você modificar o template dos hooks (em `install_hooks.py`) e incrementar a versão, pode forçar a reinstalação.

Hooks instalados: `post-commit`, `post-merge`, `post-checkout`, `post-rewrite`, `post-applypatch`, `post-reset`, `post-update`, `post-switch`.

Formas de atualizar:

1. Manual (todos os repositórios padrão):
	```bash
	./install_hooks.py
	```
2. Forçar (mesmo se versão igual):
	```bash
	./install_hooks.py --force
	```
3. Apenas repositórios específicos:
	```bash
	./install_hooks.py /caminho/do/repo1 /caminho/do/repo2
	```
4. Reinstalar ao rodar sincronização (verificando versão):
	```bash
	./reposync.py --ensure-hooks
	```
5. Forçar via reposync:
	```bash
	./reposync.py --ensure-hooks --force-hooks
	```

Assim, sempre que alterar a lógica dos hooks é só atualizar `HOOK_VERSION` e rodar um dos comandos acima.

## Integração com o Editor (Atualização ao Salvar)

Para atualizar o ícone logo após salvar arquivos (antes mesmo de um commit), foi adicionado o script `on_save_update.py`.

### VS Code
1. Instale a extensão "Run On Save" (emeraldwalk.runonsave).
2. O arquivo `.vscode/settings.json` já contém:
	 ```json
	 {
		 "runOnSave.commands": [
			 { "match": ".*", "cmd": "python3 ${workspaceFolder}/on_save_update.py ${file}" }
		 ]
	 }
	 ```
3. Ao salvar qualquer arquivo pertencente a um repositório Git sob as pastas monitoradas, somente aquele repositório é atualizado (chamada rápida com `--ensure-hooks -q`).

### Neovim (exemplo autocmd Lua)
```lua
vim.api.nvim_create_autocmd({"BufWritePost"}, {
	pattern = "*",
	callback = function(args)
		vim.fn.jobstart({"python3", "/home/kaiman/Repos/Meus/reposync/on_save_update.py", args.file}, {detach=true})
	end
})
```

### JetBrains (IDEA / PyCharm)
Use File Watchers:
1. Settings > Tools > File Watchers > + (Custom)
2. Program: `python3`
3. Arguments: `/home/kaiman/Repos/Meus/reposync/on_save_update.py $FilePath$`
4. Working dir: `$ProjectFileDir$`
5. Desmarcar “Immediate Sync” se quiser agrupar.

### Debounce / Performance
O script já de-duplifica múltiplos arquivos do mesmo repositório no mesmo disparo. Se salvar muitos arquivos muito rápido e quiser agrupar mais, pode-se implementar um pequeno cache com timestamp (fale se quiser que eu adicione).

### Limitações
- Alterações que não tocam disco (stash/pop, reset) ainda dependem dos hooks ou execução manual.
- Se o arquivo salvo estiver fora de um repositório Git, nada acontece.

## Dependências

- Python 3
- Git
- Dolphin (para visualizar os ícones)
# test change
