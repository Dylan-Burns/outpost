# Outpost

Outpost turns a Mac or a Linux computer you own into an offline knowledge server: Wikipedia, medical references,
repair guides, courses and books (ZIM archives served by kiwix-serve), street maps of the regions you
choose (cut from the Protomaps OpenStreetMap build), and an optional local AI assistant (Ollama) that
answers from your library and cites it. You bring the hardware; Outpost provides the installer, a
setup wizard and a curated, signed content catalog.

Website: https://projectoutpost.org

```
curl -fsSL https://projectoutpost.org/install.sh | sudo bash            # add -s -- --with-ai for the assistant
```

The same line works on macOS 13 or later (Intel and Apple Silicon) and on 64-bit Linux (amd64 and arm64).
There is no Windows installer. Any device with a browser can use a running node.

## Layout

| Path | What it is |
| --- | --- |
| `node/` | The node: one Go program (`outpost serve`) with the dashboard API, job queue, downloader, Kiwix/maps/Ollama integration |
| `node/web/` | The dashboard (Preact + TypeScript, bundled with esbuild into `node/internal/server/ui/dist` and embedded) |
| `catalog/` | `source.json` (the curated selection) and `build.py`, which resolves it into the signed catalog |
| `install/install.sh` | The installer for macOS and Linux (also published at `/install.sh`) |
| `site/` | The website (Next.js), which also hosts the catalog, region outlines, map assets and releases |
| `scripts/release.sh` | Builds, signs and stages a release into `site/public/releases/` |
| `tests/e2e/` | `wizard.e2e.mjs`: a fresh node in a browser (claim, wizard, downloads, offline map). `smoke.mjs`: the same over the API. `install-check.sh`: the real installer, start to finish |
| `.github/workflows/install-e2e.yml` | Runs `install-check.sh` on Linux and Mac runners; started by hand before a release goes live |

## How it fits together

- **Catalog.** `catalog/build.py` reads the Kiwix OPDS library, pins each curated archive to its newest
  file with exact size and the SHA-256 Kiwix publishes, cuts region outlines from Natural Earth and measures
  each with a `pmtiles extract --dry-run`, packs the map fonts/sprites, reads model sizes from the Ollama
  registry, then signs the result (`ssh-keygen -Y sign`, namespace `outpost-catalog`). The node embeds a copy
  and refreshes it from the website; it never accepts an older sequence number.
- **Node.** Two ports: the dashboard on 8080 and the library (a proxy to a loopback kiwix-serve) on 8081,
  a separate browser origin so archive scripts cannot reach the admin UI. Content jobs survive restarts,
  resume partial downloads with HTTP ranges, verify SHA-256 before use, and retry transient failures.
- **Security.** One admin, claimed with a one-time setup code printed on the machine; argon2id password;
  HttpOnly SameSite=Strict sessions stored as hashes; CSRF token plus same-origin and JSON checks on every
  write; per-client and account-wide sign-in limits; strict CSP. On Linux the systemd unit runs as an
  unprivileged user with a read-only system and write access only to the data directory. On macOS a launchd
  job runs as a hidden `_outpost` account that owns only the data directory and its log. Program files and
  the `outpost` command (`/opt/outpost/cli/outpost`) are root's on both; a link in `/usr/local/bin` is made
  only when that folder is root-only too.
- **Releases.** `scripts/release.sh X.Y.Z` builds static binaries for Linux and macOS (amd64, arm64), writes
  `SHA256SUMS`, signs it (namespace `outpost-release`), and updates `releases/latest.json`. The installer
  verifies the signature with `ssh-keygen -Y verify` and pins kiwix-tools and pmtiles by checksum for each
  system and processor. Before deploying the site, push the staged release and run the `install-e2e`
  workflow (`gh workflow run install-e2e.yml`): it installs from the checkout on real Linux and Mac machines.
  The commit that stages a release is tagged `vX.Y.Z`, which is what `scripts/verify-release.sh` builds.

## Check it yourself

The source here is what the website hands out, and you do not have to take that on trust. Releases are
reproducible: the same commit gives the same program, byte for byte.

```
git clone https://github.com/Dylan-Burns/outpost && cd outpost
scripts/verify-release.sh 1.3.1
```

That builds tag `v1.3.1`, downloads the release projectoutpost.org serves, checks its signature against the key
in `install/install.sh` (fingerprint `SHA256:FPLD+xaDRLa1ieZmEeYFzxkjCl2T2YliTvXER6TK84I`), compares all four
programs, and compares the installer the website serves with `install/install.sh`. The `verify-release`
workflow runs the same check here every week. To read the installer before running it:
`curl -fsSL https://projectoutpost.org/install.sh | less`. Reporting a vulnerability: see `SECURITY.md`.

Checked this way: 1.0.0, 1.0.3, 1.1.0, 1.2.0, 1.3.0 and 1.3.1 match. The two releases in between, 1.0.1 and
1.0.2, both from the first day and both superseded, do not come out the same from any commit here, so
they carry no tag.

## Development

```
cd node/web && npm ci && node build.mjs          # dashboard
cd node && go test ./... && go build ./cmd/outpost
./outpost serve -data /tmp/outpost -bin /path/to/kiwix-and-pmtiles -listen 127.0.0.1:8080 -library-listen 127.0.0.1:8081
cd tests/e2e && npm ci && npx playwright install chromium && node wizard.e2e.mjs <outpost> <bin-dir> <work-dir> <shots-dir>
node tests/e2e/smoke.mjs http://127.0.0.1:8080 http://127.0.0.1:8081 <setup-code>   # a running, unclaimed node
cd site && npm ci && npx next build
```

Refreshing the catalog: `python3 catalog/build.py --pmtiles /path/to/pmtiles` (add `--skip-maps` to reuse
region sizes), then rebuild the node and deploy the site. The release key lives outside the repository at
`~/.config/outpost-release/release_ed25519`; its public half is in `node/internal/release/release.go` and
`install/install.sh`.

## Licence

Outpost is licensed under the Apache License 2.0 (`LICENSE`); third-party parts are listed in `NOTICE`.

## Content licences

Archives keep their publishers' licences (listed per item in the catalog; several are non-commercial).
Map data © OpenStreetMap contributors (ODbL); tiles by Protomaps. kiwix-tools is GPL-3.0, pmtiles BSD-3-Clause,
Ollama MIT. Outpost downloads these from their publishers onto the owner's machine; it does not redistribute them,
except the map fonts (SIL OFL) and sprites, which the website hosts.
