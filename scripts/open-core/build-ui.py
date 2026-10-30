#!/usr/bin/env python3
"""Build the public UI and embed it in the public Go binary's source tree."""
from pathlib import Path
import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile


NOTICE = re.compile(r'(?i)(?:licen[cs]e|copying|notice|copyright)(?:[._-].*)?')


def copy_notice(source, target):
    if source.is_symlink() or not source.is_file() or source.stat().st_size > 1024 * 1024:
        raise ValueError('Unexpected public dependency notice')
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, target)


def public_notices(root, ui, output, env):
    # The installed tree belongs to the independent allowlisted UI. No private
    # workspace node_modules or engine dependency graph can reach this export.
    destination = output / 'third-party'
    if destination.exists():
        raise ValueError('Public notice destination overlaps a source asset')
    npm = {}
    for source in sorted((ui / 'node_modules').rglob('*')):
        if source.is_file() and NOTICE.fullmatch(source.name):
            relative = str(source.relative_to(ui / 'node_modules'))
            name = hashlib.sha256(relative.encode()).hexdigest()[:24] + '.txt'
            copy_notice(source, destination / 'npm' / name)
            npm[relative] = name
    if not npm:
        raise ValueError('Public frontend dependency notices are missing')
    raw = subprocess.check_output(['go', 'list', '-mod=readonly', '-deps', '-json', './cmd/axottle-core'],
                                  cwd=root, env=dict(env, GOWORK='off'), text=True)
    decoder, offset, modules = json.JSONDecoder(), 0, {}
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset == len(raw):
            break
        package, offset = decoder.raw_decode(raw, offset)
        module = package.get('Module')
        if module and not module.get('Main'):
            if module.get('Replace') or not module.get('Version'):
                raise ValueError('Unreviewed public Go dependency replacement')
            modules[module['Path']] = module
    inventory = []
    for name, module in sorted(modules.items()):
        files = []
        for source in sorted(Path(module['Dir']).iterdir()):
            if source.is_file() and NOTICE.fullmatch(source.name):
                relative = Path('go') / (name + '@' + module['Version']) / (source.name + '.txt')
                copy_notice(source, destination / relative)
                files.append(str(relative))
        if not files:
            raise ValueError('Public Go dependency has no notice: ' + name)
        inventory.append({'module': name, 'version': module['Version'], 'notices': files})
    goroot = Path(subprocess.check_output(['go', 'env', 'GOROOT'], cwd=root, env=env, text=True).strip())
    copy_notice(goroot / 'LICENSE', destination / 'Go-LICENSE.txt')
    (destination / 'inventory.json').write_text(json.dumps({'npm': npm, 'go': inventory}, indent=2) + '\n')


def source_digest(ui):
    digest = hashlib.sha256()
    for file in sorted(ui.rglob('*')):
        relative = file.relative_to(ui)
        if any(part in ['node_modules', '.next', 'out', 'generated', '_assets'] for part in relative.parts):
            continue
        if file.name in ['next-env.d.ts', 'tsconfig.tsbuildinfo']:
            continue
        if file.is_symlink():
            raise ValueError('UI source symlinks are not allowed.')
        if file.is_file():
            digest.update(str(relative).encode() + b'\0' + file.read_bytes())
    return digest.hexdigest()


def main():
    root = Path(__file__).resolve().parents[2]
    ui = root / 'web/admin'
    static = root / 'internal/coreui/static'
    if not (root / 'internal/coreui/ui.go').is_file() or not (ui / 'package-lock.json').is_file():
        raise SystemExit('Run this command inside the exported open core source tree.')
    package = json.loads((ui / 'package.json').read_text())
    if package.get('name') != 'axottle-core-ui':
        raise SystemExit('Refusing to embed the private module UI into the public core.')
    if static.is_symlink():
        raise SystemExit('The core UI destination must not be a symlink.')
    actual = subprocess.check_output(['node', '--version'], text=True).strip().removeprefix('v')
    required = (ui / '.nvmrc').read_text().strip()
    if actual != required:
        raise SystemExit(f'Public UI build requires Node.js {required}; found {actual}.')
    env = dict(os.environ, NEXT_TELEMETRY_DISABLED='1', AXOTTLE_UI_BUILD_ID='core-' + source_digest(ui)[:24])
    subprocess.run(['npm', 'ci', '--ignore-scripts', '--no-audit', '--no-fund'], cwd=ui, env=env, check=True)
    subprocess.run(['npm', 'run', 'build'], cwd=ui, env=env, check=True)
    output = ui / 'out'
    for route in ['index.html', 'login.html', 'setup.html', 'new.html', 'new/projects.html', 'new/accounts.html', 'new/settings.html', 'new/audit-log.html', 'new/profile.html']:
        if not (output / route).is_file():
            raise SystemExit('Public UI route was not built: ' + route)
    for file in output.rglob('*'):
        if file.is_symlink() or file.suffix == '.map':
            raise SystemExit('Unexpected symlink or source map in the public UI build.')
    public_notices(root, ui, output, env)
    with tempfile.TemporaryDirectory(prefix='.public-ui-', dir=static.parent) as temporary:
        staged = Path(temporary) / 'static'
        shutil.copytree(output, staged)
        if (static / '.gitkeep').is_file():
            (staged / '.gitkeep').touch()
        previous = Path(temporary) / 'previous'
        if static.exists():
            static.rename(previous)
        try:
            staged.rename(static)
        except BaseException:
            if previous.exists():
                previous.rename(static)
            raise
    print('Public UI embedded; build the binary with go build ./cmd/axottle-core.')


if __name__ == '__main__':
    main()
