#!/usr/bin/env python3
"""Signed native Mirai updates, with binary and PostgreSQL rollback."""
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request
import fcntl

ROOT = Path('/opt/mirai')
DATA = Path('/var/lib/mirai')
STATE = DATA / 'update'
STATE.mkdir(exist_ok=True)
lock = open('/run/mirai-update.lock', 'w')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)

def write_status(state, version='', previous='', error=''):
    value = {'state': state, 'version': version, 'from': previous, 'error': error,
             'at': datetime.datetime.now(datetime.timezone.utc).isoformat()}
    temporary = STATE / 'status.json.tmp'
    temporary.write_text(json.dumps(value))
    temporary.replace(STATE / 'status.json')

def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def fetch(url):
    if not url.startswith('https://github.com/EnsixD/mirai/releases/'):
        raise ValueError('Release asset is outside the Mirai repository')
    with urllib.request.urlopen(urllib.request.Request(url, headers={'User-Agent': 'Mirai-native'}), timeout=90) as response:
        return response.read(180 * 1024 * 1024)

if '--requested' in sys.argv:
    (STATE / 'request').unlink(missing_ok=True)
if '--auto' in sys.argv:
    policy = json.loads((STATE / 'policy.json').read_text()) if (STATE / 'policy.json').exists() else {}
    if not policy.get('auto'):
        sys.exit(0)
    if policy.get('channel', 'stable') != 'stable':
        raise SystemExit('Native automatic updates currently support stable releases')

previous = (ROOT / 'VERSION').read_text().strip()
backup = None
version = ''
stopped = False
try:
    with tempfile.TemporaryDirectory(prefix='mirai-update-') as temporary:
        work = Path(temporary)
        raw = fetch('https://github.com/EnsixD/mirai/releases/latest/download/manifest.json')
        signature = fetch('https://github.com/EnsixD/mirai/releases/latest/download/manifest.json.sig')
        (work / 'manifest.json').write_bytes(raw)
        (work / 'signature').write_bytes(base64.b64decode(signature.strip(), validate=True))
        run('openssl', 'pkeyutl', '-verify', '-pubin', '-inkey', '/etc/mirai/release-public.pem',
            '-rawin', '-in', str(work / 'manifest.json'), '-sigfile', str(work / 'signature'), stdout=subprocess.DEVNULL)
        manifest = json.loads(raw)
        version = manifest['version']
        if tuple(map(int, version.split('.'))) <= tuple(map(int, previous.split('.'))):
            write_status('ok', previous, previous)
            sys.exit(0)
        asset = manifest['native']['x86_64']
        archive = fetch(asset['url'])
        if hashlib.sha256(archive).hexdigest() != asset['sha256']:
            raise ValueError('Native archive checksum mismatch')
        (work / 'native.tar.gz').write_bytes(archive)
        with tarfile.open(work / 'native.tar.gz') as bundle:
            members = bundle.getmembers()
            if {m.name for m in members} != {'mirai', 'mirai-node', 'VERSION'} or not all(m.isfile() for m in members):
                raise ValueError('Unexpected native archive contents')
            bundle.extractall(work / 'new', filter='data')
        if (work / 'new/VERSION').read_text().strip() != version:
            raise ValueError('Native archive version mismatch')
        environment = {}
        for line in Path('/etc/mirai/mirai.env').read_text().splitlines():
            if line and not line.startswith('#'):
                key, value = line.split('=', 1)
                environment[key] = value
        connection = urllib.parse.urlparse(environment['MIRAI_DATABASE_URL'])
        pg = dict(os.environ, PGHOST=connection.hostname, PGPORT=str(connection.port or 5432),
                  PGUSER=connection.username, PGPASSWORD=urllib.parse.unquote(connection.password),
                  PGDATABASE=connection.path.lstrip('/'))
        backup = Path('/var/backups/mirai') / datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
        backup.mkdir(parents=True, mode=0o700)
        for name in ('mirai', 'mirai-node', 'VERSION'):
            shutil.copy2(ROOT / name, backup / name)
        write_status('running', version, previous)
        run('systemctl', 'stop', 'mirai.service', 'mirai-node.service')
        stopped = True
        run('pg_dump', '-Fc', '-f', str(backup / 'database.dump'), env=pg)
        for name in ('mirai', 'mirai-node', 'VERSION'):
            shutil.copy2(work / 'new' / name, ROOT / (name + '.new'))
            if name != 'VERSION':
                (ROOT / (name + '.new')).chmod(0o755)
            (ROOT / (name + '.new')).replace(ROOT / name)
        run('systemctl', 'start', 'mirai-node.service', 'mirai.service')
        for attempt in range(30):
            time.sleep(1)
            result = subprocess.run([str(ROOT / 'mirai'), 'health'], env=dict(os.environ, **environment), capture_output=True)
            if result.returncode == 0:
                break
        else:
            raise RuntimeError('Updated panel did not become healthy')
        stopped = False
        write_status('ok', version, previous)
except Exception as error:
    if stopped and backup is not None:
        subprocess.run(['systemctl', 'stop', 'mirai.service', 'mirai-node.service'])
        for name in ('mirai', 'mirai-node', 'VERSION'):
            shutil.copy2(backup / name, ROOT / name)
        if (backup / 'database.dump').exists():
            run('pg_restore', '--clean', '--if-exists', '--no-owner', '--exit-on-error', '-d', pg['PGDATABASE'], str(backup / 'database.dump'), env=pg)
        run('systemctl', 'start', 'mirai-node.service', 'mirai.service')
    write_status('failed', version, previous, type(error).__name__ + ': update failed; see systemd journal')
    raise
