"""Private state I/O shared by the disposable trial tools."""

import os
import tempfile
from pathlib import Path


def write_private(path: Path, content: str):
    """Replace state atomically; private permissions apply before writing bytes."""
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    descriptor, name = tempfile.mkstemp(prefix="." + path.name + ".", dir=path.parent)
    temporary = Path(name)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
            stream.write(content)
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)
