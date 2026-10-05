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
manifest = {
    'version': version,
    'published': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'native': {},
    'files': {name: hashlib.sha256(Path('dist', name).read_bytes()).hexdigest() for name in ['install.py', 'update.py']},
    'notes': {
        'ru': 'Профиль пользователя и выдача подписки в админ-боте, расширенные тексты и ссылки подключения. Модальное окно и удаление подписки с сохранением Telegram-профиля. Исправлены счётчики посетителей и зарегистрированных устройств, пояснения адреса и порта подписки.',
        'en': 'Customer profiles and contextual subscription grants in the admin bot, richer messages and visible connection links. Subscription modal and deletion preserving the Telegram profile. Correct visitor and registered-device counts, clarified subscription address and port settings.',
    },
}
for arch in ['x86_64', 'aarch64']:
    archive = Path(f'dist/mirai-linux-{arch}.tar.gz')
    manifest['native'][arch] = {
        'url': f'https://github.com/EnsixD/mirai/releases/download/{tag}/{archive.name}',
        'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
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
