# whereismyspace

A single, dependency-light Go binary that answers *"what is eating the disk on
this box?"* — built for sysadmins.

Unlike a plain recursive `du`, `whereismyspace`:

- **Stays on one filesystem by default.** Other partitions mounted inside
  subfolders (`/var/lib/docker`, `/var/lib/opensearch`, network shares, Docker
  `overlay2` mounts, …) are *not* descended into, and are listed separately so
  you can see them. Use `--cross-mounts` to include them.
- **Skips pseudo/system directories by default** (`/proc`, `/sys`, `/dev`,
  `/run`, and OS-specific equivalents) so the output is signal, not noise.
  Disable with `--no-smart-exclude`.
- **Reports real on-disk usage** (allocated blocks, matching `du`), dedupes
  hardlinks, and does not follow symlinks.
- **Caches each scan as an index** so the common drill-down workflow — scan a
  top directory, then investigate a subdirectory — is near-instant on the second
  run instead of re-walking millions of files.

It is **stdlib-only** (no external dependencies), so it builds anywhere the Go
toolchain runs and is trivial to audit and drop onto a server.

## Install a release binary

Download the archive for your OS and CPU from
[GitHub Releases](https://github.com/ckbaker10/whereismyspace/releases).
The first stable version is
[**v1.0.0**](https://github.com/ckbaker10/whereismyspace/releases/tag/v1.0.0).

| OS | CPU | Archive |
|----|-----|---------|
| Linux | x86-64 | `whereismyspace_v1.0.0_linux_amd64.tar.gz` |
| Linux | ARM64 | `whereismyspace_v1.0.0_linux_arm64.tar.gz` |
| macOS | Intel | `whereismyspace_v1.0.0_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `whereismyspace_v1.0.0_darwin_arm64.tar.gz` |
| Windows | x86-64 | `whereismyspace_v1.0.0_windows_amd64.zip` |

Download `SHA256SUMS` alongside the archive and verify it before installing.
For example, on Linux x86-64:

```sh
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf whereismyspace_v1.0.0_linux_amd64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 whereismyspace "$HOME/.local/bin/whereismyspace"
"$HOME/.local/bin/whereismyspace" --version
```

Ensure `$HOME/.local/bin` is on your `PATH`. On macOS use
`shasum -a 256 <archive>` and compare the digest with `SHA256SUMS`, then extract
and install as above using the appropriate archive. On Windows, compare
`Get-FileHash .\whereismyspace_v1.0.0_windows_amd64.zip -Algorithm SHA256`
with `SHA256SUMS`, extract the ZIP and run `.\whereismyspace.exe --version`.
The macOS and Windows binaries are not signed or notarized.

The scanner does not modify or delete scanned files. It writes its index to
the cache directory unless `--no-cache` is set. Run with your normal account;
use elevated permissions only when you need to inspect protected directories.

## Build from source

Requires Go **1.26 or newer** and, for the Make targets, GNU Make.

```sh
make build            # builds ./whereismyspace (or: go build -o whereismyspace .)
```

Cross-compile release binaries for all supported platforms into `bin/`:

```sh
make release          # linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
```

Or target a single platform directly:

```sh
GOOS=linux GOARCH=amd64 go build -o whereismyspace .
```

### Make targets

Run `make help` for the full list. The common ones:

| Target | Description |
|--------|-------------|
| `make build` | build `./whereismyspace` (stripped, with version stamp) |
| `make install` | `go install` into `$GOBIN` / `$GOPATH/bin` |
| `make run ARGS="--tree /var"` | build and run, passing flags via `ARGS` |
| `make test` | run the test suite |
| `make race` | run tests under the race detector |
| `make cover` | run tests and open an HTML coverage report |
| `make vet` / `make fmt` / `make fmt-check` | vet, format, or check formatting |
| `make check` | format check + vet + race tests (CI gate) |
| `make release` | cross-compile all platforms into `bin/` |
| `make clean` | remove build artifacts |

The build stamps the binary version from `git describe` (falling back to `dev`),
surfaced via `whereismyspace --version`.

## Usage

```
whereismyspace [flags] [path]
```

If no `path` is given, the **current working directory** is scanned.

```sh
whereismyspace                 # top-20 largest dirs under $PWD
whereismyspace /               # audit the root filesystem only
whereismyspace --tree /var     # depth-limited tree of /var
whereismyspace --files /home   # the largest individual files
whereismyspace --json / | jq   # machine-readable output
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--top N` | 20 | entries to show in list mode |
| `--tree` | off | render a depth-limited tree instead of a list |
| `--depth D` | 2 | maximum depth in tree mode |
| `--json` | off | emit machine-readable JSON |
| `--cross-mounts` | off | descend into other mounted partitions |
| `--no-smart-exclude` | off | disable built-in pseudo-fs/system exclusions |
| `--skip-caches` | off | also skip cache/library dirs (`node_modules`, `.cache`, …) |
| `--exclude PATTERN` | — | extra path / basename / glob to skip (repeatable) |
| `--min-size SIZE` | — | hide entries below a threshold (e.g. `100M`, `1.5G`) |
| `--files` | off | list the largest files instead of directories |
| `--apparent-size` | off | report logical file size instead of allocated blocks |
| `--follow-symlinks` | off | follow symlinks during the walk |
| `--count-links` | off | count hardlinked inodes multiple times |
| `--workers N` | NumCPU | scan concurrency |
| `--no-progress` | off | disable the live progress line on stderr |
| `--refresh` | off | force a live scan, ignoring any cached index |
| `--no-cache` | off | do not read or write the on-disk index |
| `--max-age D` | `1h` | reuse a cached index only if younger than this (`0` = never expire) |
| `--cache-dir P` | OS cache dir | directory for the index |
| `--version` | | print version and exit |

### Example

```
$ whereismyspace /
Scanned: /
Total:   75.0 GB (1284431 files)
Elapsed: 41.3s

Mounted partitions excluded (other filesystems):
  - /var/lib/opensearch
  - /var/lib/docker
  - /mnt/docushare

Largest directories:
      31.2 GB  /var/lib/gitea
       8.4 GB  /home/rundeck/dumps
       ...
```

The **Total** is the allocated space of the accessible files and directories
included in the scan. It can differ from `df`: filesystem metadata, snapshots,
deleted-but-open files, exclusions and permission errors affect the comparison.
Review the reported error count and exclusions when investigating a difference.

## Caching / index

A full scan of a large root can take minutes. Because the usual workflow is to
scan a top directory and *then* drill into a subdirectory, every scan is saved
as a compressed index (gzip + gob) under the OS cache directory
(`~/.cache/whereismyspace/` on Linux; override with `--cache-dir` or the
`WHEREISMYSPACE_CACHE` environment variable).

On a later run, if a **fresh** index exists whose root covers the requested
path, the subtree is extracted from it and rendered instantly — no filesystem
walk:

```
$ whereismyspace /            # live scan of / (~3 min), writes the index
$ whereismyspace /var         # served from the / index in ~0.1s
Scanned: /var
Total:   190.6 GB (…)
Source:  cached index from 2m14s ago (run --refresh for a live scan)
```

- An index is reused only while younger than `--max-age` (default `1h`; `0`
  disables expiry) and only for **matching options** — flags that change the
  numbers (`--cross-mounts`, `--apparent-size`, `--no-smart-exclude`,
  `--skip-caches`, `--exclude`, …) are fingerprinted, so a different query never
  reuses an incompatible index.
- `--refresh` forces a live scan (and updates the index); `--no-cache` disables
  reading and writing entirely.
- `--files` always scans live, since per-file data is not kept in the index.

Because a cache can be out of date, cached results always print the index age
and how to refresh.

## How it works

The scanner walks the tree with a bounded worker pool (`--workers`, default one
per CPU), reading each directory in parallel while capping total concurrency.
Every directory's device id is compared to the root's; a mismatch marks a mount
boundary that is skipped (unless `--cross-mounts`). Sizes use `st_blocks` to
reflect actual allocation, directory inodes are counted like `du`, and hardlinked
inodes are counted once. Subtree totals are aggregated in a single-threaded pass
after the concurrent walk completes.

## Platform notes

- **Linux / macOS / BSD:** full support (device-id mount detection, inode
  hardlink dedup).
- **Windows:** builds; mount-boundary detection and hardlink deduplication are
  unavailable. The default size estimate rounds logical sizes to 512-byte
  blocks; `--apparent-size` reports logical sizes directly. It does not measure
  actual Windows allocation, compression or sparse-file usage.
- Release builds cover the five platforms above. Linux x86-64 is exercised
  locally; the other binaries are cross-compiled and have not been run on their
  target OS or CPU during release preparation. BSD can be built from source.

## Testing

```sh
make test              # unit + integration tests (or: go test ./...)
make race              # race detector — the walker is concurrent
make check             # format check + vet + race, the full CI gate
```

## License and releases

Licensed under the [MIT License](LICENSE). The included Go runtime and standard
library retain their [third-party license](THIRD_PARTY_NOTICES.md).

See the [v1.0.0 release notes](docs/releases/v1.0.0.md) and
[release procedure](docs/RELEASING.md) for build provenance, checks and packaging.

## Author

**Lukas Bockel** — <https://github.com/ckbaker10>
