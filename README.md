# RepoSync

Script para exibir ícones no Dolphin indicando o status Git dos repositórios nas pastas `/home/kaiman/Repos/Meus/` e `/home/kaiman/Repos/Terceiros/`.

## Como usar

Execute o script `reposync.py` para atualizar os ícones das pastas baseados no status Git.

```bash
./reposync.py
```

## Status e Ícones

- **not_init**: Pasta não é um repositório Git. Ícone: `folder-red`.
- **clean**: Repositório inicializado, sem mudanças pendentes. Ícone: `folder-green`.
- **staged**: Há arquivos staged para commit. Ícone: `folder-blue`.
- **modified**: Há mudanças não staged. Ícone: `folder-yellow`.
- **untracked**: Há arquivos não rastreados. Ícone: `folder-purple`.

O Dolphin lerá os arquivos `.directory` criados em cada pasta e exibirá os ícones correspondentes.

## Automação

Para executar automaticamente, adicione ao cron ou crie um serviço systemd.

Exemplo para cron (a cada 5 minutos):

```bash
*/5 * * * * /home/kaiman/Repos/Meus/reposync/reposync.py
```

## Dependências

- Python 3
- Git
- Dolphin (para visualizar os ícones)
