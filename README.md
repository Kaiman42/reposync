# RepoSync

Script para exibir ícones no Dolphin indicando o status Git dos repositórios nas pastas `/home/kaiman/Repos/Meus/` e `/home/kaiman/Repos/Terceiros/`.

## Como usar

Execute o script `reposync.py` para atualizar os ícones das pastas baseados no status Git.

```bash
./bin/reposync.py
```

## Status e Ícones

- **not_init**: Pasta não é um repositório Git. Ícone: `folder-black`.
- **clean**: Repositório inicializado, sem mudanças pendentes. Ícone: `folder-green`.
- **commit**: Há arquivos para commit (staged ou modificados). Ícone: `folder-yellow`.
- **untracked**: Há arquivos não rastreados. Ícone: `folder-red`.
- **synced**: Repositório sincronizado com remoto (sem ahead/behind e limpo). Ícone: `folder-green`.
- **pending_sync**: Repositório com divergência (ahead/behind) ou mudanças locais. Ícone: `folder-violet`.

O Dolphin lerá os arquivos `.directory` criados em cada pasta e exibirá os ícones correspondentes.

## Instalação e Uso - Modo Contínuo

Para que o projeto rode a **todo momento** e atualize cores das pastas automaticamente:

```bash
./rs.sh install --user
```

Isso instala o serviço systemd que roda o watcher continuamente. Use `./rs.sh` para:
- `install [--user|--system]` — Instalar serviço
- `uninstall [--user|--system]` — Remover serviço
- `status [--user|--system]` — Ver status
- `restart [--user|--system]` — Reiniciar
- `stop [--user|--system]` — Parar

## Atualização de Cores/Ícones em Tempo Real

A atualização de ícones das pastas ocorre em **duas camadas** para máxima cobertura:

### 1. Via Watcher Systemd (Recomendado - Principal)
Ao instalar com `./rs.sh install --user`, o serviço `watch_repos.py` roda continuamente
via `inotifywait`, monitora mudanças em arquivos e atualiza ícones com debounce.
**Esta é a forma primária e recomendada.**

O watcher também envia **notificações desktop**:
- **Imediata**: Quando um repositório é sincronizado e há commits pendentes
- **Periódica**: A cada 12 horas, uma notificação consolidada com o total de commits aguardando sync em todos os repositórios

Requer `notify-send` instalado (geralmente já vem em desktops Linux).

### 2. Via Hooks Git (Automático)
Os hooks instalados por `install_hooks.py` chamam `reposync.py` automaticamente após:
- `post-commit`, `post-merge`, `post-checkout`, `post-rewrite`, `post-applypatch`, `post-reset`, `post-update`, `post-switch`

Isso garante que os ícones se atualizem logo após operações Git.

### Resumo da Arquitetura
- **Watcher (systemd)**: Cobertura contínua e central via inotify (sempre ativo)
- **Hooks Git**: Captura ações Git imediatas (commit, merge, etc.)

Ambas as camadas trabalham juntas para manter os ícones sempre atualizados.## Reinstalação automática de hooks

Os hooks do Git (post-commit, post-merge, etc.) chamam o `reposync.py` para atualizar o ícone logo após ações relevantes.
Há um controle de versão dos hooks:

- A versão atual está em `install_hooks.py` (`HOOK_VERSION`).
- Cada hook gerado contém uma linha `# hook-version: X.Y.Z`.
- Se você modificar o template dos hooks e incrementar a versão, pode forçar a reinstalação.

Formas de atualizar:

1. Manual (todos os repositórios padrão):
```bash
./bin/install_hooks.py
```
2. Forçar (mesmo se versão igual):
```bash
./bin/install_hooks.py --force
```
3. Apenas repositórios específicos:
```bash
./bin/install_hooks.py /caminho/do/repo1 /caminho/do/repo2
```
4. Reinstalar ao rodar sincronização (verificando versão):
```bash
./bin/reposync.py --ensure-hooks
```
5. Forçar via reposync:
```bash
./bin/reposync.py --ensure-hooks --force-hooks
```

## Dependências

- Python 3
- Git
- Dolphin (para visualizar os ícones)
- inotify-tools (para o watcher em tempo real)
