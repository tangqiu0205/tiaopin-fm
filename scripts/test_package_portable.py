import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import package_portable as portable


class PackagePortableTests(unittest.TestCase):
    def build(self, directory, arch="amd64"):
        directory.mkdir(parents=True)
        files = {
            "TiaopinFM.exe": b"program",
            f"tiaopin-fm-setup-0.1.0-{arch}.exe": b"installer",
            ".mygo-files": b"TiaopinFM.exe\nicon.png\nmpv\n",
            "icon.png": b"icon",
            "mpv/mpv-2.dll": b"runtime",
            "mpv/licenses/notice.txt": b"license",
            "data/settings.json": b"private user data",
            "tiaopin-fm-0.1.0-windows-amd64.tar.gz": b"update",
            "update-windows-amd64.json": b"manifest",
            "old.delta": b"delta",
        }
        for name, data in files.items():
            path = directory / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)

    def test_zip_contains_only_installed_files_and_portable_marker(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            directory, output = root / "windows-amd64", root / "portable.zip"
            self.build(directory)
            portable.package_portable(directory, output, "TiaopinFM.exe", "tiaopin-fm-setup-0.1.0-amd64.exe")
            with zipfile.ZipFile(output) as archive:
                self.assertEqual(set(archive.namelist()), {
                    "TiaopinFM.exe", ".mygo-files", "icon.png", "mpv/mpv-2.dll", "mpv/licenses/notice.txt", "portable.flag"})
                self.assertEqual(archive.read("mpv/mpv-2.dll"), b"runtime")
            self.assertFalse((directory / "portable.flag").exists())

    def test_missing_build_output_and_invalid_inventory_fail(self):
        for change in ("missing installer", "missing runtime", "unsafe path", "user data", "missing executable"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                directory = root / "windows-amd64"
                self.build(directory)
                inventory = directory / ".mygo-files"
                if change == "missing installer":
                    (directory / "tiaopin-fm-setup-0.1.0-amd64.exe").unlink()
                elif change == "missing runtime":
                    inventory.write_text("TiaopinFM.exe\nmissing.dll\n", encoding="utf-8")
                elif change == "unsafe path":
                    inventory.write_text("TiaopinFM.exe\n../outside\n", encoding="utf-8")
                elif change == "user data":
                    inventory.write_text("TiaopinFM.exe\ndata\n", encoding="utf-8")
                else:
                    inventory.write_text("icon.png\n", encoding="utf-8")
                with self.assertRaises(RuntimeError):
                    portable.package_portable(directory, root / "portable.zip", "TiaopinFM.exe", "tiaopin-fm-setup-0.1.0-amd64.exe")

    def test_main_packages_only_the_requested_architecture(self):
        for arch in ("amd64", "arm64"):
            with self.subTest(arch=arch), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                config = {"name": "TiaopinFM", "slug": "tiaopin-fm", "version": "0.1.0", "out": "build"}
                (root / "mygo.json").write_text(json.dumps(config), encoding="utf-8")
                self.build(root / "build" / f"windows-{arch}", arch)
                with patch.object(portable, "ROOT", root):
                    portable.main(["--platform", f"windows/{arch}"])
                self.assertEqual([p.name for p in (root / "build").glob("*.zip")],
                                 [f"tiaopin-fm-portable-0.1.0-windows-{arch}.zip"])


if __name__ == "__main__":
    unittest.main()
