#!/usr/bin/env python3
import os
import sys
import time
import subprocess
import threading
import shutil
from datetime import datetime

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
# Config file at project root (parent of bin)
CONFIG_FILE = os.path.join(os.path.dirname(SCRIPT_DIR), 'watched_paths.conf')
REPOSYNC = os.path.join(SCRIPT_DIR, 'reposync.py')
DEFAULT_BASES = ['/home/kaiman/Repos/Github/Meus', '/home/kaiman/Repos/Github/Terceiros']
DEBOUNCE_MS = int(os.environ.get('DEBOUNCE_MS', '2000'))

def is_git_repo(path):
    # Standard repo (has .git dir)
    if os.path.isdir(os.path.join(path, '.git')):
        return True
    # Bare repo (has HEAD, config, refs files/dirs inline)
    if os.path.isfile(os.path.join(path, 'HEAD')) and \
       os.path.isfile(os.path.join(path, 'config')) and \
       os.path.isdir(os.path.join(path, 'refs')):
        return True
    return False

def find_repos(bases):
    repos = []
    seen = set()
    for base in bases:
        base = base.strip()
        if not base or not os.path.isdir(base):
            continue
        
        # Check if base itself is a repo
        if is_git_repo(base):
            if base not in seen:
                repos.append(base)
                seen.add(base)
            continue

        # Check children if base is not a repo itself
        try:
            for name in os.listdir(base):
                p = os.path.join(base, name)
                if os.path.isdir(p) and is_git_repo(p) and p not in seen:
                    repos.append(p)
                    seen.add(p)
        except OSError:
            pass
    return repos

def repo_root(path):
    path = os.path.abspath(path)
    # Check if path itself is root of bare or standard
    if is_git_repo(path):
        return path
        
    while True:
        if os.path.isdir(os.path.join(path, '.git')):
            return path
        # Note: Walking up for bare repos is harder as files look normal. 
        # Focusing on standard detection for parent walk for now.
        
        parent = os.path.dirname(path)
        if parent == path:
            return None
        path = parent

def get_detailed_status(repo):
    """
    Retorna o status detalhado do repositório:
    - 'clean': tudo sincronizado
    - 'commit': possui alterações não commitadas
    - 'sync': possui commits locais não enviados (ahead)
    - 'both': possui alterações não commitadas E commits não enviados
    """
    dirty = False
    is_bare = False
    
    # Check if bare
    if os.path.isfile(os.path.join(repo, 'HEAD')) and not os.path.isdir(os.path.join(repo, '.git')):
         is_bare = True

    try:
        if not is_bare:
            # Verifica se há alterações (porcelain retorna output se houver)
            res = subprocess.run(['git', 'status', '--porcelain'], cwd=repo, capture_output=True, text=True, timeout=5)
            if res.returncode == 0 and res.stdout.strip():
                dirty = True
    except Exception:
        pass

    sync_needed = False
    try:
        # Verifica se há commits ahead ou behind do upstream
        # Primeiro verifica se tem upstream configurado
        subprocess.run(['git', 'rev-parse', '@{u}'], cwd=repo, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=2)
        
        # Check Ahead
        res_ahead = subprocess.run(['git', 'rev-list', '@{u}..HEAD', '--count'], cwd=repo, capture_output=True, text=True, timeout=5)
        if res_ahead.returncode == 0 and int(res_ahead.stdout.strip() or '0') > 0:
            sync_needed = True
        
        # Check Behind (se ainda não marcou como sync_needed)
        if not sync_needed:
            res_behind = subprocess.run(['git', 'rev-list', 'HEAD..@{u}', '--count'], cwd=repo, capture_output=True, text=True, timeout=5)
            if res_behind.returncode == 0 and int(res_behind.stdout.strip() or '0') > 0:
                sync_needed = True

    except Exception:
        pass

    if dirty and sync_needed:
        return 'both'
    elif dirty:
        return 'commit'
    elif sync_needed:
        return 'sync'
    return 'clean'

def send_notification(title, message, timeout=5000):
    """Envia notificação desktop via notify-send."""
    notify_send = shutil.which('notify-send')
    if not notify_send:
        return False
    try:
        # Configura a variável DISPLAY e DBUS_SESSION_BUS_ADDRESS se necessário (para cron/background)
        env = os.environ.copy()
        subprocess.run(
            [notify_send, '-t', str(timeout), title, message],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            timeout=5,
            env=env
        )
        return True
    except Exception:
        return False

