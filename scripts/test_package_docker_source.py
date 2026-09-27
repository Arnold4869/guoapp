import tarfile
import tempfile
import unittest
from pathlib import Path

from package_docker_source import create_package, package_version
from sync_source import REQUIRED_FILES


class DockerPackageTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='duanju-docker-package-')
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.source = self.root / 'source'
        self.output = self.root / 'out'
        for name in REQUIRED_FILES:
            self.write(self.source / name, 'version: 9.9.9+9\n' if name == 'pubspec.yaml' else 'source')
        self.write(self.source / 'native/core/provider.go', 'source')
        self.write(self.source / 'build/app.apk', 'artifact')
        self.write(self.source / 'dist/source.zip', 'artifact')
        self.write(self.source / '.env', 'TOKEN=secret')
        self.write(self.source / 'android/local.properties', 'sdk.dir=/opt/android')

    def write(self, path, content):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8')

    def names(self, archive):
        with tarfile.open(archive) as package:
            return sorted(package.getnames())

    def test_package_keeps_docker_inputs_and_drops_artifacts(self):
        result = create_package(self.source, self.output)
        self.assertEqual(result.archive.name, 'zhenguojian-docker-source-9.9.9+9.tar.gz')
        self.assertTrue(result.archive.is_file())
        self.assertEqual(len(result.digest), 64)
        names = self.names(result.archive)
        for required in ['zhenguojian/Dockerfile', 'zhenguojian/docker-compose.yml', 'zhenguojian/.env.example',
                         'zhenguojian/native/server/main.go', 'zhenguojian/native/go.mod']:
            self.assertIn(required, names)
        for excluded in ['zhenguojian/build/app.apk', 'zhenguojian/dist/source.zip', 'zhenguojian/.env',
                         'zhenguojian/android/local.properties']:
            self.assertNotIn(excluded, names)

    def test_version_must_be_readable(self):
        self.write(self.source / 'pubspec.yaml', 'name: duanju_app\n')
        with self.assertRaises(ValueError):
            package_version(self.source)

    def test_package_is_reproducible(self):
        first = create_package(self.source, self.output)
        second = create_package(self.source, self.output)
        self.assertEqual(first.digest, second.digest)


if __name__ == '__main__':
    unittest.main()
