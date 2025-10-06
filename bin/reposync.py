#!/usr/bin/env python3
import os
import subprocess
import sys
import shutil
from datetime import datetime

HOOK_VERSION = "0.0.0"
BASE_PATHS = ['/home/kaiman/Repos/Meus', '/home/kaiman/Repos/Terceiros']
DEFAULT_TIMEOUT = 15
FETCH_TIMEOUT = 60

ICONS = {
    'not_init': 'folder-black',
    'commit': 'folder-yellow',
    'untracked': 'folder-red',
    'no_remote': 'folder-orange',
    'synced': 'folder-green',
    'pending_sync': 'folder-violet'
}

def repo_synced(path, fetch=False):
    if not os.path.isdir(os.path.join(path, '.git')):
        return False, False
    try:
        if fetch:
            subprocess.run(['git','fetch','--quiet','--prune'], cwd=path, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=FETCH_TIMEOUT)
        
        wt = subprocess.run(['git','status','--porcelain'], cwd=path, capture_output=True, text=True, timeout=DEFAULT_TIMEOUT)
        dirty = bool(wt.stdout.strip())
        
        up = subprocess.run(['git','rev-parse','--abbrev-ref','--symbolic-full-name','@{u}'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=5)
        
        if up.returncode != 0:
            base_guess = subprocess.run(['git','remote','show'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=5)
            if base_guess.returncode == 0 and base_guess.stdout.strip():
                remote = base_guess.stdout.strip().splitlines()[0]
                for branch in ('main','master'):
                    if _check_branch_sync(path, remote, branch, dirty):
                        return True, True
            else:
                # Sem remote: considerar como no_remote (pendência)
                return False, False  # Retorna False, False para indicar no_remote
            return False, True
        
        ahead_list = subprocess.run(['git','rev-list','@{u}..HEAD','--count'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=10)
        behind_list = subprocess.run(['git','rev-list','HEAD..@{u}','--count'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=10)
        ahead_cnt = int(ahead_list.stdout.strip() or '0') if ahead_list.returncode==0 else 0
        behind_cnt = int(behind_list.stdout.strip() or '0') if behind_list.returncode==0 else 0
        
        return (ahead_cnt==0 and behind_cnt==0 and not dirty), True
    except Exception:
        return False, True

def _check_branch_sync(path, remote, branch, dirty):
    probe = subprocess.run(['git','rev-parse',f'{remote}/{branch}'], cwd=path, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    if probe.returncode == 0:
        ahead_probe = subprocess.run(['git','rev-list','--left-right','--count',f'HEAD...{remote}/{branch}'], cwd=path, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
        if ahead_probe.returncode == 0:
            parts = ahead_probe.stdout.strip().split()
            ahead_cnt = int(parts[0]) if parts else 0
            behind_cnt = int(parts[1]) if len(parts)>1 else 0
            return ahead_cnt==0 and behind_cnt==0 and not dirty
    return False

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
        result = subprocess.run(['git', 'status', '--porcelain'], cwd=path, capture_output=True, text=True, timeout=DEFAULT_TIMEOUT)
        if result.returncode != 0:
            return 'not_init'
        
        lines = [l for l in result.stdout.strip().split('\n') if l.strip() and l.strip() != '?? .directory']
        if not lines:
            return None
        
        has_commit = any(line.startswith(('A ', 'M ', 'D ', 'R ', ' M', 'MM', 'AM', 'RM')) for line in lines)
        has_untracked = any(line.startswith('?? ') for line in lines)
        
        return 'commit' if has_commit else ('untracked' if has_untracked else None)
    except Exception:
        return 'not_init'

def update_directory_icon(path, status):
    directory_file = os.path.join(path, '.directory')
    icon = ICONS.get(status, 'folder')
    content = f"[Desktop Entry]\nIcon={icon}\n"
    
    with open(directory_file, 'w') as f:
        f.write(content)
    
    try:
        os.utime(path, None)
        parent = os.path.dirname(path.rstrip('/'))
        if parent and os.path.isdir(parent):
            os.utime(parent, None)
    except Exception:
        pass

def iter_repos_in(path):
    if not os.path.isdir(path):
        return
    for name in os.listdir(path):
        fp = os.path.join(path, name)
        if os.path.isdir(os.path.join(fp, '.git')):
            yield fp

def collect_targets(targets):
    seen = set()
    for t in targets:
        t = os.path.abspath(t)
        if os.path.isdir(os.path.join(t, '.git')):
            if t not in seen:
                seen.add(t)
                yield t
        else:
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

def main(targets=None, quiet=False, log=False, ensure=False, force_hooks=False, fetch_remotes=False):
    processed = 0
    changed_summary = {k:0 for k in ICONS.keys()}
    processed_paths = []
    
    repos = list(collect_targets(targets)) if targets else [repo for base in BASE_PATHS for repo in iter_repos_in(base)]
    for repo in repos:
        if ensure and ensure_hooks(repo, force=force_hooks, quiet=quiet) and not quiet:
            print(f"[hooks] atualizado em {repo}")
        
        local_status = get_git_status(repo)
        if local_status in ('commit', 'untracked'):
            status = local_status
        else:
            synced, valid = repo_synced(repo, fetch=fetch_remotes)
            if not valid:
                status = 'no_remote'
            else:
                status = 'synced' if synced else 'pending_sync'
        
        update_directory_icon(repo, status)
        changed_summary[status] = changed_summary.get(status,0)+1
        if not quiet:
            print(f"{repo}: {status}")
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
        print(f"Total: {processed} repos")
    
    if not kdir_notify(processed_paths, quiet=True):
        refresh_dolphin(quiet=True)

if __name__ == "__main__":
    args = []
    quiet = False
    log = False
    ensure = False
    force_hooks = False
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
        elif a in ("-fR", "--fetch-remotes"):
            fetch_remotes = True
        else:
            args.append(a)
    main(args if args else None, quiet=quiet, log=log, ensure=ensure, force_hooks=force_hooks, fetch_remotes=fetch_remotes)