def check_and_notify_daily(repos):
    """Verifica todos os repositórios e envia relatório diário consolidado."""
    stats = {'commit': [], 'sync': [], 'both': []}
    
    print("[Daily Check] Iniciando verificação diária...", flush=True)
    for repo in repos:
        status = get_detailed_status(repo)
        repo_name = os.path.basename(repo)
        if status != 'clean':
            stats[status].append(repo_name)
    
    total_issues = len(stats['commit']) + len(stats['sync']) + len(stats['both'])
    
    if total_issues > 0:
        lines = []
        if stats['both']:
            lines.append(f"• {len(stats['both'])} requerem Commit & Sync")
        if stats['commit']:
            lines.append(f"• {len(stats['commit'])} requerem Commit")
        if stats['sync']:
            lines.append(f"• {len(stats['sync'])} requerem Sync")
            
        summary_text = "\n".join(lines)
        
        # Detalhes (limitar a alguns se houver muitos, ou mostrar todos)
        details = []
        if stats['both']:
            details.append("Commit & Sync: " + ", ".join(stats['both'][:5]) + ("..." if len(stats['both']) > 5 else ""))
        if stats['commit']:
            details.append("Commit: " + ", ".join(stats['commit'][:5]) + ("..." if len(stats['commit']) > 5 else ""))
        if stats['sync']:
            details.append("Sync: " + ", ".join(stats['sync'][:5]) + ("..." if len(stats['sync']) > 5 else ""))
            
        body = f"{summary_text}\n\n{chr(10).join(details)}"
        
        # 600000ms = 10 minutos
        send_notification("RepoSync: Relatório Diário", body, timeout=600000)
        print(f"[Daily Check] Notificação enviada. {total_issues} repositórios pendentes.", flush=True)
    else:
        print("[Daily Check] Tudo limpo. Nenhuma notificação necessária.", flush=True)

def run_reposync(repo):
    # Roda o reposync para atualizar ícones/hooks, mas SEM notificação
    subprocess.run([REPOSYNC, repo, '--ensure-hooks', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    # Notificação removida conforme solicitado
    # O feedback visual ficará por conta dos ícones e da notificação diária

def watcher_loop(paths, repos):
    cmd = ['inotifywait', '-m', '-r', '-e', 'modify,attrib,close_write,create,delete,move'] + paths
    try:
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    except FileNotFoundError:
        print('Erro: inotifywait não encontrado.', file=sys.stderr)
        sys.exit(1)

    pending = set()
    last_run = {}
    lock = threading.Lock()
    
    # Rastreamento para notificação diária
    # Se o script iniciar após as 22h, ele notificará no dia seguinte, a menos que forcemos check na startup?
    # Melhor seguir estritamente o horário.
    last_daily_notification_date = None

    def debounce_thread():
        nonlocal last_daily_notification_date
        while True:
            time.sleep(0.5)
            now = datetime.now()
            
            # Lógica de notificação diária: Executar uma vez por dia por volta das 22h
            if now.hour == 22:
                today = now.date()
                if last_daily_notification_date != today:
                    check_and_notify_daily(repos)
                    last_daily_notification_date = today
            
            # Lógica de debounce para reposync (atualização de ícones)
            to_run = []
            now_ts = time.time()
            with lock:
                for repo in list(pending):
                    if repo not in last_run or now_ts - last_run.get(repo, 0) >= DEBOUNCE_MS / 1000.0:
                        to_run.append(repo)
                        last_run[repo] = now_ts
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
            # print(f"DEBUG: inotify line: {line.strip()}", flush=True)
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
    bases = []
    
    if argv:
        bases = argv
    elif os.path.isfile(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, 'r') as f:
                bases = [line.strip() for line in f if line.strip() and not line.startswith('#')]
        except Exception as e:
            print(f"Erro ao ler config: {e}", file=sys.stderr)
    
    if not bases:
        bases = DEFAULT_BASES

    repos = find_repos(bases)
    if not repos:
        print('Nenhum repositório encontrado.', file=sys.stderr)
        return 1
    print(f'Observando {len(repos)} repositórios. Debounce {DEBOUNCE_MS}ms. Notificação diária às 22h.')
    for r in repos:
        print(f" - {r}", flush=True)

    # Initial sync
    print("Realizando sincronização inicial...", flush=True)
    for r in repos:
        run_reposync(r)
    
    watcher_loop(repos, repos)
    return 0

if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))