# Security

## Reporting a problem

Report it privately: on this repository open **Security → Report a vulnerability**. Please do not open a public
issue for something that could put people's machines at risk. You will get an answer within a few days, and
credit in the release notes if you want it.

Worth reporting: anything that lets someone on the network claim, sign in to, or change an Outpost they do not
own; a way to make a node or the installer accept a release, catalog or archive that is not the signed or
checksummed one; a way out of the service account; a script in an archive reaching the dashboard.

## What you can check yourself

- **The signing key.** Releases and the catalog are signed with one Ed25519 key, in OpenSSH's signature format.
  Its fingerprint is `SHA256:FPLD+xaDRLa1ieZmEeYFzxkjCl2T2YliTvXER6TK84I`. The public half is in
  `install/install.sh` and `node/internal/release/release.go`. The private half has never been in this
  repository or on a build server; releases are built and signed on the maintainer's machine.
- **That a download matches this source.** Releases are reproducible. `scripts/verify-release.sh 1.3.1` builds
  the tag, downloads what projectoutpost.org serves, checks the signature and compares the programs byte for
  byte. It also compares the installer the website serves with `install/install.sh`.
- **What the node sends.** No accounts and no telemetry. A node contacts projectoutpost.org for the catalog and
  releases, and the publishers (Kiwix, Protomaps, Ollama) for what its owner chooses to download.

The design is described under "Security" in the README and at https://projectoutpost.org/docs#security.

## Supported versions

Only the newest release gets fixes. A node offers the update in its dashboard.
