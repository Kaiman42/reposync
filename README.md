# RepoSync (Go Edition) 🚀

Versão em Go do RepoSync, agora como um único binário ultra-rápido e sem dependências externas (como Python).

## Funcionalidades

- **Ícones dinâmicos**: Altera o ícone da pasta no Dolphin (Linux) ou Windows Explorer para refletir o status do Git.
- **Watcher em tempo real**: Monitora mudanças nos arquivos e atualiza os ícones instantaneamente (com debounce).
- **Sem dependências**: Basta baixar o binário e rodar. Não precisa de Python ou pacotes extras.
- **Multiplataforma**: Funciona perfeitamente no Fedora 43 (KDE/Plasma) e Windows.

## Estrutura do Projeto

O projeto está organizado por sistema operacional:
- `windows/`: Contém o binário `.exe`, os ícones customizados e a lógica de UI do Windows.
- `linux/`: Contém o binário para Linux, o serviço systemd e a lógica de UI para Linux.
- Raiz: Código core em Go, configurações e documentação.

## Uso no Windows

Apenas rode:
```powershell
.\windows\reposync.exe setup
```

## Uso no Linux (Fedora)

Apenas rode:
```bash
./linux/reposync setup
```

## Compilação (Desenvolvimento)

Se você quiser compilar manualmente:

```bash
# Para Windows
go build -o reposync.exe .

# Para Linux
GOOS=linux GOARCH=amd64 go build -o reposync .
```

## Status dos Ícones

- `clean`: Verde (Sincronizado e limpo)
- `commit`: Amarelo (Mudanças pendentes para commit)
- `untracked`: Vermelho (Arquivos novos não rastreados)
- `pending_sync`: Violeta (Commits locais não enviados ou mudanças remotas)
- `not_init`: Preto (Não é um repositório git)
