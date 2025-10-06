#!/usr/bin/env python3
import os
import subprocess
import sys
import shutil
from datetime import datetime
HOOK_VERSION = "0.0.0"

# Caminhos base
base_paths = ['/home/kaiman/Repos/Meus', '/home/kaiman/Repos/Terceiros']

# Ícones (usando ícones padrão do sistema; ajuste se necessário)
icons = {
    'not_init': 'folder-black',
    'clean': 'folder-green',
    'staged': 'folder-yellow',
    'modified': 'folder-orange',
    'untracked': 'folder-red',
    'synced': 'folder-green',
    'pending_sync': 'folder-purple'
}

def repo_synced(path, fetch=False):
    if not os.path.isdir(os.path.join(path, '.git')):
        return False, False
    try:
        if fetch:
            subprocess.run(['git','fetch','--quiet','--prune'], cwd=path, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=60)
        wt = subprocess.run(['git','status','--porcelain'], cwd=path, capture_output=True, text=True, timeout=15)
        dirty = bool(wt.stdout.strip())
        up = subprocess.run(['git','rev-parse','--abbrev-ref','--symbolic-full-name','@{u}'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=5)
        if up.returncode != 0:
            # sem upstream: considerar pendente se dirty ou há commits locais em relação a origin/HEAD tentativo
            base_guess = subprocess.run(['git','remote','show'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=5)
            if base_guess.returncode == 0 and base_guess.stdout.strip():
                remote = base_guess.stdout.strip().splitlines()[0]
                # tenta origin/main, origin/master
                for branch in ('main','master'):
                    probe = subprocess.run(['git','rev-parse',f'{remote}/{branch}'], cwd=path, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                    if probe.returncode == 0:
                        ahead_probe = subprocess.run(['git','rev-list','--left-right','--count',f'HEAD...{remote}/{branch}'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
                        if ahead_probe.returncode == 0:
                            parts = ahead_probe.stdout.strip().split()
                            ahead_cnt = int(parts[0]) if parts else 0
                            behind_cnt = int(parts[1]) if len(parts)>1 else 0
                            if ahead_cnt==0 and behind_cnt==0 and not dirty:
                                return True, True
                            return False, True
            return False, True
        # tem upstream definido
        # commits locais não enviados
        ahead_list = subprocess.run(['git','rev-list','@{u}..HEAD','--count'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=10)
        behind_list = subprocess.run(['git','rev-list','HEAD..@{u}','--count'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=10)
        ahead_cnt = int(ahead_list.stdout.strip() or '0') if ahead_list.returncode==0 else 0
        behind_cnt = int(behind_list.stdout.strip() or '0') if behind_list.returncode==0 else 0
        if ahead_cnt==0 and behind_cnt==0 and not dirty:
            return True, True
        return False, True
    except Exception:
        return False, True

def refresh_dolphin(quiet: bool=True):
    """Tenta forçar o Dolphin a recarregar ícones."""
    if not shutil.which('qdbus') and not shutil.which('qdbus6'):
        return False
    qdbus_bin = shutil.which('qdbus') or shutil.which('qdbus6')
    cmds = [
        [qdbus_bin, 'org.kde.dolphin', '/dolphin/Dolphin_1', 'refresh'],
        [qdbus_bin, 'org.kde.dolphin', '/dolphin/Dolphin_1', 'org.qtproject.Qt.QWidget.update'],
    ]
    for c in cmds:
        try:
            r = subprocess.run(c, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=1)
            if r.returncode == 0:
                return True
        except Exception:
            pass
    return False

def kdir_notify(paths, quiet: bool=True):
    """Emite sinais KDirNotify."""
    if not paths:
        return False
    dbus_send = shutil.which('dbus-send')
    if not dbus_send:
        return False
    success = False
    sent = set()
    for p in paths:
        if p in sent:
            continue
        sent.add(p)
        norm = p.rstrip('/') + '/'
        url = 'file://' + norm
        cmd = [dbus_send, '--session', '--dest=org.kde.KDirNotify', '/KDirNotify', 'org.kde.KDirNotify.DirectoryChanged', f'string:{url}']
        try:
            r = subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=1)
            if r.returncode == 0:
                success = True
        except Exception:
            pass
    return success

def get_git_status(path):
    if not os.path.isdir(os.path.join(path, '.git')):
        return 'not_init'
    try:
        result = subprocess.run(['git', 'status', '--porcelain'], cwd=path, capture_output=True, text=True)
        if result.returncode != 0:
            return 'not_init'  # erro, talvez não repo
        status = result.stdout.strip()
        if not status:
            return 'clean'
        lines = status.split('\n')
        # IGNORE_MARKERS: remover .directory de considerações
        lines = [l for l in lines if l.strip() != '?? .directory']
        has_staged = any(line.startswith(('A ', 'M ', 'D ', 'R ')) for line in lines)
        has_modified = any(line.startswith((' M', 'MM', 'AM', 'RM')) for line in lines)
        has_untracked = any(line.startswith('?? ') for line in lines)
        if has_staged and not has_modified:
            return 'staged'
        elif has_modified:
            return 'modified'
        elif has_untracked:
            return 'untracked'
        else:
            return 'clean'
    except Exception as e:
        return 'not_init'

def update_directory_icon(path, status):
    directory_file = os.path.join(path, '.directory')
    icon = icons.get(status, 'folder')
    content = f"""[Desktop Entry]
Icon={icon}
"""
    with open(directory_file, 'w') as f:
        f.write(content)
    # Toca mtime da pasta e do pai para ajudar o Dolphin a perceber mudança
    try:
        now = None  # usa tempo atual
        os.utime(path, None)
        parent = os.path.dirname(path.rstrip('/'))
        if parent and os.path.isdir(parent):
            os.utime(parent, None)
    except Exception:
        pass

def iter_repos_in(path):
    """Itera repositórios diretos dentro de um caminho base."""
    if not os.path.isdir(path):
        return
    for name in os.listdir(path):
        fp = os.path.join(path, name)
        if os.path.isdir(os.path.join(fp, '.git')):
            yield fp

def collect_targets(targets):
    """Normaliza lista de caminhos passados por linha de comando/hook.
    Se for repo, inclui; se for diretório base, inclui seus filhos repos.
    """
    seen = set()
    for t in targets:
        t = os.path.abspath(t)
        if os.path.isdir(os.path.join(t, '.git')):
            if t not in seen:
                seen.add(t)
                yield t
        else:
            # tratar como base
            for repo in iter_repos_in(t):
                if repo not in seen:
                    seen.add(repo)
                    yield repo

def hook_current_version(repo: str) -> str:
    path = os.path.join(repo, '.git', 'hooks', 'post-commit')
    if not os.path.isfile(path):
        return ""
    try:
        with open(path, 'r') as f:
            for line in f:
                if line.startswith('# hook-version:'):
                    return line.split(':',1)[1].strip()
    except Exception:
        return ""
    return ""

def ensure_hooks(repo: str, force: bool=False, quiet: bool=True):
    cur = hook_current_version(repo)
    if not force and cur == HOOK_VERSION:
        return False
    # Chama install_hooks.py somente para este repo
    installer = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'install_hooks.py')
    if os.path.isfile(installer):
        try:
            subprocess.run([installer, repo], check=False, stdout=subprocess.DEVNULL if quiet else None, stderr=subprocess.DEVNULL if quiet else None)
            return True
        except Exception:
            return False
    return False

def main(targets=None, quiet=False, log=False, ensure=False, force_hooks=False, sync_mode=False, fetch_remotes=False):
    processed = 0
    changed_summary = {k:0 for k in icons.keys()}
    processed_paths = []
    def out(msg):
        if not quiet:
            print(msg)
    if targets:
        repos = list(collect_targets(targets))
    else:
        repos = []
        for base in base_paths:
            repos.extend(list(iter_repos_in(base)))
    for repo in repos:
        if ensure:
            updated = ensure_hooks(repo, force=force_hooks, quiet=quiet)
            if updated and not quiet:
                out(f"[hooks] atualizado em {repo}")
        if sync_mode:
            synced, valid = repo_synced(repo, fetch=fetch_remotes)
            status = 'synced' if synced else 'pending_sync'
        else:
            status = get_git_status(repo)
        update_directory_icon(repo, status)
        changed_summary[status] = changed_summary.get(status,0)+1
        if not quiet:
            out(f"{repo}: {status}")
        processed += 1
        processed_paths.append(repo)
    if log:
        try:
            log_dir = os.path.join(os.path.expanduser('~'), '.cache')
            os.makedirs(log_dir, exist_ok=True)
            with open(os.path.join(log_dir, 'reposync.log'), 'a') as f:
                summary = " ".join(f"{k}={v}" for k,v in changed_summary.items())
                f.write(f"[{datetime.now().isoformat(timespec='seconds')}] processed={processed} {summary}\n")
        except Exception:
            pass
    if not quiet:
        out(f"Total: {processed} repos")
    # Primeiro tenta notificar via KDirNotify
    notified = kdir_notify(processed_paths, quiet=True)
    if not notified:
        # Fallback para método anterior
        refresh_dolphin(quiet=True)

if __name__ == "__main__":
    args = []
    quiet = False
    log = False
    ensure = False
    force_hooks = False
    sync_mode = False
    fetch_remotes = False
    for a in sys.argv[1:]:
        if a in ("-q", "--quiet"):
            quiet = True
        elif a in ("-l", "--log"):
            log = True
        elif a in ("-e", "--ensure-hooks"):
            ensure = True
        elif a in ("-F", "--force-hooks"):
            force_hooks = True
        elif a in ("-s", "--sync-mode"):
            sync_mode = True
        elif a in ("-fR", "--fetch-remotes"):
            fetch_remotes = True
        else:
            args.append(a)
    main(args if args else None, quiet=quiet, log=log, ensure=ensure, force_hooks=force_hooks, sync_mode=sync_mode, fetch_remotes=fetch_remotes)
