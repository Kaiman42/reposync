#!/usr/bin/env python3
"""Script para ser chamado pelo editor no evento de salvar arquivo.
Recebe um caminho de arquivo e descobre o repositório Git raiz.
Atualiza somente aquele repositório via reposync e força refresh do Dolphin.
Uso típico (VS Code runOnSave):
  python /home/kaiman/Repos/Meus/reposync/on_save_update.py ${file}
"""
import os
import sys
import subprocess

REPOSYNC = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'reposync.py')

def find_git_root(path: str):
    path = os.path.abspath(path)
    candidate = os.path.dirname(path) if os.path.isfile(path) else path
    while True:
        if os.path.isdir(os.path.join(candidate, '.git')):
            return candidate
        new_candidate = os.path.dirname(candidate)
        if new_candidate == candidate:
            return None
        candidate = new_candidate

def update_repo(repo: str):
    subprocess.run([REPOSYNC, repo, '--ensure-hooks', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def refresh_dolphin():
    # Força refresh do Dolphin via qdbus
    qdbus = subprocess.run(['which', 'qdbus'], capture_output=True, text=True)
    if qdbus.returncode != 0:
        return
    qdbus_bin = qdbus.stdout.strip()
    cmds = [
        [qdbus_bin, 'org.kde.dolphin', '/dolphin/Dolphin_1', 'refresh'],
        [qdbus_bin, 'org.kde.dolphin', '/dolphin/Dolphin_1', 'org.qtproject.Qt.QWidget.update']
    ]
    for cmd in cmds:
        try:
            subprocess.run(cmd, timeout=1, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except:
            pass

def main(argv):
    files = [a for a in argv if not a.startswith('-') and a.strip()]
    repos = set()
    for f in files:
        root = find_git_root(f)
        if root:
            repos.add(root)
    for repo in repos:
        update_repo(repo)
    refresh_dolphin()

if __name__ == '__main__':
    main(sys.argv[1:])
