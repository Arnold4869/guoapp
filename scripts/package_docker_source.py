import argparse
import hashlib
import re
import tarfile
from dataclasses import dataclass
from pathlib import Path

from sync_source import source_files

root = Path(__file__).resolve().parents[1]


@dataclass(frozen=True)
class DockerPackage:
    archive: Path
    files: int
    total_bytes: int
    digest: str


def package_version(source):
    match = re.search(r'^version:\s*([\w.+-]+)\s*$', (Path(source) / 'pubspec.yaml').read_text(encoding='utf-8'), re.MULTILINE)
    if not match:
        raise ValueError('pubspec.yaml 缺少合法版本号')
    return match.group(1)


def create_package(source, output, name='zhenguojian'):
    source = Path(source).resolve()
    output = Path(output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    archive = output / f'{name}-docker-source-{package_version(source)}.tar.gz'
    selected = source_files(source)
    total = 0
    with tarfile.open(archive, 'w:gz', compresslevel=9) as package:
        for relative in sorted(selected, key=lambda path: path.as_posix()):
            path = source / relative
            info = package.gettarinfo(str(path), arcname=f'{name}/{relative.as_posix()}')
            info.uid, info.gid, info.uname, info.gname = 0, 0, '', ''
            info.mtime = 0
            with path.open('rb') as stream:
                package.addfile(info, stream)
            total += info.size
    checksum = hashlib.sha256()
    with archive.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            checksum.update(block)
    return DockerPackage(archive, len(selected), total, checksum.hexdigest())


def main():
    parser = argparse.ArgumentParser(description='打包 Docker 部署源码，供他人直接构建镜像。')
    parser.add_argument('--output', type=Path, default=root / 'dist' / 'docker', help='输出目录')
    options = parser.parse_args()
    try:
        result = create_package(root, options.output)
    except (OSError, ValueError) as error:
        print(str(error))
        return 1
    print(f'已打包 Docker 部署源码：{result.files} 个文件，{result.total_bytes / 1024:.1f} KB')
    print('源码压缩包：' + str(result.archive))
    print('SHA-256：' + result.digest)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
