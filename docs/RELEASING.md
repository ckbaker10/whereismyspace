# Release procedure

Build from a clean checkout. The initial v1.0.0 assets use Go 1.27.1,
Python 3 and `CGO_ENABLED=0`; the module requires Go 1.26 or newer.

```sh
make check
make package-release VERSION=v1.0.0
cd dist/v1.0.0
sha256sum -c SHA256SUMS
```

`scripts/release.py` builds the five supported targets with `-trimpath`,
`-buildvcs=false` and a version stamp. Linux amd64 uses the baseline amd64 v1
instruction set; ARM64 uses v8.0. Archives use fixed timestamps, permissions
and ordering. The script runs an isolated smoke test on the host-compatible
binary, preserves existing output directories and records the source commit,
source file hashes and compiler in `BUILDINFO.json`. The manifest is included
in every archive and uploaded separately; `SHA256SUMS` covers all uploaded
archives and the manifest. Checksums detect corruption but are not signatures.

To reproduce identical assets, use the same clean commit, Go toolchain, Python
and zlib versions on the same build host architecture. Move the first `dist`
directory aside, repeat the build, and compare archive SHA-256 values. Keep
raw build/test logs under ignored `work/`; do not upload those logs.

Before a future release, update the release notes and `DOCUMENTS` in
`scripts/release.py` to refer to the new version. Run the existing checks,
verify the archives and inspect the native binary's `--version`. Cross-build
success does not replace runtime tests on the other target systems.

Prepare an unpublished release with actual assets after pushing the source
commit (run from the repository root):

```sh
gh release create v1.0.0 dist/v1.0.0/* \
  --repo ckbaker10/whereismyspace \
  --target "$(git rev-parse HEAD)" --draft \
  --title 'whereismyspace v1.0.0' --notes-file docs/releases/v1.0.0.md
```

Review the draft's seven assets (five archives, `BUILDINFO.json`, `SHA256SUMS`)
and source target on GitHub. Publishing the draft makes the release and assets
public and creates its release tag. Publication is a separate final action.
