#!/usr/bin/env python3
import importlib.util
import json
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

if __name__ == '__main__': unittest.main()
