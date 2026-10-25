#!/usr/bin/env python3
"""Check fresh CLI installation without opening Chrome or registering a host."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent


def main():
    with tempfile.TemporaryDirectory(prefix='bab-install-smoke-') as directory:
        scratch = Path(directory)
        prefix = scratch / 'prefix'
        registry = scratch / 'registry'
        environment = dict(os.environ, BAB_HOME=str(registry))

        def run(command, expected=0):
            result = subprocess.run(command, cwd=ROOT, env=environment,
                                    capture_output=True, text=True, timeout=180)
            if result.returncode != expected:
                raise RuntimeError(f'{command[0]} exited {result.returncode}:\n'
                                   f'{result.stdout}\n{result.stderr}')
            return result.stdout

        # Explicit prefix: no user's binary directory or shell startup is changed.
        run(['make', 'install-bab', f'PREFIX={prefix}'])
        binary = str(prefix / 'bin/bab')
        if not (prefix / 'share/zsh/site-functions/_bab').is_file():
            raise RuntimeError('completion file was not installed')
        if json.loads(run([binary, 'plugin', 'list', '--json'])) != []:
            raise RuntimeError('fresh registry contains unsolicited plugins')
        catalog = json.loads(run([binary, 'plugin', 'available', '--json']))
        by_name = {entry['name']: entry for entry in catalog}
        if set(by_name) != {'ttyd', 'tjupt'} or not by_name['ttyd']['originRequired'] or by_name['tjupt']['originRequired']:
            raise RuntimeError('unexpected distributed source catalog')
        if any(entry['installed'] for entry in catalog):
            raise RuntimeError('optional example was installed by default')
        run([binary, 'setup'])  # No extension ID: never writes a native manifest.
        extension = registry / 'extension'
        if not (extension / 'manifest.json').is_file():
            raise RuntimeError('setup did not assemble the extension')
        if (extension / 'plugins').exists():
            raise RuntimeError('empty installation contains browser plugin code')
        if (registry / 'setup.json').exists():
            raise RuntimeError('setup registered an extension without an ID')
        result = subprocess.run([binary, 'plugin', 'install', 'ttyd'], cwd=ROOT,
                                env=environment, capture_output=True, text=True, timeout=15)
        if result.returncode == 0 or 'requires --origin' not in result.stderr:
            raise RuntimeError('first installation did not require a chosen origin')
        if json.loads(run([binary, 'plugin', 'list', '--json'])) != []:
            raise RuntimeError('failed installation modified the registry')
        run([binary, 'extension', 'build'])
        run([binary, 'completion', 'zsh'])
        print('Fresh CLI installation passed: isolated prefix and registry, empty '
              'defaults, setup, completion and required-origin validation. '
              'No Chrome, Native Host registration or live accounts were used.')


if __name__ == '__main__':
    main()
