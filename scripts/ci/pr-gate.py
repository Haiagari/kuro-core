#!/usr/bin/env python3
"""Trusted PR scanner. Execute this file and Kuro from the base, never the PR."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tempfile

SUPPRESS = re.compile(rb'checkov\s*:\s*skip\s*=', re.I)

def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args])

def snapshot(repo, commit, destination):
    if not re.fullmatch(r'[0-9a-f]{40}', commit):
        raise ValueError('Expected a full Git commit SHA')
    files = {}
    for entry in git(repo, 'ls-tree', '-rz', commit).split(b'\0'):
        if not entry:
            continue
        header, name = entry.split(b'\t', 1)
        mode, kind, oid = header.split()
        path = name.decode('utf-8', errors='strict')
        parts = PurePosixPath(path)
        if parts.is_absolute() or '..' in parts.parts or '.git' in parts.parts:
            raise ValueError('Unsupported source path')
        if kind != b'blob' or mode not in (b'100644', b'100755'):
            raise ValueError('Symlinks and submodules require a separate scan contract')
        data = git(repo, 'cat-file', 'blob', oid.decode())
        target = destination / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        files[path] = hashlib.sha256(data).hexdigest()
    return files

def scan(binary, source, policy):
    # Never forward the workflow token or repository-configured environments.
    allowed = ('PATH', 'HOME', 'KURO_TRIVY_CACHE_DIR')
    env = {k: os.environ[k] for k in allowed if k in os.environ}
    env.update(KURO_POLICY_PATH=str(policy), KURO_NO_CACHE='1', KURO_MAX_CONCURRENCY='2')
    proc = subprocess.run([str(binary), 'scan', str(source), '--json', '--no-cache', '--ci'],
                          env=env, capture_output=True, timeout=2100)
    try:
        result = json.loads(proc.stdout)
    except (ValueError, UnicodeError):
        raise ValueError('Scanner output is not valid JSON') from None
    expected = {'pass': 0, 'review': 2, 'block': 1}
    if (not isinstance(result, dict) or result.get('status') != 'completed'
            or result.get('error') or result.get('decision') not in expected
            or proc.returncode != expected[result['decision']]
            or not isinstance(result.get('findings'), list)):
        # Do not echo raw diagnostics: they may contain source secrets.
        raise ValueError('Incomplete or inconsistent scanner execution')
    return result['findings']

def identities(findings, source, files):
    result = set()
    for item in findings:
        if not isinstance(item, dict):
            raise ValueError('Invalid finding')
        path = item.get('file')
        if not isinstance(path, str):
            raise ValueError('Finding has no source path')
        for prefix in (str(source) + '/', '/src/'):
            if path.startswith(prefix):
                path = path[len(prefix):]
                break
        path = path.removeprefix('./')
        # Checkov's native paths are root-relative (/Dockerfile).
        path = path.removeprefix('/')
        if path not in files:
            raise ValueError('Finding cannot be bound to a scanned source file')
        fields = [item.get(k) for k in ('scanner', 'severity', 'title')]
        if not all(isinstance(v, str) and v for v in fields):
            raise ValueError('Incomplete finding identity')
        line = item.get('line')
        if not isinstance(line, int) or isinstance(line, bool) or line < 0:
            raise ValueError('Invalid finding location')
        # Whole-file hash prevents changed secrets on the same line from inheriting approval.
        detail = item.get('detail', '')
        if not isinstance(detail, str) or (fields[0] == 'trivy' and not detail):
            raise ValueError('Dependency finding lacks package/version detail')
        dependency_context = ''
        if fields[0] == 'trivy':
            # Lockfiles can influence a report attributed to an unchanged manifest.
            inputs = {p: digest for p, digest in files.items()
                      if Path(p).name in ('go.mod','go.sum','go.work','go.work.sum','package.json',
                                          'package-lock.json','yarn.lock','pnpm-lock.yaml','Pipfile',
                                          'Pipfile.lock','pyproject.toml','poetry.lock','Cargo.toml',
                                          'Cargo.lock','Gemfile','Gemfile.lock','composer.json',
                                          'composer.lock','pom.xml','build.gradle','build.gradle.kts')
                      or p.endswith(('.lock','.lock.json','.csproj'))
                      or (Path(p).name.startswith('requirements') and p.endswith('.txt'))}
            dependency_context = hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()
        result.add((*fields, path, line, files[path], detail, dependency_context))
    return result

def evaluate(binary, repo, base, candidate, policy):
    with tempfile.TemporaryDirectory(prefix='kuro-pr-gate-') as temp:
        root = Path(temp)
        before, after = root / 'base', root / 'candidate'
        before.mkdir(); after.mkdir()
        base_files = snapshot(repo, base, before)
        head_files = snapshot(repo, candidate, after)
        for path, digest in head_files.items():
            if (base_files.get(path) != digest and (path.endswith('.tf') or 'dockerfile' in Path(path).name.lower())
                    and SUPPRESS.search((after / path).read_bytes())):
                raise ValueError('Changed source contains a Checkov suppression directive')
        old = identities(scan(binary, before, policy), before, base_files)
        new = identities(scan(binary, after, policy), after, head_files)
        introduced = new - old
        return {'state': 'pass' if not introduced else 'block',
                'base_findings': len(old), 'candidate_findings': len(new),
                'introduced_findings': len(introduced),
                'introduced': [{'scanner': f[0], 'severity': f[1], 'rule': f[2], 'file': f[3], 'line': f[4]}
                               for f in sorted(introduced)], 'base_sha': base,
                'candidate_sha': candidate}

def main():
    parser = argparse.ArgumentParser()
    for name in ('binary', 'repo', 'base', 'candidate', 'policy', 'output'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    try:
        result = evaluate(Path(args.binary).resolve(), Path(args.repo), args.base,
                          args.candidate, Path(args.policy).resolve())
    except (ValueError, OSError, subprocess.SubprocessError):
        result = {'state': 'error', 'message': 'Gate failed: inspect trusted runtime/configuration; raw scanner output is not published.'}
    Path(args.output).write_text(json.dumps(result) + '\n')
    print(json.dumps(result))
    return 0 if result['state'] == 'pass' else 1

if __name__ == '__main__':
    raise SystemExit(main())
