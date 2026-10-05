#!/usr/bin/env python3
"""Create and sign the native release manifest from CI build outputs."""
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

version = os.environ['RELEASE_VERSION']
tag = os.environ['GITHUB_REF_NAME']
archive = Path('dist/mirai-linux-x86_64.tar.gz')
manifest = {
    'version': version,
    'published': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'native': {'x86_64': {
        'url': f'https://github.com/EnsixD/mirai/releases/download/{tag}/{archive.name}',
        'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
    }},
    'notes': {
        'ru': 'Нативная Mirai: панель и нода без Docker. Подписанные обновления с резервной копией и откатом.',
        'en': 'Native Mirai panel and node without Docker. Signed updates with backup and rollback.',
    },
}
target = Path('dist/manifest.json')
target.write_text(json.dumps(manifest, ensure_ascii=False), encoding='utf-8')
with tempfile.TemporaryDirectory() as temporary:
    key = Path(temporary) / 'key.pem'
    key.write_text(os.environ['RELEASE_SIGNING_KEY'])
    key.chmod(0o600)
    signature = Path(temporary) / 'signature'
    subprocess.run(['openssl', 'pkeyutl', '-sign', '-inkey', str(key), '-rawin',
                    '-in', str(target), '-out', str(signature)], check=True)
    Path('dist/manifest.json.sig').write_text(base64.b64encode(signature.read_bytes()).decode())
