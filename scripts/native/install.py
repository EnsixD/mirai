#!/usr/bin/env python3
"""Install verified Mirai binaries as a panel or a remote node, without Docker."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import platform
import secrets
import shutil
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.request

PUBLIC_KEY = 'OCOOJzn+R2XICC8i5jCInj4vwKyrSEa5PN5eigk8Mcg='
REPO = 'https://github.com/EnsixD/mirai/releases/'

def run(*args, **kwargs):
    return subprocess.run(args, check=True, **kwargs)

def fetch(url):
    if not url.startswith(REPO):
        raise ValueError('Unexpected release URL')
    with urllib.request.urlopen(urllib.request.Request(url, headers={'User-Agent': 'Mirai-installer'}), timeout=120) as response:
        return response.read(180 * 1024 * 1024)

def write(path, text, mode=0o644):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    path.chmod(mode)

def service(name, command, memory, extra=''):
    write(f'/etc/systemd/system/{name}.service', f'''[Unit]
Description=Mirai {name}
After=network-online.target postgresql.service
Wants=network-online.target
[Service]
User=mirai
Group=mirai
EnvironmentFile=/etc/mirai/mirai.env
Environment=GOMAXPROCS=1
Environment=GOMEMLIMIT={memory}MiB
ExecStart={command}
Restart=on-failure
RestartSec=3
RuntimeDirectory=mirai
RuntimeDirectoryPreserve=yes
WorkingDirectory=/var/lib/mirai
MemoryMax={memory + 64}M
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/mirai /run/mirai
PrivateTmp=yes
LimitNOFILE=65536
{extra}
[Install]
WantedBy=multi-user.target
''')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--domain', default='')
    parser.add_argument('--public-host', default='')
    parser.add_argument('--lang', choices=['en', 'ru'], default='en')
    parser.add_argument('--email', default='')
    parser.add_argument('--join', default='')
    parser.add_argument('--yes', action='store_true')
    parser.add_argument('--tag', default='latest')
    parser.add_argument('--credentials-file', default='')
    parser.add_argument('--repair-existing', action='store_true', help='Finish an incomplete setup; preserve its database credentials')
    args = parser.parse_args()
    if os.geteuid() != 0:
        raise SystemExit('Run this installer as root')
    arch = {'x86_64': 'x86_64', 'aarch64': 'aarch64'}.get(platform.machine())
    if not arch:
        raise SystemExit('Supported architectures: amd64 and arm64')
    existing = {}
    if Path('/etc/mirai/mirai.env').exists() and not args.repair_existing:
        raise SystemExit('Mirai is already installed. Use mirai update; configuration is preserved.')
    if args.repair_existing and Path('/etc/mirai/mirai.env').exists():
        existing = dict(line.split('=', 1) for line in Path('/etc/mirai/mirai.env').read_text().splitlines() if '=' in line)
    if not args.yes and not args.join:
        try:
            terminal = open('/dev/tty', 'r+')
        except OSError:
            raise SystemExit('No interactive terminal: use --yes --lang en --domain vpn.example.com')
        with terminal:
            if not args.domain:
                terminal.write('Panel domain (optional): '); terminal.flush()
                args.domain = terminal.readline().strip()
            terminal.write('Language [en/ru, default en]: '); terminal.flush()
            args.lang = terminal.readline().strip() or 'en'
        if args.lang not in ('en', 'ru'): raise SystemExit('Language must be en or ru')
    host = args.public_host
    if not args.join and not host:
        with urllib.request.urlopen('https://api.ipify.org', timeout=15) as response:
            host = response.read(100).decode().strip()
    if args.domain:
        if any(c not in 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-' for c in args.domain):
            raise SystemExit('Invalid domain')
        ips = {item[4][0] for item in socket.getaddrinfo(args.domain, None)}
        if host not in ips:
            raise SystemExit(f'Domain A/AAAA record must point to this server ({host})')
    packages = ['ca-certificates', 'openssl', 'python3', 'curl']
    if not args.join:
        packages += ['postgresql', 'postgresql-client', 'nginx', 'certbot']
    missing = [p for p in packages if subprocess.run(['dpkg-query', '-W', '-f=${Status}', p], capture_output=True, text=True).stdout.strip() != 'install ok installed']
    if missing:
        run('apt-get', 'update', '-qq')
        run('apt-get', 'install', '-y', *missing, env={**os.environ, 'DEBIAN_FRONTEND': 'noninteractive'})
    if subprocess.run(['id', 'mirai'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode != 0:
        run('useradd', '--system', '--user-group', '--home-dir', '/var/lib/mirai', '--shell', '/usr/sbin/nologin', 'mirai')
    Path('/opt/mirai').mkdir(parents=True, exist_ok=True)
    Path('/var/lib/mirai/update').mkdir(parents=True, exist_ok=True)
    run('chown', '-R', 'mirai:mirai', '/var/lib/mirai')
    Path('/etc/mirai').mkdir(exist_ok=True)
    Path('/etc/mirai').chmod(0o750)
    run('chown', 'root:mirai', '/etc/mirai')
    # Ed25519 SubjectPublicKeyInfo prefix, followed by the raw trusted public key.
    der = bytes.fromhex('302a300506032b6570032100') + base64.b64decode(PUBLIC_KEY)
    pem = '-----BEGIN PUBLIC KEY-----\n' + base64.b64encode(der).decode() + '\n-----END PUBLIC KEY-----\n'
    write('/etc/mirai/release-public.pem', pem)
    base = REPO + ('latest/download/' if args.tag == 'latest' else f'download/{args.tag}/')
    with tempfile.TemporaryDirectory() as temporary:
        work = Path(temporary)
        raw = fetch(base + 'manifest.json')
        (work / 'manifest').write_bytes(raw)
        (work / 'signature').write_bytes(base64.b64decode(fetch(base + 'manifest.json.sig').strip(), validate=True))
        run('openssl', 'pkeyutl', '-verify', '-pubin', '-inkey', '/etc/mirai/release-public.pem', '-rawin', '-in', str(work / 'manifest'), '-sigfile', str(work / 'signature'))
        manifest = json.loads(raw)
        asset = manifest['native'][arch]
        archive = fetch(asset['url'])
        if hashlib.sha256(archive).hexdigest() != asset['sha256']:
            raise SystemExit('Release checksum mismatch')
        (work / 'bundle').write_bytes(archive)
        with tarfile.open(work / 'bundle') as bundle:
            if {m.name for m in bundle.getmembers()} != {'mirai', 'mirai-node', 'VERSION'} or not all(m.isfile() for m in bundle.getmembers()):
                raise SystemExit('Invalid archive contents')
            # Only explicit regular files are extracted; no paths, links or devices.
            for member in bundle.getmembers():
                target = Path('/opt/mirai') / member.name
                pending = target.with_name(target.name + '.new')
                pending.write_bytes(bundle.extractfile(member).read())
                pending.chmod(0o644 if member.name == 'VERSION' else 0o755)
                pending.replace(target)
        if Path('/opt/mirai/VERSION').read_text().strip() != manifest['version']:
            raise SystemExit('Archive version mismatch')
        for name in ['update.py', 'firewall.py']:
            data = fetch(base + name)
            if hashlib.sha256(data).hexdigest() != manifest['files'][name]:
                raise SystemExit('Updater checksum mismatch')
            (Path('/opt/mirai') / name).write_bytes(data)
            (Path('/opt/mirai') / name).chmod(0o755)
    env = {'MIRAI_DATA_DIR': '/var/lib/mirai', 'MIRAI_NODE_SOCKET': '/run/mirai/node.sock'}
    if args.join:
        env['MIRAI_NODE_JOIN'] = args.join
        env['MIRAI_NODE_API_LISTEN'] = '0.0.0.0'
    else:
        password = secrets.token_hex(24)
        sql = f"CREATE ROLE mirai LOGIN PASSWORD '{password}';\nCREATE DATABASE mirai OWNER mirai;\n"
        run('systemctl', 'enable', '--now', 'postgresql')
        if not existing.get('MIRAI_DATABASE_URL'):
            run('runuser', '-u', 'postgres', '--', 'psql', '-v', 'ON_ERROR_STOP=1', input=sql, text=True, stdout=subprocess.DEVNULL)
        env.update(MIRAI_DATABASE_URL=existing.get('MIRAI_DATABASE_URL') or f'postgres://mirai:{password}@127.0.0.1:5432/mirai?sslmode=disable', MIRAI_PANEL_LISTEN='127.0.0.1:2053', MIRAI_DEV='1', MIRAI_TRUST_PROXY='1', MIRAI_ACME_LISTEN='127.0.0.1:2080')
    write('/etc/mirai/mirai.env', ''.join(f'{k}={v}\n' for k, v in env.items()), 0o640)
    run('chown', 'root:mirai', '/etc/mirai/mirai.env')
    service('mirai-node', '/opt/mirai/mirai-node', 384, 'AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_ADMIN\nCapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_ADMIN')
    if args.join:
        run('/opt/mirai/mirai-node', 'key-port', env={**os.environ, **env}, stdout=subprocess.DEVNULL)
    else:
        credentials = run('/opt/mirai/mirai', 'admin', 'bootstrap', '--username', 'admin', '--public-host', host, '--port', '443' if args.domain else '80', '--domain', args.domain, '--lang', args.lang, env={**os.environ, **env}, capture_output=True, text=True).stdout
        if not args.domain: credentials = credentials.replace('https://', 'http://')
        credential_path = args.credentials_file or '/root/mirai-login.txt'
        write(credential_path, credentials, 0o600)
        service('mirai', '/opt/mirai/mirai serve', 192)
        name = args.domain or host
        cert = Path(f'/etc/letsencrypt/live/{args.domain}') if args.domain else None
        site = f'''server {{
 listen 80;
 server_name {name};
 location ^~ /.well-known/acme-challenge/ {{ root /var/www/html; }}
 location / {{ proxy_pass http://127.0.0.1:2053; proxy_set_header Host $host; proxy_set_header X-Forwarded-For $remote_addr; proxy_set_header X-Forwarded-Proto $scheme; }}
}}
'''
        site_path = f'/etc/nginx/sites-available/mirai-{name}'
        write(site_path, site)
        enabled = Path(f'/etc/nginx/sites-enabled/mirai-{name}')
        if not enabled.exists(): enabled.symlink_to(site_path)
        run('nginx', '-t')
        run('systemctl', 'reload', 'nginx')
        if args.domain and not (cert / 'fullchain.pem').exists():
            run('certbot', 'certonly', '--webroot', '-w', '/var/www/html', '--non-interactive', '--agree-tos', '--domain', args.domain, *(['--email', args.email] if args.email else ['--register-unsafely-without-email']))
        if args.domain:
            custom = Path('/var/lib/mirai/tls/custom')
            custom.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(cert / 'fullchain.pem', custom / 'custom.crt')
            shutil.copyfile(cert / 'privkey.pem', custom / 'custom.key')
            (custom / 'custom.key').chmod(0o600)
            run('chown', '-R', 'mirai:mirai', '/var/lib/mirai')
            site += f'''server {{
 listen 443 ssl;
 server_name {name};
 ssl_certificate {cert}/fullchain.pem;
 ssl_certificate_key {cert}/privkey.pem;
 location / {{ proxy_pass http://127.0.0.1:2053; proxy_set_header Host $host; proxy_set_header X-Forwarded-For $remote_addr; proxy_set_header X-Forwarded-Proto https; }}
}}
'''
            write(site_path, site)
            write('/etc/letsencrypt/renewal-hooks/deploy/mirai-cert', f'''#!/bin/sh
set -eu
[ "$RENEWED_LINEAGE" = "{cert}" ] || exit 0
install -o mirai -g mirai -m 600 "$RENEWED_LINEAGE/privkey.pem" "{custom}/custom.key.new"
install -o mirai -g mirai -m 600 "$RENEWED_LINEAGE/fullchain.pem" "{custom}/custom.crt.new"
mv "{custom}/custom.key.new" "{custom}/custom.key"
mv "{custom}/custom.crt.new" "{custom}/custom.crt"
systemctl kill -s HUP mirai
systemctl reload nginx
''', 0o755)
        # Stored settings are JSON strings in a text column.
        settings = {'brand': 'Mirai', 'sub_public_url': f'{"https" if args.domain else "http"}://{name}', 'sub_id_length': 9}
        sql = ''.join("INSERT INTO settings(key,value) VALUES ('"+k+"','"+json.dumps(v)+"') ON CONFLICT(key) DO UPDATE SET value=excluded.value;\n" for k,v in settings.items())
        run('runuser', '-u', 'postgres', '--', 'psql', '-d', 'mirai', '-v', 'ON_ERROR_STOP=1', input=sql, text=True, stdout=subprocess.DEVNULL)
        run('nginx', '-t')
        run('systemctl', 'reload', 'nginx')
    write('/usr/local/bin/mirai', '''#!/bin/sh
set -a
. /etc/mirai/mirai.env
set +a
case "$1" in
 logs) shift; if [ "$1" = node ]; then shift; exec journalctl -u mirai-node "$@"; else exec journalctl -u mirai "$@"; fi ;;
 status) if [ -n "$MIRAI_DATABASE_URL" ]; then exec systemctl status mirai mirai-node --no-pager; else exec systemctl status mirai-node --no-pager; fi ;;
 update) exec python3 /opt/mirai/update.py ;;
 *) exec /opt/mirai/mirai "$@" ;;
esac
''', 0o755)
    if not args.join:
        write('/etc/systemd/system/mirai-update.service', '[Unit]\nDescription=Verified Mirai update\n[Service]\nType=oneshot\nExecStart=/usr/bin/python3 /opt/mirai/update.py --requested\n')
        write('/etc/systemd/system/mirai-update.path', '[Unit]\nDescription=Mirai update requests\n[Path]\nPathExists=/var/lib/mirai/update/request\n[Install]\nWantedBy=multi-user.target\n')
        write('/etc/systemd/system/mirai-update-auto.service', '[Service]\nType=oneshot\nExecStart=/usr/bin/python3 /opt/mirai/update.py --auto\n')
        write('/etc/systemd/system/mirai-update.timer', '[Timer]\nOnCalendar=daily\nRandomizedDelaySec=1h\nUnit=mirai-update-auto.service\n[Install]\nWantedBy=timers.target\n')
    run('systemctl', 'daemon-reload')
    run('python3', '/opt/mirai/firewall.py', '--install-service')
    run('systemctl', 'enable', '--now', 'mirai-node')
    if not args.join:
        run('systemctl', 'enable', '--now', 'mirai', 'mirai-update.path', 'mirai-update.timer')
        for _ in range(30):
            if subprocess.run(['/opt/mirai/mirai', 'health'], env={**os.environ, **env}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0: break
            time.sleep(1)
        else: raise SystemExit('Panel did not become healthy; check journalctl -u mirai')
        if not args.domain: print('No domain configured: choose a REALITY camouflage target explicitly before publishing inbounds.')
        print('Mirai installed successfully. Credentials saved to ' + credential_path)
        if not args.credentials_file: print(credentials)
    else:
        print('Mirai node installed. Connect it from the main panel; this server does not run a second admin panel.')

if __name__ == '__main__':
    main()
