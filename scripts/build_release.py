#!/usr/bin/env python3
"""Build installable binaries; GitHub supplies source downloads automatically."""
import argparse
import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parent.parent
TARGETS = ('darwin/arm64', 'darwin/amd64', 'linux/arm64', 'linux/amd64')
VERSION = re.compile(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?')


def write_archive(output, files):
    with tarfile.open(output, 'w:gz') as archive:
        for name, source in files.items():
            data = Path(source).read_bytes()
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = 0o755 if name in {"bab", "bridge-ttyd"} else 0o644
            archive.addfile(info, io.BytesIO(data))


def build(version, output, targets):
    if not VERSION.fullmatch(version):
        raise ValueError('version must be a numeric release version, optionally with a prerelease suffix')
    if not targets or len(set(targets)) != len(targets) or any(target not in TARGETS for target in targets):
        raise ValueError('unsupported release target')
    output.mkdir(parents=True, exist_ok=True)
    checksums = []
    goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], text=True).strip())
    with tempfile.TemporaryDirectory(prefix='bab-release-build-') as directory:
        temporary = Path(directory)
        for target in targets:
            system, arch = target.split('/')
            files = {}
            for name in ('bab', 'bridge-ttyd'):
                binary = temporary / name
                subprocess.run(['go', 'build', '-trimpath', '-ldflags',
                                f'-s -w -X main.version={version}', '-o', str(binary),
                                f'./cmd/{name}'], cwd=ROOT,
                               env=dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED='0'), check=True)
                files[name] = binary
            asset = output / f'bab_{version}_{system}_{arch}.tar.gz'
            files.update({"LICENSE": ROOT / "LICENSE", "NOTICE": ROOT / "NOTICE", "GO_LICENSE": goroot / "LICENSE"})
            write_archive(asset, files)
            checksums.append(f'{hashlib.sha256(asset.read_bytes()).hexdigest()}  {asset.name}\n')
    (output / 'SHA256SUMS').write_text(''.join(checksums))
    print(f'Built {len(targets)} binary release assets and SHA256SUMS in {output}')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', default=(ROOT / 'VERSION').read_text().strip())
    parser.add_argument('--output', type=Path, default=ROOT / 'dist')
    parser.add_argument('--target', action='append', choices=TARGETS, help='default: all supported platforms')
    args = parser.parse_args()
    build(args.version, args.output, args.target or TARGETS)


if __name__ == '__main__':
    main()
