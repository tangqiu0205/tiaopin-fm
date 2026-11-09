#!/usr/bin/env python3
"""Prepare pinned mpv-audio runtime bundles; no native code is downloaded at runtime."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import stat
import tempfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
LIBRARIES = {"windows": "mpv-2.dll", "darwin": "libmpv.dylib", "linux": "libmpv.so.2"}
METADATA_FILES = {"manifest.json", "native-lock.json", "BUILD-README.md", "smoke-report.json", "sdk-report.json"}


def host_platform():
    system = {"Windows": "windows", "Darwin": "darwin", "Linux": "linux"}[platform.system()]
    arch = {"AMD64": "amd64", "x86_64": "amd64", "arm64": "arm64", "ARM64": "arm64", "aarch64": "arm64"}[platform.machine()]
    return f"{system}/{arch}"


def sha256(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def read_lock():
    lock = json.loads((ROOT / "scripts/mpv.lock.json").read_text(encoding="utf-8"))
    if lock.get("schema") != 1:
        raise RuntimeError("unsupported mpv lock schema")
    return lock


def download_archive(lock, asset):
    digest = asset["sha256"]
    if not re.fullmatch(r"[0-9a-f]{64}", digest):
        raise RuntimeError("invalid locked archive SHA-256")
    name = asset["asset"]
    if PurePosixPath(name).name != name or "\\" in name or ":" in name:
        raise RuntimeError("invalid locked asset name")
    cache = ROOT / ".mygo/mpv"
    cache.mkdir(parents=True, exist_ok=True)
    archive = cache / (digest + ".zip")
    if archive.exists():
        if sha256(archive) != digest:
            raise RuntimeError(f"cached libmpv archive SHA-256 mismatch; remove {archive} and retry")
        return archive
    url = f"https://github.com/{lock['repository']}/releases/download/{lock['tag']}/{name}"
    print(f"Downloading {lock['tag']}: {name}", flush=True)
    with tempfile.TemporaryDirectory(prefix="download-", dir=cache) as tmp:
        part = Path(tmp) / "bundle.zip"
        request = urllib.request.Request(url, headers={"User-Agent": "TiaopinFM-native-preparation"})
        with urllib.request.urlopen(request, timeout=120) as response, part.open("wb") as stream:
            shutil.copyfileobj(response, stream)
        if sha256(part) != digest:
            raise RuntimeError(f"libmpv archive SHA-256 mismatch: {name}")
        part.replace(archive)
    return archive


def safe_entries(bundle):
    entries = []
    seen = set()
    for item in bundle.infolist():
        original = item.orig_filename
        path = PurePosixPath(original)
        if (path.is_absolute() or ".." in path.parts or "\\" in original or ":" in original
                or "\x00" in original or not path.parts or path.parts[0] != "mpv"):
            raise RuntimeError(f"unsafe archive path: {original!r}")
        key = path.as_posix().casefold()
        if key in seen:
            raise RuntimeError(f"duplicate archive path: {original}")
        seen.add(key)
        kind = stat.S_IFMT(item.external_attr >> 16)
        if kind not in (0, stat.S_IFREG, stat.S_IFDIR):
            raise RuntimeError(f"unsupported archive entry type: {original}")
        if not item.is_dir():
            if len(path.parts) < 2:
                raise RuntimeError(f"unsafe archive path: {original!r}")
            entries.append((item, path))
    return entries


def extract_runtime(archive, dest, target, lock):
    with zipfile.ZipFile(archive) as bundle:
        entries = safe_entries(bundle)
        manifest = json.loads(bundle.read("mpv/manifest.json"))
        if (manifest.get("platform") != target or manifest.get("mpv_version") != lock["version"]
                or manifest.get("profile") != "audio" or manifest.get("license_profile") != "lgpl"):
            raise RuntimeError("libmpv bundle platform, version or build profile mismatch")
        libraries = manifest["libraries"]
        if not libraries or LIBRARIES[target.split("/")[0]] not in libraries:
            raise RuntimeError("libmpv primary library is missing from manifest")
        for name, digest in libraries.items():
            if (name == ".." or PurePosixPath(name).name != name or "\\" in name or ":" in name
                    or not re.fullmatch(r"[0-9a-f]{64}", digest)):
                raise RuntimeError(f"invalid library manifest entry: {name}")
        for item, path in entries:
            relative = path.relative_to("mpv")
            root_file = len(relative.parts) == 1
            keep = (root_file and (relative.name in libraries or relative.name in METADATA_FILES)) or relative.parts[0] == "licenses"
            if not keep:
                continue  # C headers, import libraries and examples are not runtime resources.
            output = dest.joinpath(*relative.parts)
            output.parent.mkdir(parents=True, exist_ok=True)
            with bundle.open(item) as source, output.open("wb") as stream:
                shutil.copyfileobj(source, stream)
            output.chmod((item.external_attr >> 16) & 0o777 or 0o644)
    for name, digest in libraries.items():
        library = dest / name
        if not library.is_file() or sha256(library) != digest:
            raise RuntimeError(f"libmpv library SHA-256 mismatch: {name}")
    if not (dest / "licenses").is_dir() or not any((dest / "licenses").rglob("*")):
        raise RuntimeError("libmpv license material is missing")
    origin = json.loads((dest / "native-lock.json").read_text(encoding="utf-8"))
    if (origin["mpv"]["version"] != lock["version"] or origin["release"]["tag"] != lock["tag"]
            or origin["release"]["repository"] != lock["repository"]):
        raise RuntimeError("libmpv source lock does not match the pinned release")
    if target.startswith("darwin/"):
        minimum = manifest["macos_minimum"]
        configured = json.loads((ROOT / "mygo.json").read_text(encoding="utf-8"))["macos"]["minimumSystemVersion"]
        if version_tuple(minimum) > version_tuple(configured):
            raise RuntimeError(f"libmpv requires macOS {minimum}, above configured {configured}")
    return manifest


def version_tuple(value):
    parts = tuple(map(int, value.split(".")))
    return parts + (0,) * (3 - len(parts))


def prepare(target):
    lock = read_lock()
    if target not in lock["platforms"] or target.split("/")[0] not in LIBRARIES:
        raise RuntimeError(f"unsupported libmpv platform: {target}")
    asset = lock["platforms"][target]
    archive = download_archive(lock, asset)
    parent = ROOT / "resources" / target.replace("/", "-")
    parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="mpv-stage-", dir=parent) as tmp:
        dest = Path(tmp) / "mpv"
        dest.mkdir()
        extract_runtime(archive, dest, target, lock)
        (dest / "bundle.json").write_text(json.dumps({"schema": 1, "repository": lock["repository"],
            "tag": lock["tag"], "channel": lock["channel"], "platform": target,
            "asset": asset["asset"], "archive_sha256": asset["sha256"]}, indent=2) + "\n", encoding="utf-8")
        shutil.copy2(ROOT / "THIRD_PARTY_NOTICES.md", dest / "THIRD_PARTY_NOTICES.md")
        final = parent / "mpv"
        # A backup outside the temporary directory survives even a failed rollback.
        backup = parent / (Path(tmp).name + "-previous")
        if final.exists():
            final.rename(backup)
        try:
            dest.rename(final)
        except OSError:
            if backup.exists():
                backup.rename(final)
            raise
        if backup.exists():
            shutil.rmtree(backup)
    print(f"Prepared {target}: {lock['tag']} -> {final}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", default=host_platform(), help="comma-separated GOOS/GOARCH targets")
    args = parser.parse_args()
    for target in args.platform.split(","):
        prepare(target)


if __name__ == "__main__":
    main()
