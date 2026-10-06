#!/usr/bin/env python3
"""Build deterministic release archives from a clean Git checkout."""

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import zipfile


ROOT = Path(__file__).resolve().parents[1]
PLATFORMS = (("linux", "amd64"), ("linux", "arm64"),
             ("darwin", "amd64"), ("darwin", "arm64"), ("windows", "amd64"))
DOCUMENTS = ("README.md", "LICENSE", "THIRD_PARTY_NOTICES.md",
             "third_party/GO-LICENSE", "docs/releases/v1.0.0.md")


def run(*args, env=None):
    return subprocess.check_output(args, cwd=ROOT, env=env, text=True).strip()


def archive(path, members):
    if path.suffix == ".zip":
        with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as out:
            for name, data, mode in members:
                info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
                info.create_system = 3
                info.external_attr = (0o100000 | mode) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                out.writestr(info, data)
    else:
        with path.open("wb") as raw:
            with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz:
                with tarfile.open(fileobj=gz, mode="w", format=tarfile.USTAR_FORMAT) as out:
                    for name, data, mode in members:
                        info = tarfile.TarInfo(name)
                        info.size = len(data)
                        info.mode = mode
                        info.mtime = 0
                        out.addfile(info, io.BytesIO(data))


def smoke(binary, version):
    """Check the actual native release binary with an isolated fixture."""
    assert run(str(binary), "--version").splitlines()[0] == f"whereismyspace {version}"
    with tempfile.TemporaryDirectory(prefix="whereismyspace-smoke-") as tmp:
        base = Path(tmp)
        tree = base / "tree"
        tree.mkdir()
        (tree / "child").mkdir()
        (tree / "child" / "payload").write_bytes(b"x" * 12345)
        common = (str(binary), "--json", "--no-progress", "--apparent-size",
                  "--cache-dir", str(base / "cache"))
        fresh = json.loads(run(*common, "--refresh", str(tree)))
        cached = json.loads(run(*common, str(tree / "child")))
        live = json.loads(run(*common, "--no-cache", str(tree / "child")))
        files = json.loads(run(*common, "--files", str(tree)))
        assert fresh["total_files"] == 1 and fresh["errors"] == 0
        assert fresh["cached"] is False and cached["cached"] is True
        assert live["cached"] is False and live["total_bytes"] == cached["total_bytes"]
        assert live["total_files"] == 1 and live["errors"] == 0
        assert files["cached"] is False and len(files["top_files"]) == 1
    print("Native release smoke test passed (version, JSON, cache, subtree, files).")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version", help="release version, e.g. v1.0.0")
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+", args.version):
        parser.error("version must be vMAJOR.MINOR.PATCH")
    if run("git", "status", "--porcelain"):
        parser.error("commit changes first; release builds require a clean checkout")
    dist = ROOT / "dist" / args.version
    if dist.exists():
        parser.error(f"output already exists: {dist}; preserve it or move it before rebuilding")
    license_path = Path(run("go", "env", "GOROOT")) / "LICENSE"
    if license_path.read_bytes() != (ROOT / "third_party/GO-LICENSE").read_bytes():
        parser.error("the toolchain license differs from third_party/GO-LICENSE")
    source_files = sorted(run("git", "ls-files").splitlines())
    provenance = {
        "version": args.version,
        "source_commit": run("git", "rev-parse", "HEAD"),
        "go_version": run("go", "version"),
        "CGO_ENABLED": "0",
        "build_flags": ["-trimpath", "-buildvcs=false", "-ldflags",
                        f"-s -w -X main.version={args.version}"],
        "source_sha256": {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest()
                          for name in source_files},
        "platforms": [f"{goos}/{goarch}" for goos, goarch in PLATFORMS],
    }
    buildinfo = (json.dumps(provenance, indent=2, sort_keys=True) + "\n").encode()
    docs = [(name, (ROOT / name).read_bytes(), 0o644) for name in DOCUMENTS]
    dist.mkdir(parents=True)
    for goos, goarch in PLATFORMS:
        suffix = ".exe" if goos == "windows" else ""
        name = "whereismyspace" + suffix
        env = dict(os.environ, CGO_ENABLED="0", GOOS=goos, GOARCH=goarch,
                   GOAMD64="v1", GOARM64="v8.0", GOFLAGS="", GOTOOLCHAIN="local")
        with tempfile.TemporaryDirectory(prefix="whereismyspace-build-") as tmp:
            binary = Path(tmp) / name
            command = ("go", "build", "-trimpath", "-buildvcs=false", "-ldflags",
                       f"-s -w -X main.version={args.version}", "-o", str(binary), ".")
            run(*command, env=env)
            if (goos, goarch) == (run("go", "env", "GOHOSTOS"), run("go", "env", "GOHOSTARCH")):
                smoke(binary, args.version)
            extension = ".zip" if goos == "windows" else ".tar.gz"
            target = dist / f"whereismyspace_{args.version}_{goos}_{goarch}{extension}"
            archive(target, [(name, binary.read_bytes(), 0o755), *docs,
                             ("BUILDINFO.json", buildinfo, 0o644)])
            print(f"Built {target.name}")
    (dist / "BUILDINFO.json").write_bytes(buildinfo)
    checksums = "".join(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n"
                        for path in sorted(dist.iterdir()))
    (dist / "SHA256SUMS").write_text(checksums)
    print(f"Release assets: {dist}")


if __name__ == "__main__":
    main()
