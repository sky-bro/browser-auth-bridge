import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import build_extension as builder


class ExtensionBuildTests(unittest.TestCase):
    def setUp(self):
        self.config = json.loads((builder.ROOT / 'examples/ttyd/providers.example.json').read_text())

    def test_denies_unscoped_origins_and_untrusted_scripts(self):
        for origin in ['http://terminal.example.test', 'https://*.example.test',
                       'https://user@example.test', 'https://example.test/path',
                       'https://example.test#fragment', 'https://example.test:invalid']:
            config = copy.deepcopy(self.config)
            config['providers'][0]['origin'] = origin
            with self.subTest(origin=origin), self.assertRaises(ValueError):
                builder.validate(config)
        for dest, source in [('adapters/a.js', '../outside.js'),
                             ('background.js', 'examples/plugins/ttyd/browser/ttyd.js'),
                             ('adapters/../background.js', 'examples/plugins/ttyd/browser/ttyd.js')]:
            config = copy.deepcopy(self.config)
            config['assets'] = {dest: source}
            with self.subTest(dest=dest, source=source), self.assertRaises(ValueError):
                builder.validate(config)
        self.assertEqual(builder.origin_pattern('http://127.0.0.1:8765'), 'http://127.0.0.1/*')

    def test_builds_multiple_providers_in_same_extension_and_reloads_same_path(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'internal/extension').mkdir(parents=True)
            for name in (*builder.CORE_ASSETS, 'manifest.json'):
                (root / 'internal/extension' / name).write_bytes((builder.ROOT / 'internal/extension' / name).read_bytes())
            (root / 'examples/plugins/ttyd/browser').mkdir(parents=True)
            (root / 'examples/plugins/ttyd/browser/ttyd.js').write_text('// adapter\n')
            self.config['providers'].append({
                'id': 'demo', 'origin': 'http://127.0.0.1:8765',
                'nativeHost': 'org.browserauthbridge.demo',
            })
            with patch.object(builder, 'ROOT', root):
                output = builder.build(self.config)
                manifest = json.loads((output / 'manifest.json').read_text())
                self.assertIn('https://*/*', manifest['optional_host_permissions'])
                self.assertIn('http://127.0.0.1/*', manifest['optional_host_permissions'])
                self.assertNotIn('host_permissions', manifest)
                self.assertIn('org.browserauthbridge.demo', (output / 'config.js').read_text())
                self.assertTrue((output / 'adapters/ttyd.js').is_file())
                (output / 'stale.js').write_text('// removed provider')
                self.assertEqual(output, builder.build(self.config))
                self.assertFalse((output / 'stale.js').exists())
                (output / '.bridge-generated').unlink()
                with self.assertRaises(ValueError):
                    builder.build(self.config)

    def test_duplicate_provider_and_missing_asset_are_denied(self):
        self.config['providers'].append(copy.deepcopy(self.config['providers'][0]))
        with self.assertRaises(ValueError):
            builder.validate(self.config)
        self.config['providers'].pop()
        self.config['assets'] = {}
        with self.assertRaises(ValueError):
            builder.validate(self.config)


if __name__ == '__main__':
    unittest.main()
