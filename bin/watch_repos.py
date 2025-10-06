#!/usr/bin/env python3
"""Watch repos for file changes and trigger reposync incrementally.

Requer: inotify-tools (inotifywait no PATH).

Uso:
  ./watch_repos.py            # observa caminhos padrão (base_paths do reposync)
  ./watch_repos.py /caminho/extra

Env vars:
  DEBOUNCE_MS=400  (janela de agregação por repo)

Saída: logs simples no stdout.
"""
import os
import sys
import time
import subprocess
import threading

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPOSYNC = os.path.join(SCRIPT_DIR, 'reposync.py')

DEFAULT_BASES = ['/home/kaiman/Repos/Meus', '/home/kaiman/Repos/Terceiros']

DEBOUNCE_MS = int(os.environ.get('DEBOUNCE_MS', '2000'))

def log(msg):
    print(msg, flush=True)

def find_repos(bases):
    repos = []
    seen = set()
    for base in bases:
        if not os.path.isdir(base):
            continue
        for name in os.listdir(base):
            p = os.path.join(base, name)
            if os.path.isdir(os.path.join(p, '.git')) and p not in seen:
                repos.append(p)
                seen.add(p)
    return repos

def repo_root(path):
    path = os.path.abspath(path)
    cur = path
    while True:
        if os.path.isdir(os.path.join(cur, '.git')):
            return cur
        parent = os.path.dirname(cur)
        if parent == cur:
            return None
        cur = parent

def run_reposync(repo):
    subprocess.run([REPOSYNC, repo, '--ensure-hooks', '--sync-mode', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def watcher_loop(paths):
    # Monta comando inotifywait recursivo para os paths
    cmd = ['inotifywait', '-m', '-r', '-e', 'modify,attrib,close_write,create,delete,move'] + paths
    try:
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    except FileNotFoundError:
        print('Erro: inotifywait não encontrado.', file=sys.stderr)
        sys.exit(1)

    pending = set()
    last_run = {}
    lock = threading.Lock()

    def debounce_thread():
        while True:
            time.sleep(0.2)
            now = time.time()
            to_run = []
            with lock:
                for repo in list(pending):
                    if repo not in last_run or now - last_run.get(repo, 0) >= DEBOUNCE_MS / 1000.0:
                        to_run.append(repo)
                        last_run[repo] = now
                pending.clear()
            for r in to_run:
                log(f"reposync {r}")
                run_reposync(r)

    threading.Thread(target=debounce_thread, daemon=True).start()

    stream = proc.stdout
    if stream is None:
        print('stdout do inotifywait não disponível', file=sys.stderr)
        return
    for line in stream:
        line = line.strip()
        if not line:
            continue
        parts = line.split(None, 3)
        base = parts[0]
        full_path = base
        repo = repo_root(full_path)
        if not repo:
            continue
        with lock:
            pending.add(repo)

def main(argv):
    bases = argv if argv else DEFAULT_BASES
    repos = find_repos(bases)
    paths = repos  # observar somente root de cada repo
    if not paths:
        print('Nenhum repositório encontrado.', file=sys.stderr)
        return 1
    print(f'Observando {len(paths)} repositórios. Debounce {DEBOUNCE_MS}ms.')
    watcher_loop(paths)
    return 0

if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))