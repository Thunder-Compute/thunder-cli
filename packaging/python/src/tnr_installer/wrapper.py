import hashlib
import io
import json
import os
import platform
import shutil
import stat
import sys
import tarfile
import tempfile
import zipfile
from pathlib import Path
from urllib.request import urlopen


def _user_cache_dir() -> Path:
    return Path(os.path.expanduser("~/.cache/tnr/versions"))


def _detect_os_arch():
    os_name = platform.system().lower()
    arch = platform.machine().lower()
    if arch in ("x86_64", "amd64"):
        arch = "amd64"
    elif arch in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        raise RuntimeError(f"Unsupported arch: {arch}")
    return os_name, arch


def _manifest_url() -> str:
    url = os.getenv("TNR_LATEST_URL")
    if url:
        return url
    base = os.getenv("TNR_DOWNLOAD_BASE")
    if base:
        return f"{base}/tnr/releases/latest.json"
    bucket = os.getenv("TNR_S3_BUCKET")
    region = os.getenv("AWS_REGION")
    if not bucket or not region:
        raise RuntimeError("Set TNR_LATEST_URL or TNR_DOWNLOAD_BASE, or TNR_S3_BUCKET and AWS_REGION")
    return f"https://{bucket}.s3.{region}.amazonaws.com/tnr/releases/latest.json"


def _download(url: str) -> bytes:
    with urlopen(url) as r:
        return r.read()


def _sha256(b: bytes) -> str:
    h = hashlib.sha256()
    h.update(b)
    return h.hexdigest()


def _extract(archive: bytes, os_name: str, dest: Path) -> Path:
    dest.parent.mkdir(parents=True, exist_ok=True)
    if os_name == "windows":
        with zipfile.ZipFile(io.BytesIO(archive)) as z:
            z.extract("tnr.exe", dest.parent)
        p = dest.parent / "tnr.exe"
        return p
    else:
        with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as t:
            t.extract("tnr", path=dest.parent)
        p = dest.parent / "tnr"
        p.chmod(p.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        return p


def main():
    os_name, arch = _detect_os_arch()
    cache_dir = _user_cache_dir()

    manifest = json.loads(_download(_manifest_url()))
    version = os.getenv("TNR_VERSION") or manifest["version"]
    key = f"{os_name}/{arch}"
    asset_url = manifest["assets"][key]
    checksums_url = manifest["assets"]["checksums"]

    version_dir = cache_dir / version
    exe = version_dir / ("tnr.exe" if os_name == "windows" else "tnr")
    if not exe.exists():
        data = _download(asset_url)
        sums = _download(checksums_url).decode()
        if _sha256(data) not in sums:
            raise RuntimeError("Checksum mismatch for downloaded artifact")
        version_dir.mkdir(parents=True, exist_ok=True)
        bin_path = _extract(data, os_name, exe)
        shutil.move(str(bin_path), str(exe))

    # Exec the binary, passing through args
    args = [str(exe), *sys.argv[1:]]
    os.execv(args[0], args)


if __name__ == "__main__":
    main()


