import hashlib
import io
import os
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile
import unittest

from build_release import write_archive

ROOT = Path(__file__).resolve().parent.parent


class BinaryInstallerTests(unittest.TestCase):
    def fixture(self, directory, symlink=False):
        root = Path(directory)
        assets = root / 'assets'
        assets.mkdir()
        system = {'Darwin': 'darwin', 'Linux': 'linux'}[platform.system()]
        arch = {'arm64': 'arm64', 'aarch64': 'arm64', 'x86_64': 'amd64', 'amd64': 'amd64'}[platform.machine()]
        asset = assets / f'bab_1.2.3_{system}_{arch}.tar.gz'
        binary = root / 'stub'
        binary.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$BAB_TEST_LOG"\n')
        if symlink:
            with tarfile.open(asset, 'w:gz') as archive:
                info = tarfile.TarInfo('bab')
                info.type = tarfile.SYMTYPE
                info.linkname = '/outside'
                archive.addfile(info)
                info = tarfile.TarInfo('bridge-ttyd')
                info.size = 1
                archive.addfile(info, io.BytesIO(b'x'))
        else:
            write_archive(asset, {'bab': binary, 'bridge-ttyd': binary})
        (assets / 'SHA256SUMS').write_text(f'{hashlib.sha256(asset.read_bytes()).hexdigest()}  {asset.name}\n')
        return assets, asset

    def run_installer(self, directory, assets, *options):
        root = Path(directory)
        prefix = root / 'prefix with spaces'
        result = subprocess.run(['sh', str(ROOT / 'install.sh'), '--version', '1.2.3',
                                 '--prefix', str(prefix), '--from-dir', str(assets), *options],
                                env=dict(os.environ, BAB_TEST_LOG=str(root / 'calls.log')),
                                capture_output=True, text=True, timeout=20)
        return result, prefix

    def test_default_and_selected_client_install_without_build_tools(self):
        for selected in (False, True):
            with self.subTest(selected=selected), tempfile.TemporaryDirectory() as directory:
                assets, _ = self.fixture(directory)
                options = ['--with', 'ttyd', '--origin', 'https://terminal.example.test'] if selected else []
                result, prefix = self.run_installer(directory, assets, *options)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue((prefix / 'bin/bab').is_file())
                self.assertEqual((prefix / 'bin/bridge-ttyd').exists(), selected)
                calls = (Path(directory) / 'calls.log').read_text()
                self.assertIn('completion zsh --install --prefix', calls)
                self.assertIn('setup', calls)
                self.assertEqual('plugin install ttyd --origin' in calls, selected)

    def test_bad_checksum_and_nonregular_assets_do_not_replace_existing_program(self):
        for bad_hash in (False, True):
            with self.subTest(bad_hash=bad_hash), tempfile.TemporaryDirectory() as directory:
                assets, asset = self.fixture(directory, symlink=not bad_hash)
                if bad_hash:
                    (assets / 'SHA256SUMS').write_text('0' * 64 + f'  {asset.name}\n')
                prefix = Path(directory) / 'prefix with spaces'
                (prefix / 'bin').mkdir(parents=True)
                (prefix / 'bin/bab').write_text('preserve existing installation')
                result, _ = self.run_installer(directory, assets)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual((prefix / 'bin/bab').read_text(), 'preserve existing installation')
                self.assertFalse((Path(directory) / 'calls.log').exists())

    def test_reinstall_selected_client_updates_existing_plugin(self):
        with tempfile.TemporaryDirectory() as directory:
            assets, asset = self.fixture(directory)
            binary = Path(directory) / 'stub'
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$BAB_TEST_LOG"\n'
                              'if [ "$*" = "plugin list --json" ]; then printf \'[\\n  {\\n    "site": "ttyd"\\n  }\\n]\\n\'; fi\n')
            write_archive(asset, {'bab': binary, 'bridge-ttyd': binary})
            (assets / 'SHA256SUMS').write_text(f'{hashlib.sha256(asset.read_bytes()).hexdigest()}  {asset.name}\n')
            result, _ = self.run_installer(directory, assets, '--with', 'ttyd', '--origin', 'https://terminal.example.test')
            self.assertEqual(result.returncode, 0, result.stderr)
            calls = (Path(directory) / 'calls.log').read_text()
            self.assertIn('plugin update ttyd --origin', calls)
            self.assertNotIn('plugin install ttyd --origin', calls)

    def test_default_version_uses_published_release_and_handles_alpha_only(self):
        for stable in (False, True):
            with self.subTest(stable=stable), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                assets, _ = self.fixture(directory)
                tools = root / 'tools'
                tools.mkdir()
                curl = tools / 'curl'
                curl.write_text('''#!/bin/sh
set -eu
url=; output=
while [ "$#" -gt 0 ]; do
 case "$1" in
  -o) shift; output=$1;;
  https://*) url=$1;;
 esac
 shift
done
printf '%s\\n' "$url" >> "$BAB_TEST_REQUESTS"
case "$url" in
 */releases/latest)
  [ "$BAB_TEST_STABLE" = yes ] || exit 22
  printf '{"tag_name":"v1.2.3"}' > "$output";;
 *'releases?per_page=1') printf '[{"tag_name":"v1.2.3","prerelease":true}]' > "$output";;
 */releases/download/v1.2.3/*) cp "$BAB_TEST_ASSETS/${url##*/}" "$output";;
 *) exit 23;;
esac
''')
                curl.chmod(0o700)
                env = dict(os.environ, PATH=str(tools)+os.pathsep+os.environ['PATH'],
                           BAB_TEST_REQUESTS=str(root / 'requests'), BAB_TEST_ASSETS=str(assets),
                           BAB_TEST_LOG=str(root / 'calls.log'), BAB_TEST_STABLE='yes' if stable else 'no')
                result = subprocess.run(['sh', str(ROOT / 'install.sh'), '--prefix', str(root / 'prefix'), '--no-setup'],
                                        env=env, capture_output=True, text=True, timeout=20)
                self.assertEqual(result.returncode, 0, result.stderr)
                requests = (root / 'requests').read_text()
                self.assertNotIn('/main/VERSION', requests)
                self.assertEqual('releases?per_page=1' in requests, not stable)
                self.assertTrue((root / 'prefix/bin/bab').is_file())
