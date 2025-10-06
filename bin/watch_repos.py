#!/usr/bin/env python3
import os
import sys
import time
import subprocess
import threading

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPOSYNC = os.path.join(SCRIPT_DIR, 'reposync.py')
DEFAULT_BASES = ['/home/kaiman/Repos/Meus', '/home/kaiman/Repos/Terceiros']
DEBOUNCE_MS = int(os.environ.get('DEBOUNCE_MS', '2000'))

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
    while True:
        if os.path.isdir(os.path.join(path, '.git')):
            return path
        parent = os.path.dirname(path)
        if parent == path:
            return None
        path = parent

def run_reposync(repo):
    subprocess.run([REPOSYNC, repo, '--ensure-hooks', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def watcher_loop(paths):
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
                print(f"reposync {r}", flush=True)
                run_reposync(r)

    threading.Thread(target=debounce_thread, daemon=True).start()

    try:
        while True:
            line = proc.stdout.readline()
            if not line:
                break
            if not line.strip():
                continue
            parts = line.split(None, 3)
            if parts:
                repo = repo_root(parts[0])
                if repo:
                    with lock:
                        pending.add(repo)
    except KeyboardInterrupt:
        proc.terminate()
        return

def main(argv):
    bases = argv if argv else DEFAULT_BASES
    repos = find_repos(bases)
    if not repos:
        print('Nenhum repositório encontrado.', file=sys.stderr)
        return 1
    print(f'Observando {len(repos)} repositórios. Debounce {DEBOUNCE_MS}ms.')
    watcher_loop(repos)
    return 0

if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))