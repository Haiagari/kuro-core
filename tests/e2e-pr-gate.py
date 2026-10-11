#!/usr/bin/env python3
"""Real-image acceptance: hooks/PR ignore settings cannot approve a new finding."""
import importlib.util
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('gate', ROOT/'scripts/ci/pr-gate.py')
gate = importlib.util.module_from_spec(spec); spec.loader.exec_module(gate)
with tempfile.TemporaryDirectory(prefix='kuro-gate-acceptance-') as temp:
    repo = Path(temp)/'source'; repo.mkdir()
    def git(*args):
        return subprocess.check_output(['git','-C',str(repo),*args], stderr=subprocess.DEVNULL).decode().strip()
    git('init','-q'); git('config','user.name','Kuro Gate Test'); git('config','user.email','test@example.invalid')
    def commit(contents):
        for name, text in contents.items(): (repo/name).write_text(text)
        git('add','--all'); git('commit','-qm','synthetic acceptance fixture','--no-verify')
        return git('rev-parse','HEAD')
    binary, policy = ROOT/'bin/kuro', ROOT/'deploy/policies/default-policy.json'
    def evaluate(base, head): return gate.evaluate(binary, repo, base, head, policy)
    base = commit({'main.py':'value = 1\n'})
    clean = commit({'main.py':'value = 2\n'})
    assert evaluate(base, clean)['state'] == 'pass'
    # Only synthetic code: no real credentials. Candidate ignores and binaries are data.
    unsafe = commit({'main.py':'value = ev'+'al(input())\n', '.semgrepignore':'main.py\n',
                     '.gitignore':'main.py\n', '.gitleaks.toml':'[allowlist]\npaths = [".*"]\n',
                     'kuro':'#!/bin/sh\nexit 0\n'})
    remote = Path(temp)/'remote.git'
    subprocess.run(['git','init','--bare','-q',str(remote)],check=True)
    hook = repo/'.git/hooks/pre-push'; hook.write_text('#!/bin/sh\nexit 1\n'); hook.chmod(0o755)
    git('push','--no-verify',str(remote),'HEAD:refs/heads/proposed')
    assert evaluate(base, unsafe)['state'] == 'block', 'Bypassed hook or ignore file approved unsafe source'
    unchanged = commit({'README.md':'Documentation-only update\n'})
    assert evaluate(unsafe, unchanged)['state'] == 'pass'
    changed = commit({'main.py':'value = ev'+'al(input()) # changed\n'})
    assert evaluate(unsafe, changed)['state'] == 'block', 'Same-line finding inherited baseline approval'
print('Native PR gate acceptance passed: clean, bypassed hook, ignores, unchanged debt, changed affected file.')
