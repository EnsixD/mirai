#!/usr/bin/env python3
"""Signed native Mirai updates, with binary and PostgreSQL rollback."""
import base64
import datetime
import hashlib
import json
import os
import platform
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.parse
import urllib.request

ROOT = Path('/opt/mirai')
DATA = Path('/var/lib/mirai')
STATE = DATA / 'update'

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
    if '/releases/latest/' in url:
        url += ('&' if '?' in url else '?') + 'mirai_check=' + str(time.time_ns())
    with urllib.request.urlopen(urllib.request.Request(url, headers={'User-Agent': 'Mirai-native', 'Cache-Control': 'no-cache'}), timeout=90) as response:
        return response.read(180 * 1024 * 1024)

def restore_database(dump, environment):
    """Restore the pre-update schema and data together, including removed migrations.

    pg_restore --clean cannot drop old keys referenced by newly migrated tables.
    Reset only Mirai's public schema, in the same transaction as the restore: a
    failed restore leaves the current database intact instead of half restored.
    """
    listing = run('pg_restore', '-l', str(dump), capture_output=True, text=True).stdout
    has_public_schema = any(' SCHEMA - public ' in line for line in listing.splitlines())
    with tempfile.TemporaryDirectory(prefix='mirai-restore-') as temporary:
        sql = Path(temporary) / 'restore.sql'
        run('pg_restore', '--no-owner', '--no-privileges', '-f', str(sql), str(dump))
        transaction = Path(temporary) / 'transaction.sql'
        with transaction.open('wb') as target, sql.open('rb') as source:
            target.write(b'DROP SCHEMA public CASCADE;\n')
            if not has_public_schema:
                target.write(b'CREATE SCHEMA public;\n')
            shutil.copyfileobj(source, target)
        run('psql', '-X', '--single-transaction', '-v', 'ON_ERROR_STOP=1',
            '-f', str(transaction), env=environment, stdout=subprocess.DEVNULL)

def recover(backup, services, pg, installed, database_backed_up):
    recovery_error = None
    try:
        run('systemctl', 'stop', *services)
        if installed:
            for name in ('mirai', 'mirai-node', 'VERSION', 'update.py'):
                target = ROOT / (name + '.rollback')
                shutil.copy2(backup / name, target)
                target.replace(ROOT / name)
            if database_backed_up:
                restore_database(backup / 'database.dump', pg)
    except Exception as rollback_error:
        recovery_error = rollback_error
    finally:
        # Even a failed database restore must not leave both services stopped.
        try:
            run('systemctl', 'start', *services)
        except Exception as restart_error:
            recovery_error = restart_error
    return recovery_error

def main():
    import fcntl
    STATE.mkdir(exist_ok=True)
    lock = open('/run/mirai-update.lock', 'w')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
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
    database_backed_up = False
    installed = False
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
            arch = {'x86_64': 'x86_64', 'aarch64': 'aarch64'}[platform.machine()]
            asset = manifest['native'][arch]
            archive = fetch(asset['url'])
            if hashlib.sha256(archive).hexdigest() != asset['sha256']:
                raise ValueError('Native archive checksum mismatch')
            (work / 'native.tar.gz').write_bytes(archive)
            with tarfile.open(work / 'native.tar.gz') as bundle:
                members = bundle.getmembers()
                if {m.name for m in members} != {'mirai', 'mirai-node', 'VERSION'} or not all(m.isfile() for m in members):
                    raise ValueError('Unexpected native archive contents')
                (work / 'new').mkdir()
                for member in members:
                    (work / 'new' / member.name).write_bytes(bundle.extractfile(member).read())
            if (work / 'new/VERSION').read_text().strip() != version:
                raise ValueError('Native archive version mismatch')
            environment = {}
            for line in Path('/etc/mirai/mirai.env').read_text().splitlines():
                if line and not line.startswith('#'):
                    key, value = line.split('=', 1)
                    environment[key] = value
            services = ['mirai-node.service']
            pg = None
            if 'MIRAI_DATABASE_URL' in environment:
                services.append('mirai.service')
                connection = urllib.parse.urlparse(environment['MIRAI_DATABASE_URL'])
                pg = dict(os.environ, PGHOST=connection.hostname, PGPORT=str(connection.port or 5432),
                          PGUSER=connection.username, PGPASSWORD=urllib.parse.unquote(connection.password),
                          PGDATABASE=connection.path.lstrip('/'))
            updater = fetch(asset['url'].rsplit('/', 1)[0] + '/update.py')
            if hashlib.sha256(updater).hexdigest() != manifest['files']['update.py']:
                raise ValueError('Updater checksum mismatch')
            (work / 'new/update.py').write_bytes(updater)
            backup = Path('/var/backups/mirai') / datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
            backup.mkdir(parents=True, mode=0o700)
            for name in ('mirai', 'mirai-node', 'VERSION', 'update.py'):
                shutil.copy2(ROOT / name, backup / name)
            write_status('running', version, previous)
            stopped = True
            run('systemctl', 'stop', *services)
            if pg is not None:
                run('pg_dump', '-Fc', '-f', str(backup / 'database.dump'), env=pg)
                database_backed_up = True
            installed = True
            for name in ('mirai', 'mirai-node', 'VERSION', 'update.py'):
                shutil.copy2(work / 'new' / name, ROOT / (name + '.new'))
                if name != 'VERSION':
                    (ROOT / (name + '.new')).chmod(0o755)
                (ROOT / (name + '.new')).replace(ROOT / name)
            run('systemctl', 'start', *services)
            for attempt in range(30):
                time.sleep(1)
                result = subprocess.run([str(ROOT / 'mirai'), 'health'] if pg is not None else ['systemctl', 'is-active', 'mirai-node'], env=dict(os.environ, **environment), capture_output=True)
                if result.returncode == 0:
                    break
            else:
                raise RuntimeError('Updated panel did not become healthy')
            stopped = False
            write_status('ok', version, previous)
    except Exception as error:
        recovery_error = None
        if stopped and backup is not None:
            recovery_error = recover(backup, services, pg, installed, database_backed_up)
        detail = type(error).__name__ + ': update failed; see systemd journal'
        if recovery_error is not None:
            detail += '; rollback failed: ' + type(recovery_error).__name__
        write_status('failed', version, previous, detail)
        if recovery_error is not None:
            raise RuntimeError('Update and recovery failed; see systemd journal') from recovery_error
        raise

if __name__ == '__main__':
    main()
