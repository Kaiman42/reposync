#!/usr/bin/env python3
import os
import sys
import time
import subprocess
import threading
import shutil

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPOSYNC = os.path.join(SCRIPT_DIR, 'reposync.py')
DEFAULT_BASES = ['/home/kaiman/Repos/Github/Meus', '/home/kaiman/Repos/Github/Terceiros']
DEBOUNCE_MS = int(os.environ.get('DEBOUNCE_MS', '2000'))
NOTIFICATION_INTERVAL_HOURS = 12  # Notificar a cada 12 horas

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

def count_pending_commits(repo):
    """Conta commits não sincronizados (ahead do remote)."""
    try:
        # Tenta obter o upstream branch
        result = subprocess.run(
            ['git', 'rev-list', '@{u}..HEAD', '--count'],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=5
        )
        if result.returncode == 0:
            return int(result.stdout.strip() or '0')
    except Exception:
        pass
    return 0

def send_notification(title, message, timeout=5000):
    """Envia notificação desktop via notify-send."""
    notify_send = shutil.which('notify-send')
    if not notify_send:
        return False
    try:
        subprocess.run(
            [notify_send, '-t', str(timeout), title, message],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=2
        )
        return True
    except Exception:
        return False

def check_and_notify_pending_commits(repos):
    """Verifica todos os repos e envia notificação consolidada de commits pendentes."""
    pending_repos = {}
    total_commits = 0
    
    for repo in repos:
        pending = count_pending_commits(repo)
        if pending > 0:
            repo_name = os.path.basename(repo)
            pending_repos[repo_name] = pending
            total_commits += pending
    
    if pending_repos:
        # Cria mensagem consolidada
        repo_list = "\n".join([f"  • {name}: {count}" for name, count in pending_repos.items()])
        message = f"Total: {total_commits} commit{'s' if total_commits != 1 else ''} aguardando sync\n\n{repo_list}"
        send_notification("RepoSync - Notificação Periódica", message, timeout=10000)
        print(f"[Notificação] {total_commits} commits aguardando sync em {len(pending_repos)} repositório(s)", flush=True)

def run_reposync(repo):
    subprocess.run([REPOSYNC, repo, '--ensure-hooks', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    
    # Verifica se há commits pendentes e notifica
    pending_commits = count_pending_commits(repo)
    if pending_commits > 0:
        repo_name = os.path.basename(repo)
        msg = f"{pending_commits} commit{'s' if pending_commits != 1 else ''} aguardando sync"
        send_notification(f"RepoSync: {repo_name}", msg)

def watcher_loop(paths, repos):
    cmd = ['inotifywait', '-m', '-r', '-e', 'modify,attrib,close_write,create,delete,move'] + paths
    try:
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    except FileNotFoundError:
        print('Erro: inotifywait não encontrado.', file=sys.stderr)
        sys.exit(1)

    pending = set()
    last_run = {}
    last_notification = time.time()  # Rastreia última notificação periódica
    lock = threading.Lock()

    def debounce_thread():
        nonlocal last_notification
        while True:
            time.sleep(0.2)
            now = time.time()
            to_run = []
            
            # Verifica se é hora de enviar notificação periódica (12 horas)
            if now - last_notification >= NOTIFICATION_INTERVAL_HOURS * 3600:
                print(f"[Notificação periódica] Verificando commits pendentes...", flush=True)
                check_and_notify_pending_commits(repos)
                last_notification = now
            
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
            print(f"DEBUG: inotify line: {line.strip()}", flush=True)
            parts = line.split(None, 3)
            if parts:
                repo = repo_root(parts[0])
                print(f"DEBUG: repo found: {repo}", flush=True)
                if repo:
                    with lock:
                        pending.add(repo)
                        print(f"DEBUG: added to pending: {repo}", flush=True)
    except KeyboardInterrupt:
        proc.terminate()
        return

def main(argv):
    bases = argv if argv else DEFAULT_BASES
    repos = find_repos(bases)
    if not repos:
        print('Nenhum repositório encontrado.', file=sys.stderr)
        return 1
    print(f'Observando {len(repos)} repositórios. Debounce {DEBOUNCE_MS}ms. Notificação a cada {NOTIFICATION_INTERVAL_HOURS}h.')
    watcher_loop(repos, repos)
    return 0

if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))