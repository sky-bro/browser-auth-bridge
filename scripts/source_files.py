"""Public source selection for auditing; generated runtime configuration is excluded."""
import os
from pathlib import Path


def source_files(root):
    root = Path(root)
    for directory, dirs, files in os.walk(root, followlinks=False):
        parent = Path(directory)
        for name in list(dirs):
            path = parent / name
            if name in {'__pycache__', '.git', '.local', '.secrets', '.private'} or parent == root and name in {'bin', 'dist', 'extension'}:
                dirs.remove(name)
            elif path.is_symlink():
                raise ValueError(f'source symlink is not allowed: {path.relative_to(root)}')
        for name in sorted(files):
            if name == '.git' or name.endswith('.local.json'):
                continue
            path = parent / name
            if path.is_symlink():
                raise ValueError(f'source symlink is not allowed: {path.relative_to(root)}')
            yield path
