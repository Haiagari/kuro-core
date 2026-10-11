"""Exercise the real installer against local release fixtures, without network."""
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class InstallerTests(unittest.TestCase):
    def run_installer(self, bad_checksum=False, latest_missing=False, os_name='Linux', arch='x86_64'):
        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp)
            platform = {'Linux': 'linux', 'Darwin': 'darwin'}[os_name]
            architecture = {'x86_64': 'amd64', 'aarch64': 'arm64'}[arch]
            name = f'kuro_0.1.1_{platform}_{architecture}.tar.gz'
            with tarfile.open(d / name, 'w:gz') as tar:
                content = b'#!/bin/sh\necho fixture\n'
                member = tarfile.TarInfo('kuro')
                member.size = len(content)
                member.mode = 0o755
                tar.addfile(member, io.BytesIO(content))
            digest = hashlib.sha256((d / name).read_bytes()).hexdigest()
            if bad_checksum:
                digest = '0' * 64
            (d / 'kuro_0.1.1_checksums.txt').write_text(f'{digest}  {name}\n')
            mocks = d / 'mocks'
            mocks.mkdir()
            (mocks / 'curl').write_text('''#!/usr/bin/env python3
import os, pathlib, sys
args = sys.argv[1:]
url = args[-1]
root = pathlib.Path(os.environ['FIXTURE_DIR'])
if url.endswith('/releases/latest'):
    if os.environ.get('MISSING_RELEASE') == '1': sys.exit(22)
    print('{"tag_name": "v0.1.1"}')
else:
    path = root / url.rsplit('/', 1)[-1]
    if not path.exists(): sys.exit(22)
    pathlib.Path(args[args.index('-sLfo') + 1]).write_bytes(path.read_bytes())
''')
            (mocks / 'uname').write_text(f'#!/bin/sh\ncase "$1" in -s) echo {os_name};; -m) echo {arch};; esac\n')
            for p in mocks.iterdir():
                p.chmod(0o755)
            env = dict(os.environ, PATH=f'{mocks}:{os.environ["PATH"]}', FIXTURE_DIR=tmp,
                       KURO_INSTALL_DIR=str(d / 'installed'), MISSING_RELEASE=str(int(latest_missing)))
            env.pop('KURO_VERSION', None)
            args = [] if latest_missing else ['v0.1.1']
            result = subprocess.run(['sh', str(ROOT / 'scripts/install.sh'), *args], env=env, capture_output=True, text=True)
            return result, (d / 'installed/kuro').exists()

    def test_pinned_archive_matches_goreleaser_naming(self):
        for system, arch in [('Linux', 'x86_64'), ('Linux', 'aarch64'), ('Darwin', 'x86_64'), ('Darwin', 'aarch64')]:
            with self.subTest(system=system, arch=arch):
                result, installed = self.run_installer(os_name=system, arch=arch)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertTrue(installed)

    def test_checksum_mismatch_never_installs(self):
        result, installed = self.run_installer(bad_checksum=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(installed)

    def test_missing_release_reports_source_build_path(self):
        result, installed = self.run_installer(latest_missing=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(installed)
        self.assertIn('make build', result.stdout + result.stderr)

if __name__ == '__main__':
    unittest.main()
