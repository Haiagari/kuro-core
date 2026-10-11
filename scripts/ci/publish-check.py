#!/usr/bin/env python3
"""Publish only a fixed check summary, bound to the PR's exact head SHA."""
import json
import os
from pathlib import Path
import re
import urllib.request

repo, sha = os.environ['GITHUB_REPOSITORY'], os.environ['KURO_HEAD_SHA']
if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo) or not re.fullmatch(r'[0-9a-f]{40}', sha):
    raise SystemExit('Invalid repository or head SHA')
headers = {'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
           'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28'}
phase = os.environ['KURO_CHECK_PHASE']
body = {'name': 'Kuro PR Gate', 'head_sha': sha,
        'details_url': f'https://github.com/{repo}/actions/runs/{os.environ["GITHUB_RUN_ID"]}'}
if phase == 'start':
    body['status'] = 'in_progress'
    url = f'https://api.github.com/repos/{repo}/check-runs'
    method = 'POST'
else:
    try:
        result = json.loads(Path(os.environ['KURO_GATE_RESULT']).read_text())
    except (OSError, ValueError):
        result = {'state': 'error'}
    passed = result.get('state') == 'pass' and os.environ.get('KURO_JOB_STATUS') == 'success'
    body.update(status='completed', conclusion='success' if passed else 'failure',
                output={'title': 'Kuro gate passed' if passed else 'Kuro gate blocked',
                        'summary': 'Trusted base tools scanned the proposed merge snapshot. '
                                   'Existing findings are tolerated only in byte-identical files at the same path. '
                                   'New findings, changed affected files, unsupported snapshots and scanner errors block.'})
    check_id = os.environ['KURO_CHECK_ID']
    if not check_id.isdigit():
        raise SystemExit('Missing check id')
    url = f'https://api.github.com/repos/{repo}/check-runs/{check_id}'
    method = 'PATCH'
request = urllib.request.Request(url, data=json.dumps(body).encode(), headers=headers, method=method)
with urllib.request.urlopen(request, timeout=30) as response:
    check = json.load(response)
if phase == 'start':
    with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
        output.write(f'check_id={check["id"]}\n')
