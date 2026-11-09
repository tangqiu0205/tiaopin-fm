#!/usr/bin/env python3
"""Package installed Windows files as a portable ZIP, without update artifacts."""
import argparse
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[1]


def package_portable(directory, output, executable, installer):
    for required in (executable, installer, ".mygo-files"):
        if not (directory / required).is_file():
            raise RuntimeError(f"missing build output: {directory / required}")
    # MyGo's installed-file inventory excludes installers, update archives and
    # user data. Keep it in the ZIP so future updates can remove obsolete files.
    entries = (directory / ".mygo-files").read_text(encoding="utf-8").splitlines()
    if executable not in entries:
        raise RuntimeError("executable is missing from installed-file inventory")
    files = {directory / ".mygo-files"}
    for entry in entries:
        relative = Path(entry)
        if not entry or relative.is_absolute() or ".." in relative.parts or relative.parts[0] in ("data", "portable.flag"):
            raise RuntimeError(f"invalid installed-file inventory entry: {entry!r}")
        path = directory / relative
        if not path.exists():
            raise RuntimeError(f"missing installed file: {path}")
        if path.is_dir():
            files.update(p for p in path.rglob("*") if p.is_file())
        else:
            files.add(path)
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(files):
            archive.write(path, path.relative_to(directory).as_posix())
        archive.writestr("portable.flag", "")
    print(f"Packaged {output}")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", required=True, choices=("windows/amd64", "windows/arm64"))
    args = parser.parse_args(argv)
    arch = args.platform.split("/")[1]
    config = json.loads((ROOT / "mygo.json").read_text(encoding="utf-8"))
    out = ROOT / config["out"]
    slug, version = config["slug"], config["version"]
    package_portable(out / f"windows-{arch}", out / f"{slug}-portable-{version}-windows-{arch}.zip",
                     config["name"] + ".exe", f"{slug}-setup-{version}-{arch}.exe")


if __name__ == "__main__":
    main()
