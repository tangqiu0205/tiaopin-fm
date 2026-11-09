import hashlib
import io
import json
from pathlib import Path
import stat
import tempfile
import unittest
from contextlib import contextmanager
from unittest.mock import patch
import zipfile

import prepare_mpv as mpv


def runtime_archive(target="windows/amd64", manifest_changes=None, extra=None, remove=()):
    library = mpv.LIBRARIES[target.split("/")[0]]
    dependency = {"windows": "dependency.dll", "darwin": "dependency.dylib", "linux": "dependency.so.1"}[target.split("/")[0]]
    libraries = {library: b"native primary", dependency: b"native dependency"}
    manifest = {"platform": target, "mpv_version": "0.42.0", "profile": "audio", "license_profile": "lgpl",
                "macos_minimum": "15.0", "libraries": {name: hashlib.sha256(data).hexdigest() for name, data in libraries.items()}}
    manifest.update(manifest_changes or {})
    files = {f"mpv/{name}": data for name, data in libraries.items()}
    files.update({
        "mpv/manifest.json": json.dumps(manifest).encode(),
        "mpv/native-lock.json": json.dumps({"mpv": {"version": "0.42.0"}, "release": {
            "repository": "tangqiu0205/mpv-audio", "tag": "libmpv-audio-0.42.0-r1"}}).encode(),
        "mpv/licenses/mpv/LICENSE.LGPL": b"license notice",
        "mpv/include/mpv/client.h": b"C SDK header",
        "mpv/examples/version.c": b"C SDK example",
        "mpv/libmpv.dll.a": b"import library",
    })
    files.update(extra or {})
    for name in remove:
        del files[name]
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as archive:
        for name, data in files.items():
            item = zipfile.ZipInfo()
            item.filename = name
            item.create_system = 3
            item.external_attr = (stat.S_IFREG | 0o755) << 16
            archive.writestr(item, data)
    return output.getvalue()


@contextmanager
def environment(data, target="windows/amd64"):
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / "scripts").mkdir()
        lock = {"schema": 1, "repository": "tangqiu0205/mpv-audio", "tag": "libmpv-audio-0.42.0-r1",
                "version": "0.42.0", "channel": "candidate", "platforms": {target: {
                    "asset": "mpv-audio-" + target.replace("/", "-") + ".zip",
                    "sha256": hashlib.sha256(data).hexdigest()}}}
        (root / "scripts/mpv.lock.json").write_text(json.dumps(lock), encoding="utf-8")
        (root / "mygo.json").write_text(json.dumps({"macos": {"minimumSystemVersion": "15.0"}}), encoding="utf-8")
        (root / "THIRD_PARTY_NOTICES.md").write_text("application notices", encoding="utf-8")
        with patch.object(mpv, "ROOT", root), patch.object(mpv.urllib.request, "urlopen", side_effect=lambda *a, **k: io.BytesIO(data)) as network:
            yield root, lock, network


