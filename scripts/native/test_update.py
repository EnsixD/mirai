"""Native update recovery regression tests; PostgreSQL tests use a disposable DB."""
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import urllib.parse
import uuid

spec = importlib.util.spec_from_file_location('native_update', Path(__file__).with_name('update.py'))
update = importlib.util.module_from_spec(spec)
spec.loader.exec_module(update)


class RecoveryTests(unittest.TestCase):
    def test_latest_fetch_bypasses_cached_redirect(self):
        url = 'https://github.com/EnsixD/mirai/releases/latest/download/manifest.json'
        with patch.object(update.urllib.request, 'urlopen') as open_url, patch.object(update.time, 'time_ns', side_effect=[101, 102]):
            update.fetch(url)
            update.fetch(url + '.sig')
        requests = [call.args[0] for call in open_url.call_args_list]
        self.assertEqual(requests[0].full_url, url + '?mirai_check=101')
        self.assertEqual(requests[1].full_url, url + '.sig?mirai_check=102')
        self.assertEqual(requests[0].get_header('Cache-control'), 'no-cache')

    def test_restore_failure_still_restarts_services(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            backup = Path(directory) / 'backup'
            root.mkdir()
            backup.mkdir()
            for name in ('mirai', 'mirai-node', 'VERSION', 'update.py'):
                (backup / name).write_text('old')
                (root / name).write_text('new')
            failure = RuntimeError('restore failed')
            with patch.object(update, 'ROOT', root), patch.object(update, 'run') as run, patch.object(update, 'restore_database', side_effect=failure):
                error = update.recover(backup, ['mirai-node', 'mirai'], {}, True, True)
            self.assertIs(error, failure)
            self.assertEqual(run.call_args_list[-1].args, ('systemctl', 'start', 'mirai-node', 'mirai'))
            self.assertEqual((root / 'mirai').read_text(), 'old')

    def test_failed_backup_does_not_restore_partial_dump(self):
        with patch.object(update, 'run') as run, patch.object(update, 'restore_database') as restore:
            self.assertIsNone(update.recover(Path('/unused'), ['mirai'], {}, False, False))
            restore.assert_not_called()
            self.assertEqual(run.call_args_list[-1].args, ('systemctl', 'start', 'mirai'))


@unittest.skipUnless(os.environ.get('MIRAI_TEST_DATABASE_URL'), 'PostgreSQL test URL not configured')
class DatabaseRecoveryTests(unittest.TestCase):
    def setUp(self):
        connection = urllib.parse.urlparse(os.environ['MIRAI_TEST_DATABASE_URL'])
        self.environment = dict(os.environ, PGHOST=connection.hostname, PGPORT=str(connection.port or 5432),
                                PGUSER=urllib.parse.unquote(connection.username),
                                PGPASSWORD=urllib.parse.unquote(connection.password), PGDATABASE=connection.path.lstrip('/'))
        self.name = 'mirai_update_test_' + uuid.uuid4().hex
        self.sql('CREATE DATABASE ' + self.name)
        self.environment['PGDATABASE'] = self.name
        self.temporary = tempfile.TemporaryDirectory()
        self.dump = Path(self.temporary.name) / 'before.dump'
        self.sql('CREATE TABLE bound_devices(id bigint PRIMARY KEY); INSERT INTO bound_devices VALUES (42)')
        update.run('pg_dump', '-Fc', '-f', str(self.dump), env=self.environment)
        self.sql('CREATE TABLE bound_device_traffic(device_id bigint REFERENCES bound_devices(id)); INSERT INTO bound_device_traffic VALUES (42)')

    def tearDown(self):
        self.environment['PGDATABASE'] = 'postgres'
        self.sql('DROP DATABASE ' + self.name + ' WITH (FORCE)')
        self.temporary.cleanup()

    def sql(self, sql):
        return subprocess.run(['psql', '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-c', sql],
                              env=self.environment, capture_output=True, text=True, check=True).stdout.strip()

    def test_new_foreign_keys_do_not_block_old_database_restore(self):
        update.restore_database(self.dump, self.environment)
        self.assertEqual(self.sql('SELECT id FROM bound_devices'), '42')
        self.assertEqual(self.sql("SELECT to_regclass('public.bound_device_traffic') IS NULL"), 't')

    def test_restore_error_preserves_current_schema_and_data(self):
        real_run = update.run

        def inject_error(*args, **kwargs):
            result = real_run(*args, **kwargs)
            if args[0] == 'pg_restore' and '-f' in args:
                with Path(args[args.index('-f') + 1]).open('a') as target:
                    target.write('\nSELECT 1 / 0;\n')
            return result

        with patch.object(update, 'run', side_effect=inject_error):
            with self.assertRaises(subprocess.CalledProcessError):
                update.restore_database(self.dump, self.environment)
        self.assertEqual(self.sql('SELECT device_id FROM bound_device_traffic'), '42')
        self.assertEqual(self.sql('SELECT id FROM bound_devices'), '42')


if __name__ == '__main__':
    unittest.main()
