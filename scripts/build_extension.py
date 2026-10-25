#!/usr/bin/env python3
"""Package one shared extension from trusted, build-time provider configuration."""
import argparse
import ipaddress
import json
from pathlib import Path, PurePosixPath
import re
import shutil
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parent.parent
CORE_ASSETS = (
    'background.js', 'browser-exec.js', 'stream-transport.js', 'pty-transport.js',
    'pty-content.js', 'bab-background.js', 'bab.html', 'bab.js', 'permissions.html', 'permissions.js', 'site-config.js', 'services.js',
)
IDENTIFIER = re.compile(r'[a-z][a-z0-9_-]{0,63}')
HOST = re.compile(r'[a-z0-9_]+(?:\.[a-z0-9_]+)+')


def origin_pattern(origin):
    if not isinstance(origin, str) or any(c.isspace() for c in origin):
        raise ValueError('origin must be an exact HTTP(S) origin')
    url = urlsplit(origin)
    # Force port validation and disallow URL parser normalization ambiguities.
    _ = url.port
    if (not url.hostname or url.username is not None or url.password is not None
            or origin != f'{url.scheme}://{url.netloc}' or '*' in origin
            or '\\' in origin or '%' in origin or url.netloc.endswith(':')):
        raise ValueError('origin must have no credentials, path, query or fragment')
    try:
        loopback = ipaddress.ip_address(url.hostname).is_loopback
    except ValueError:
        loopback = url.hostname == 'localhost'
    if url.scheme != 'https' and not (url.scheme == 'http' and loopback):
        raise ValueError('HTTPS required except for explicit loopback origins')
    # Chrome match patterns grant hosts across ports. Runtime routing and the
    # consumer's native host must still pin the exact origin, including its port.
    hostname = f'[{url.hostname}]' if ':' in url.hostname else url.hostname
    return f'{url.scheme}://{hostname}/*'


def asset_path(value):
    if (not isinstance(value, str) or not re.fullmatch(r'[a-zA-Z0-9_/-]+\.js', value)
            or '..' in value or PurePosixPath(value).is_absolute()
            or str(PurePosixPath(value)) != value):
        raise ValueError('adapter assets must be relative JavaScript paths')
    return value


def validate(config, root=None):
    root = ROOT if root is None else root
    if not isinstance(config, dict) or set(config) != {'providers', 'assets'}:
        raise ValueError('configuration requires only providers and assets')
    assets = config['assets']
    if not isinstance(assets, dict):
        raise ValueError('assets must map extension paths to source paths')
    for dest, source in assets.items():
        if not asset_path(dest).startswith('adapters/'):
            raise ValueError('adapter destinations must be under adapters/')
        path = root / asset_path(source)
        if not path.resolve().is_relative_to(root.resolve()) or not path.is_file():
            raise ValueError('adapter source must be a file inside the source tree')
    providers = config['providers']
    if not isinstance(providers, list) or not providers:
        raise ValueError('at least one provider is required')
    ids, hosts, patterns, referenced = set(), set(), set(), set()
    for provider in providers:
        if not isinstance(provider, dict):
            raise ValueError('provider must be an object')
        required = {'id', 'origin', 'nativeHost'}
        if not required <= provider.keys() or provider.keys() - required - {'ptyAdapter', 'ptyScripts', 'editableOrigin', 'label'}:
            raise ValueError('unknown or missing provider fields')
        if 'label' in provider and not isinstance(provider['label'], str):
            raise ValueError('provider label must be text')
        ident, host = provider['id'], provider['nativeHost']
        if (not isinstance(ident, str) or not IDENTIFIER.fullmatch(ident)
                or not isinstance(host, str) or not HOST.fullmatch(host)
                or ident in ids or host in hosts):
            raise ValueError('provider IDs and native hosts must be valid and unique')
        ids.add(ident)
        hosts.add(host)
        patterns.add(origin_pattern(provider['origin']))
        if not isinstance(provider.get('editableOrigin', False), bool):
            raise ValueError('editableOrigin must be boolean')
        if provider.get('editableOrigin'):
            patterns.update({'https://*/*', 'http://localhost/*', 'http://127.0.0.1/*', 'http://[::1]/*'})
        if 'ptyAdapter' in provider or 'ptyScripts' in provider:
            adapter, scripts = provider.get('ptyAdapter'), provider.get('ptyScripts')
            if (not isinstance(adapter, str) or not IDENTIFIER.fullmatch(adapter)
                    or not isinstance(scripts, list) or not scripts):
                raise ValueError('PTY adapters require an ID and packaged scripts')
            for script in scripts:
                if asset_path(script) not in assets:
                    raise ValueError('PTY scripts must reference packaged assets')
                referenced.add(script)
    if referenced != set(assets):
        raise ValueError('unreferenced adapter assets')
    return sorted(patterns)


def build(config):
    patterns = validate(config)
    output = ROOT / 'bin/extension'
    # Never overwrite source or an arbitrary user-selected directory. Rebuild at
    # the same path so Chrome can reload the existing extension installation.
    if output.parent.is_symlink() or output.is_symlink():
        raise ValueError('build output must not be a symlink')
    marker = output / '.bridge-generated'
    if output.exists():
        if not marker.is_file():
            raise ValueError('refusing to replace an unmarked extension directory')
        shutil.rmtree(output)
    output.mkdir(parents=True)
    marker.write_text('Generated by scripts/build_extension.py\n')
    for name in CORE_ASSETS:
        shutil.copyfile(ROOT / 'internal/extension' / name, output / name)
    for dest, source in config['assets'].items():
        target = output / dest
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / source, target)
    manifest = json.loads((ROOT / 'internal/extension/manifest.json').read_text())
    manifest.update(name='Browser Auth Bridge',
                    description='Browser-authenticated transport for explicitly configured providers.',
                    optional_host_permissions=patterns)
    (output / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (output / 'config.js').write_text(
        '// Generated from trusted installation configuration.\n'
        'globalThis.BridgeProviders = Object.freeze(' + json.dumps(config['providers'], indent=2) + ');\n')
    return output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True, help='trusted provider JSON file')
    args = parser.parse_args()
    try:
        print(build(json.loads(args.config.read_text())))
    except (ValueError, OSError) as err:
        parser.exit(1, f'extension build failed: {err}\n')


if __name__ == '__main__':
    main()
