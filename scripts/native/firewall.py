#!/usr/bin/env python3
"""Reconcile UFW rules with public ports actually listening in Mirai processes."""
import json
from pathlib import Path
import re
import shutil
import subprocess

STATE = Path('/var/lib/mirai-firewall/ports.json')

def install_service():
    directory = Path('/etc/systemd/system')
    (directory / 'mirai-firewall.service').write_text('[Unit]\nDescription=Mirai inbound firewall rules\nAfter=mirai-node.service\n[Service]\nType=oneshot\nExecStart=/usr/bin/python3 /opt/mirai/firewall.py\n')
    (directory / 'mirai-firewall.timer').write_text('[Unit]\nDescription=Synchronize Mirai ports\n[Timer]\nOnBootSec=5s\nOnUnitActiveSec=5s\nAccuracySec=1s\n[Install]\nWantedBy=timers.target\n')
    subprocess.run(['systemctl', 'daemon-reload'], check=True)
    subprocess.run(['systemctl', 'enable', '--now', 'mirai-firewall.timer'], check=True)

def listening_ports(text):
    desired, protected = set(), set()
    for line in text.splitlines():
        fields = line.split()
        if len(fields) < 6 or fields[0] not in ('tcp', 'udp'):
            continue
        address = fields[4]
        host, _, port = address.rpartition(':')
        if host.strip('[]') in ('127.0.0.1', '::1') or not port.isdigit():
            continue
        port = int(port)
        if not 1 <= port <= 65535:
            continue
        key = f'{port}/{fields[0]}'
        processes = re.findall(r'\("([^"]+)"', line)
        if not processes:
            # An unidentified public listener must not lose its existing rule.
            protected.add(key)
        elif all(name in ('mirai', 'mirai-node') for name in processes):
            desired.add(key)
        else:
            protected.add(key)
    return desired, protected

def allowed_ports(text):
    ports = set()
    for line in text.splitlines():
        match = re.match(r'^\s*(\d+)(?:/(tcp|udp))?\s+(?:\(v6\)\s+)?ALLOW\b', line)
        if match:
            for network in ([match[2]] if match[2] else ['tcp', 'udp']):
                ports.add(f'{int(match[1])}/{network}')
    return ports

def reconcile(desired, protected, existing, managed, command):
    result = set(managed)
    for port in sorted(desired - managed):
        if port not in existing:
            command('ufw', 'allow', port, 'comment', 'mirai-auto')
            result.add(port)
    for port in sorted(managed - desired):
        if port not in protected:
            command('ufw', '--force', 'delete', 'allow', port)
        result.discard(port)
    return result

def main():
    if not shutil.which('ufw'):
        return
    status = subprocess.run(['ufw', 'status'], capture_output=True, text=True, check=True).stdout
    if 'Status: active' not in status:
        return
    listeners = subprocess.run(['ss', '-H', '-lntu', '-p'], capture_output=True, text=True, check=True).stdout
    desired, protected = listening_ports(listeners)
    managed = set(json.loads(STATE.read_text())) if STATE.exists() else set()
    if any(not re.fullmatch(r'(?:[1-9]\d{0,4})/(?:tcp|udp)', port) or int(port.split('/')[0]) > 65535 for port in managed):
        raise ValueError('Invalid managed firewall state')
    def command(*args):
        subprocess.run(args, check=True, stdout=subprocess.DEVNULL)
    result = reconcile(desired, protected, allowed_ports(status), managed, command)
    if result != managed or not STATE.exists():
        STATE.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        temporary = STATE.with_suffix('.tmp')
        temporary.write_text(json.dumps(sorted(result)))
        temporary.chmod(0o600)
        temporary.replace(STATE)

if __name__ == '__main__':
    import sys
    if '--install-service' in sys.argv:
        install_service()
    else:
        main()
