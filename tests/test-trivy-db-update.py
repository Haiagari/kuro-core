"""Exercise DB preparation staging, source isolation and failure preservation."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class DatabasePreparationTests(unittest.TestCase):
    def exercise(self, fail=False):
        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp)
            cache = d / 'cache'
            (cache / 'db').mkdir(parents=True)
            (cache / 'db/trivy.db').write_bytes(b'previous')
            (cache / 'db/metadata.json').write_text('{}')
            mocks = d / 'mocks'
            mocks.mkdir()
            executable = mocks / 'docker'
            executable.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
if args[0] == 'run':
    pathlib.Path(os.environ['DB_TEST_LOG']).write_text(json.dumps(args))
    if os.environ.get('DB_TEST_FAIL') == '1': sys.exit(1)
    volume = args[args.index('-v') + 1]
    target = pathlib.Path(volume.split(':', 1)[0]) / 'db'
    target.mkdir()
    (target / 'trivy.db').write_bytes(b'new')
    (target / 'metadata.json').write_text('{}')
''')
            executable.chmod(0o755)
            log = d / 'args.json'
            env = dict(os.environ, PATH=f'{mocks}:{os.environ["PATH"]}',
                       KURO_TRIVY_CACHE_DIR=str(cache), DB_TEST_LOG=str(log), DB_TEST_FAIL=str(int(fail)))
            result = subprocess.run(['sh', str(ROOT / 'scripts/update-trivy-db.sh')], env=env, capture_output=True, text=True)
            return result, (cache / 'db/trivy.db').read_bytes(), json.loads(log.read_text()), (cache / '.update-lock').exists()

    def test_success_replaces_database_without_source_mount(self):
        result, data, args, lock = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(data, b'new')
        self.assertEqual(args.count('-v'), 1)
        self.assertIn(':/trivy-cache:Z', args[args.index('-v') + 1])
        self.assertIn('--download-db-only', args)
        self.assertNotIn('/src', args)
        self.assertFalse(lock)

    def test_failed_download_preserves_previous_database(self):
        result, data, _, lock = self.exercise(fail=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(data, b'previous')
        self.assertFalse(lock)

if __name__ == '__main__':
    unittest.main()
