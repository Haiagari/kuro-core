#!/usr/bin/env python3
import importlib.util
import json
import io
import runpy
import urllib.request
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('gate', ROOT / 'scripts/ci/pr-gate.py')
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

class GateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / 'repo'; self.repo.mkdir()
        self.git('init', '-q')
        self.git('config', 'user.name', 'Gate Test')
        self.git('config', 'user.email', 'test@example.invalid')
    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], stderr=subprocess.DEVNULL).decode().strip()
    def commit(self, content, path='main.py'):
        dest = self.repo / path; dest.parent.mkdir(parents=True, exist_ok=True); dest.write_text(content)
        self.git('add', '--all'); self.git('commit', '-qm', 'fixture', '--no-verify')
        return self.git('rev-parse', 'HEAD')
    @staticmethod
    def fake_scan(binary, source, policy):
        return ([{'scanner':'semgrep','severity':'CRITICAL','title':'unsafe', 'file':'/src/main.py','line':1}]
                if 'unsafe' in (source / 'main.py').read_text() else [])
    def evaluate(self, base, head):
        with patch.object(gate, 'scan', self.fake_scan):
            return gate.evaluate(self.root/'trusted-kuro', self.repo, base, head, self.root/'trusted-policy')
    def test_clean_change_passes(self):
        base = self.commit('clean'); head = self.commit('still clean')
        self.assertEqual(self.evaluate(base, head)['state'], 'pass')
    def test_skipped_hook_does_not_approve_finding(self):
        base = self.commit('clean'); head = self.commit('unsafe')
        self.assertEqual(self.evaluate(base, head)['state'], 'block')
    def test_existing_finding_requires_identical_file(self):
        base = self.commit('unsafe'); head = self.commit('documentation', 'README.md')
        self.assertEqual(self.evaluate(base, head)['state'], 'pass')
        changed = self.commit('unsafe changed value on same line')
        self.assertEqual(self.evaluate(base, changed)['state'], 'block')
    def test_export_ignore_does_not_hide_file(self):
        base = self.commit('clean'); self.commit('main.py export-ignore\n', '.gitattributes')
        head = self.commit('unsafe')
        self.assertEqual(self.evaluate(base, head)['state'], 'block')
    def test_symlink_rejected(self):
        base = self.commit('clean'); os.symlink('/etc/passwd', self.repo/'link')
        head = self.commit('docs', 'README.md')
        with self.assertRaises(ValueError): self.evaluate(base, head)
    def test_suppression_rejected(self):
        base = self.commit('clean')
        head = self.commit('# checkov' + ':skip=CKV_1\n', 'main.tf')
        with self.assertRaises(ValueError): self.evaluate(base, head)
    def test_failed_or_malformed_reports_never_pass(self):
        binary = self.root/'kuro'; policy = self.root/'policy'
        for body, code in [({'status':'failed','decision':'pass','findings':[]},0),
                           ({'status':'completed','decision':'pass','findings':[]},1),
                           ({'status':'completed','decision':'pass'},0), ('invalid',0)]:
            data = json.dumps(body) if isinstance(body,dict) else body
            binary.write_text('#!/usr/bin/env python3\nimport os\nassert "GH_TOKEN" not in os.environ\nprint('+repr(data)+')\nraise SystemExit('+str(code)+')\n')
            binary.chmod(0o755)
            with patch.dict(os.environ, {'GH_TOKEN':'not-forwarded'}):
                with self.assertRaises(ValueError): gate.scan(binary, self.repo, policy)
    def test_unknown_finding_path_rejected(self):
        with self.assertRaises(ValueError):
            gate.identities([{'file':'/src/missing'}], self.repo, {'main.py':'hash'})

class PublisherTests(unittest.TestCase):
    def publish(self, state, job_status='success'):
        with tempfile.TemporaryDirectory() as temp:
            result = Path(temp)/'result.json'
            result.write_text(json.dumps({'state':state, 'secret':'never-echo-this'}))
            env = {'GITHUB_REPOSITORY':'owner/repo', 'KURO_HEAD_SHA':'a'*40,
                   'GH_TOKEN':'token', 'GITHUB_RUN_ID':'42', 'KURO_CHECK_PHASE':'finish',
                   'KURO_GATE_RESULT':str(result), 'KURO_CHECK_ID':'123', 'KURO_JOB_STATUS':job_status}
            with patch.dict(os.environ, env), patch.object(urllib.request, 'urlopen', return_value=io.BytesIO(b'{"id":123}')) as call:
                runpy.run_path(str(ROOT/'scripts/ci/publish-check.py'))
            request = call.call_args.args[0]
            body = json.loads(request.data)
            self.assertEqual(body['head_sha'], 'a'*40)
            self.assertEqual(request.full_url, 'https://api.github.com/repos/owner/repo/check-runs/123')
            self.assertNotIn('never-echo-this', request.data.decode())
            return body
    def test_success_is_head_bound(self):
        self.assertEqual(self.publish('pass')['conclusion'], 'success')
    def test_block_is_failure(self):
        self.assertEqual(self.publish('block')['conclusion'], 'failure')
    def test_error_is_failure(self):
        self.assertEqual(self.publish('error')['conclusion'], 'failure')
    def test_failed_job_cannot_publish_success(self):
        self.assertEqual(self.publish('pass','failure')['conclusion'], 'failure')

if __name__ == '__main__': unittest.main()
