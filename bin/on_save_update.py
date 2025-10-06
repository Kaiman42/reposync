#!/usr/bin/env python3
import os
import sys
import subprocess

REPOSYNC = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'reposync.py')

def find_git_root(path):
    path = os.path.abspath(os.path.dirname(path) if os.path.isfile(path) else path)
    while True:
        if os.path.isdir(os.path.join(path, '.git')):
            return path
        parent = os.path.dirname(path)
        if parent == path:
            return None
        path = parent

def main(argv):
    repos = {find_git_root(f) for f in argv if not f.startswith('-') and f.strip()}
    repos.discard(None)
    for repo in repos:
        subprocess.run([REPOSYNC, repo, '--ensure-hooks', '-q'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

if __name__ == '__main__':
    main(sys.argv[1:])