class PrepareMPVTests(unittest.TestCase):
    def test_transient_http_failure_retries_then_verifies_archive(self):
        data = runtime_archive()
        with environment(data) as (root, lock, network), patch.object(mpv.time, "sleep") as sleep:
            network.side_effect = [mpv.urllib.error.HTTPError("https://github.com/bundle", 504, "Gateway Time-out", {}, None), io.BytesIO(data)]
            asset = lock["platforms"]["windows/amd64"]
            archive = mpv.download_archive(lock, asset)
            self.assertEqual(archive.read_bytes(), data)
            self.assertEqual(network.call_count, 2)
            sleep.assert_called_once_with(2)
            self.assertEqual(list((root / ".mygo/mpv").glob("download-*")), [])

    def test_download_retries_are_bounded_and_preserve_previous_bundle(self):
        with environment(runtime_archive()) as (root, _, network), patch.object(mpv.time, "sleep") as sleep:
            network.side_effect = [mpv.urllib.error.HTTPError("https://github.com/bundle", 504, "Gateway Time-out", {}, None)
                                   for _ in range(3)]
            final = root / "resources/windows-amd64/mpv"
            final.mkdir(parents=True)
            (final / "mpv-2.dll").write_bytes(b"previous")
            with self.assertRaises(mpv.urllib.error.HTTPError):
                mpv.prepare("windows/amd64")
            self.assertEqual(network.call_count, 3)
            self.assertEqual([c.args[0] for c in sleep.call_args_list], [2, 4])
            self.assertEqual((final / "mpv-2.dll").read_bytes(), b"previous")
            self.assertEqual(list((root / ".mygo/mpv").iterdir()), [])

    def test_permanent_http_and_tls_errors_are_not_retried(self):
        errors = [mpv.urllib.error.HTTPError("https://github.com/bundle", code, "permanent", {}, None)
                  for code in (401, 403, 404)]
        errors.append(mpv.urllib.error.URLError("certificate verification failed"))
        for error in errors:
            with self.subTest(error=error), environment(runtime_archive()) as (_, lock, network), patch.object(mpv.time, "sleep") as sleep:
                network.side_effect = error
                with self.assertRaises(mpv.urllib.error.URLError):
                    mpv.download_archive(lock, lock["platforms"]["windows/amd64"])
                self.assertEqual(network.call_count, 1)
                sleep.assert_not_called()

    def test_partial_download_is_replaced_on_retry(self):
        class BrokenStream(io.BytesIO):
            def read(self, size=-1):
                if self.tell():
                    raise ConnectionResetError("connection reset during response")
                return super().read(size)

        data = runtime_archive()
        with environment(data) as (_, lock, network), patch.object(mpv.time, "sleep"):
            network.side_effect = [BrokenStream(b"partial" * len(data)), io.BytesIO(data)]
            archive = mpv.download_archive(lock, lock["platforms"]["windows/amd64"])
            self.assertEqual(archive.read_bytes(), data)
            self.assertEqual(network.call_count, 2)

    def test_timeout_retries_but_checksum_failure_does_not(self):
        data = runtime_archive()
        for error in (TimeoutError("timed out"), mpv.urllib.error.URLError(TimeoutError("timed out"))):
            with self.subTest(error=error), environment(data) as (_, lock, network), patch.object(mpv.time, "sleep") as sleep:
                network.side_effect = [error, io.BytesIO(data)]
                mpv.download_archive(lock, lock["platforms"]["windows/amd64"])
                self.assertEqual(network.call_count, 2)
                sleep.assert_called_once_with(2)
        with environment(data) as (_, lock, network), patch.object(mpv.time, "sleep") as sleep:
            network.side_effect = lambda *a, **k: io.BytesIO(b"wrong content")
            with self.assertRaisesRegex(RuntimeError, "archive SHA-256"):
                mpv.download_archive(lock, lock["platforms"]["windows/amd64"])
            self.assertEqual(network.call_count, 1)
            sleep.assert_not_called()

    def test_production_lock_pins_all_six_desktop_targets(self):
        lock = mpv.read_lock()
        self.assertEqual(set(lock["platforms"]), {f"{system}/{arch}" for system in mpv.LIBRARIES for arch in ("amd64", "arm64")})
        self.assertEqual(lock["version"], "0.42.0")
        self.assertEqual(lock["channel"], "candidate")
        for target, asset in lock["platforms"].items():
            self.assertEqual(asset["asset"], "mpv-audio-" + target.replace("/", "-") + ".zip")
            self.assertRegex(asset["sha256"], r"^[0-9a-f]{64}$")

    def test_runtime_libraries_and_licenses_are_installed_but_sdk_files_are_not(self):
        with environment(runtime_archive()) as (root, lock, network):
            mpv.prepare("windows/amd64")
            dest = root / "resources/windows-amd64/mpv"
            self.assertEqual((dest / "mpv-2.dll").read_bytes(), b"native primary")
            self.assertEqual((dest / "dependency.dll").read_bytes(), b"native dependency")
            self.assertTrue((dest / "licenses/mpv/LICENSE.LGPL").is_file())
            self.assertTrue((dest / "THIRD_PARTY_NOTICES.md").is_file())
            self.assertFalse((dest / "include").exists())
            self.assertFalse((dest / "examples").exists())
            self.assertFalse((dest / "libmpv.dll.a").exists())
            origin = json.loads((dest / "bundle.json").read_text(encoding="utf-8"))
            self.assertEqual(origin["archive_sha256"], lock["platforms"]["windows/amd64"]["sha256"])
            self.assertIn("/libmpv-audio-0.42.0-r1/", network.call_args.args[0].full_url)

    def test_all_platforms_can_be_prepared_without_native_build_tools(self):
        for target in ("windows/arm64", "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"):
            with self.subTest(target=target), environment(runtime_archive(target), target) as (root, _, _):
                mpv.prepare(target)
                self.assertTrue((root / "resources" / target.replace("/", "-") / "mpv" / mpv.LIBRARIES[target.split("/")[0]]).exists())

    def test_valid_cached_archive_is_reused_and_previous_bundle_replaced(self):
        with environment(runtime_archive()) as (root, _, network):
            mpv.prepare("windows/amd64")
            (root / "resources/windows-amd64/mpv/obsolete.dll").write_bytes(b"old")
            mpv.prepare("windows/amd64")
            self.assertEqual(network.call_count, 1)
            self.assertFalse((root / "resources/windows-amd64/mpv/obsolete.dll").exists())
            self.assertEqual(list((root / "resources/windows-amd64").glob("mpv-stage-*")), [])

    def test_wrong_archive_checksum_preserves_previous_bundle(self):
        with environment(runtime_archive()) as (root, lock, _):
            lock["platforms"]["windows/amd64"]["sha256"] = "0" * 64
            (root / "scripts/mpv.lock.json").write_text(json.dumps(lock), encoding="utf-8")
            final = root / "resources/windows-amd64/mpv"
            final.mkdir(parents=True)
            (final / "mpv-2.dll").write_bytes(b"previous")
            with self.assertRaisesRegex(RuntimeError, "archive SHA-256"):
                mpv.prepare("windows/amd64")
            self.assertEqual((final / "mpv-2.dll").read_bytes(), b"previous")

    def test_corrupted_cache_fails_without_network_fallback(self):
        with environment(runtime_archive()) as (root, lock, network):
            cache = root / ".mygo/mpv"
            cache.mkdir(parents=True)
            digest = lock["platforms"]["windows/amd64"]["sha256"]
            (cache / (digest + ".zip")).write_bytes(b"corrupt")
            with self.assertRaisesRegex(RuntimeError, "cached libmpv archive"):
                mpv.prepare("windows/amd64")
            network.assert_not_called()

    def test_mismatched_platform_version_and_profile_are_rejected(self):
        for change in ({"platform": "windows/arm64"}, {"mpv_version": "0.38.0"}, {"license_profile": "gpl"}):
            with self.subTest(change=change), environment(runtime_archive(manifest_changes=change)):
                with self.assertRaisesRegex(RuntimeError, "bundle platform, version or build profile"):
                    mpv.prepare("windows/amd64")

    def test_manifest_must_match_each_library(self):
        for changes in ({"extra": {"mpv/mpv-2.dll": b"modified"}}, {"remove": ("mpv/dependency.dll",)}):
            with self.subTest(changes=changes), environment(runtime_archive(**changes)):
                with self.assertRaisesRegex(RuntimeError, "library SHA-256"):
                    mpv.prepare("windows/amd64")

    def test_unsafe_paths_and_case_collisions_are_rejected(self):
        for name in ("../outside", "/absolute", "mpv/../outside", "mpv\\outside", "C:/outside", "mpv/MANIFEST.JSON"):
            with self.subTest(name=name), environment(runtime_archive(extra={name: b"bad"})):
                with self.assertRaisesRegex(RuntimeError, "unsafe archive path|duplicate archive path"):
                    mpv.prepare("windows/amd64")

    def test_symlinks_are_rejected(self):
        stream = io.BytesIO(runtime_archive())
        with zipfile.ZipFile(stream, "a") as archive:
            item = zipfile.ZipInfo("mpv/link")
            item.external_attr = (stat.S_IFLNK | 0o777) << 16
            archive.writestr(item, "../outside")
        with environment(stream.getvalue()):
            with self.assertRaisesRegex(RuntimeError, "unsupported archive entry type"):
                mpv.prepare("windows/amd64")

    def test_licenses_and_source_lock_are_required(self):
        for changes in ({"remove": ("mpv/licenses/mpv/LICENSE.LGPL",)},
                        {"extra": {"mpv/native-lock.json": json.dumps({"mpv": {"version": "0.42.0"}, "release": {
                            "repository": "other/repository", "tag": "libmpv-audio-0.42.0-r1"}}).encode()}}):
            with self.subTest(changes=changes), environment(runtime_archive(**changes)):
                with self.assertRaises(RuntimeError):
                    mpv.prepare("windows/amd64")

    def test_failed_validation_preserves_previous_bundle(self):
        with environment(runtime_archive(manifest_changes={"platform": "wrong"})) as (root, _, _):
            final = root / "resources/windows-amd64/mpv"
            final.mkdir(parents=True)
            (final / "mpv-2.dll").write_bytes(b"previous")
            with self.assertRaises(RuntimeError):
                mpv.prepare("windows/amd64")
            self.assertEqual((final / "mpv-2.dll").read_bytes(), b"previous")
            self.assertEqual(list(final.parent.glob("mpv-stage-*")), [])

    def test_failed_install_restores_previous_bundle(self):
        with environment(runtime_archive()) as (root, _, _):
            final = root / "resources/windows-amd64/mpv"
            final.mkdir(parents=True)
            (final / "mpv-2.dll").write_bytes(b"previous")
            original = Path.rename

            def rename(source, target):
                if source.name == "mpv" and source.parent.name.startswith("mpv-stage-"):
                    raise OSError("simulated install failure")
                return original(source, target)

            with patch.object(Path, "rename", new=rename), self.assertRaisesRegex(OSError, "simulated install"):
                mpv.prepare("windows/amd64")
            self.assertEqual((final / "mpv-2.dll").read_bytes(), b"previous")

    def test_macos_requirement_cannot_silently_raise_app_minimum(self):
        target = "darwin/arm64"
        with environment(runtime_archive(target), target) as (root, _, _):
            (root / "mygo.json").write_text(json.dumps({"macos": {"minimumSystemVersion": "14.0"}}), encoding="utf-8")
            with self.assertRaisesRegex(RuntimeError, "requires macOS 15.0"):
                mpv.prepare(target)

    def test_unsupported_target_fails_before_network(self):
        with environment(runtime_archive()) as (_, _, network):
            with self.assertRaisesRegex(RuntimeError, "unsupported libmpv platform"):
                mpv.prepare("windows/386")
            network.assert_not_called()


if __name__ == "__main__":
    unittest.main()
