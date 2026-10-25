from pathlib import Path
import tempfile
import unittest

from source_files import source_files


class SourceBoundaryTests(unittest.TestCase):
    def test_export_excludes_history_builds_and_caches(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ['README.md', '.git/config', 'bin/extension/config.js',
                         'dist/archive.gz', 'extension/config.js', 'scripts/__pycache__/cache.pyc', 'bridge/client.go',
                         'plugins/external/example/.git', 'plugins/.private/manifest.json',
                         '.local/catalog.json', 'examples/site.local.json']:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('fixture')
            self.assertEqual({str(p.relative_to(root)) for p in source_files(root)},
                             {'README.md', 'bridge/client.go'})

    def test_symlink_file_or_directory_cannot_escape_publication_boundary(self):
        for directory_link in [False, True]:
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / 'public'
                root.mkdir()
                outside = Path(directory) / 'private'
                if directory_link:
                    outside.mkdir()
                else:
                    outside.write_text('private fixture')
                (root / 'link').symlink_to(outside, target_is_directory=directory_link)
                with self.assertRaises(ValueError):
                    list(source_files(root))
