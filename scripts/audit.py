"""Fail closed on private hosts, credential artifacts and unexpected source paths."""
import argparse
from pathlib import Path
import re
import subprocess
from source_files import source_files
root = Path(__file__).resolve().parent.parent
allowed = {'bridge', 'nativehost', 'pool', 'internal', 'testdata', 'cmd', 'scripts', 'examples', '.github', 'commandstream', 'plugins', 'docs'}
files = {'README.md', 'README.zh-CN.md', 'SECURITY.md', 'Makefile', 'go.mod', '.gitignore', 'LICENSE', 'NOTICE', 'PUBLICATION.md', 'VERSION', 'install.sh', '.gitmodules'}
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--deny-patterns', type=Path, help='private operator regex patterns, one per line; never packaged by default')
args = parser.parse_args()
patterns = []
if args.deny_patterns:
    patterns = [re.compile(line) for line in args.deny_patterns.read_text().splitlines()
                if line and not line.startswith('#')]
errors = []
# Ignored runtime directories must never be force-added to public Git history.
if (root / '.git').exists():
    tracked = subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z']).decode().split('\0')
    for name in filter(None, tracked):
        path = Path(name)
        if (path.parts[0] in {'extension', 'bin', 'dist'} or
                any(part in {'.local', '.private', '.secrets', '__pycache__'} for part in path.parts) or
                name.endswith('.local.json')):
            errors.append(f'generated or private path tracked by Git: {name}')
for p in source_files(root):
    relative = p.relative_to(root)
    if relative.parts[0] not in allowed and str(relative) not in files:
        errors.append(f'unexpected file: {relative}')
    if p.suffix in {'.pem', '.key', '.gz', '.zip'} or p.name in {'.env', 'clusters.json'}:
        errors.append(f'forbidden artifact: {relative}')
    try:
        text = p.read_text(encoding='utf-8')
    except UnicodeError:
        errors.append(f'non-text source artifact: {relative}')
        continue
    if any(pattern.search(text) or pattern.search(str(relative)) for pattern in patterns):
        errors.append(f'private identifier: {relative}')
    secrets = r'-----BEGIN (?:RSA |EC |OPENSSH )?' + r'PRIVATE KEY-----|\bgh' + r'[pousr]_[A-Za-z0-9]{30,}|\bAK' + r'IA[A-Z0-9]{16}\b'
    if re.search(secrets, text):
        errors.append(f'possible credential: {relative}')
if errors:
    raise SystemExit('\n'.join(errors))
print('Source allowlist, credential and configured deny-pattern checks passed (not a legal/history review).')
