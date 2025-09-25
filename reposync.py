#!/usr/bin/env python3
import os
import subprocess
import glob

# Caminhos base
base_paths = ['/home/kaiman/Repos/Meus', '/home/kaiman/Repos/Terceiros']

# Ícones (usando ícones padrão do sistema; ajuste se necessário)
icons = {
    'not_init': 'folder-red',
    'clean': 'folder-green',
    'staged': 'folder-blue',
    'modified': 'folder-yellow',
    'untracked': 'folder-purple'
}

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

def main(targets=None):
    if targets:
        dirs = []
        for t in targets:
            if os.path.isdir(t):
                if os.path.isdir(os.path.join(t, ".git")):
                    dirs.append(t)
                else:
                    # se for uma pasta base, varrer filhos
                    for item in os.listdir(t):
                        fp=os.path.join(t,item)
                        if os.path.isdir(os.path.join(fp, ".git")):
                            dirs.append(fp)
        for full_path in dirs:
            status = get_git_status(full_path)
            update_directory_icon(full_path, status)
            print(f"{full_path}: {status}")
    else:
        for base in base_paths:
            if os.path.exists(base):
                for item in os.listdir(base):
                    full_path = os.path.join(base, item)
                    if os.path.isdir(full_path):
                        status = get_git_status(full_path)
                        update_directory_icon(full_path, status)
                        print(f"{full_path}: {status}")
if __name__ == "__main__":
    import sys
    args=[a for a in sys.argv[1:] if not a.startswith("-")]
    main(args if args else None)
