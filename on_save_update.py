#!/usr/bin/env python3
"""Script para ser chamado pelo editor no evento de salvar arquivo.
Recebe um caminho de arquivo (ou vários) e descobre o repositório Git raiz.
Atualiza somente aquele repositório via reposync para minimizar custo.
Uso típico (VS Code runOnSave):
  python /home/kaiman/Repos/Meus/reposync/on_save_update.py ${file}
"""
import os
import sys
import subprocess
from typing import Optional

REPOSYNC = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'reposync.py')


def find_git_root(path: str) -> Optional[str]:
    path = os.path.abspath(path)
    if os.path.isdir(path) and not os.path.isfile(path):
        candidate = path
    else:
        candidate = os.path.dirname(path)
    while True:
        if os.path.isdir(os.path.join(candidate, '.git')):
            return candidate
        new_candidate = os.path.dirname(candidate)
        if new_candidate == candidate:
            return None
        candidate = new_candidate


def update_repo(repo: str):
    # Executa reposync apenas para esse repo. Permite verbose via env.
    verbose = os.environ.get('REPOSYNC_VERBOSE') == '1'
    cmd = [REPOSYNC, repo, '--ensure-hooks']
    if not verbose:
        cmd.append('-q')
    subprocess.run(cmd, stdout=subprocess.DEVNULL if not verbose else None, stderr=subprocess.DEVNULL if not verbose else None)


def main(argv):
    # Remover flags desconhecidas que editores possam inserir acidentalmente.
    files = [a for a in argv if not a.startswith('-')]
    repos = set()
    for f in files:
        if not f.strip():
            continue
        root = find_git_root(f)
        if root:
            repos.add(root)
    for repo in repos:
        update_repo(repo)

if __name__ == '__main__':
    main(sys.argv[1:])
