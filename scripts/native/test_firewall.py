import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('firewall', Path(__file__).with_name('firewall.py'))
firewall = importlib.util.module_from_spec(spec)
spec.loader.exec_module(firewall)

class FirewallTests(unittest.TestCase):
    def test_only_public_mirai_listeners_are_managed(self):
        text = '''tcp LISTEN 0 8192 *:8443 *:* users:(("mirai-node",pid=12,fd=7))
udp UNCONN 0 0 0.0.0.0:443 0.0.0.0:* users:(("mirai-node",pid=12,fd=8))
tcp LISTEN 0 8192 127.0.0.1:2053 0.0.0.0:* users:(("mirai",pid=13,fd=8))
tcp LISTEN 0 511 [::]:443 [::]:* users:(("nginx",pid=14,fd=7))
tcp LISTEN 0 128 *:22 *:* users:(("sshd",pid=15,fd=7))'''
        self.assertEqual(firewall.listening_ports(text), ({'8443/tcp', '443/udp'}, {'443/tcp', '22/tcp'}))

    def test_opens_new_ports_closes_removed_and_preserves_other_services(self):
        commands = []
        result = firewall.reconcile({'9443/tcp', '443/tcp'}, {'443/tcp'}, {'443/tcp', '8443/tcp'}, {'8443/tcp'}, lambda *args: commands.append(args))
        self.assertEqual(result, {'9443/tcp'})
        self.assertIn(('ufw', 'allow', '9443/tcp', 'comment', 'mirai-auto'), commands)
        self.assertIn(('ufw', '--force', 'delete', 'allow', '8443/tcp'), commands)
        self.assertEqual(len(commands), 2)

    def test_shared_port_is_kept_when_another_service_uses_it(self):
        commands = []
        self.assertEqual(firewall.reconcile(set(), {'8443/tcp'}, {'8443/tcp'}, {'8443/tcp'}, lambda *args: commands.append(args)), set())
        self.assertEqual(commands, [])

    def test_existing_ssh_and_web_rules_are_not_adopted(self):
        self.assertEqual(firewall.allowed_ports('22/tcp ALLOW Anywhere\n443 ALLOW Anywhere\n8443/tcp (v6) ALLOW Anywhere (v6)'), {'22/tcp', '443/tcp', '443/udp', '8443/tcp'})

if __name__ == '__main__':
    unittest.main()
