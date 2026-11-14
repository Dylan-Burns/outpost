#!/usr/bin/env bash
# Check that what the website serves is what this source builds.
#   scripts/verify-release.sh 1.3.1
# Builds tag v1.3.1 in a scratch folder, downloads that release from the
# website, checks its signature against the key in install/install.sh, and
# compares each program byte for byte. Then compares the installer the
# website serves with the one in this checkout.
# Needs git, Go, Node, curl and ssh-keygen. Changes nothing in the checkout.
set -euo pipefail
VERSION="${1:?usage: scripts/verify-release.sh VERSION}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like 1.2.3" >&2; exit 1; }
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SITE="${OUTPOST_SITE:-https://projectoutpost.org}"
WORK="$(mktemp -d)"
cleanup() { git -C "$ROOT" worktree remove --force "$WORK/src" >/dev/null 2>&1 || true; rm -rf "$WORK"; }
trap cleanup EXIT
sha() { shasum -a 256 "$@" 2>/dev/null || sha256sum "$@"; }
bad=0

git -C "$ROOT" rev-parse -q --verify "refs/tags/v$VERSION" >/dev/null || { echo "no tag v$VERSION here; try: git fetch --tags" >&2; exit 1; }
echo "==> source: tag v$VERSION ($(git -C "$ROOT" rev-parse --short "v$VERSION"))"
git -C "$ROOT" worktree add -q --detach "$WORK/src" "v$VERSION"

echo "==> download: $SITE/releases/v$VERSION"
mkdir "$WORK/pub"
for f in SHA256SUMS SHA256SUMS.sig; do curl -fsSL "$SITE/releases/v$VERSION/$f" -o "$WORK/pub/$f"; done
KEY="$(sed -n 's/^RELEASE_KEY="\(.*\)"$/\1/p' "$ROOT/install/install.sh")"
printf 'outpost-release %s\n' "$KEY" > "$WORK/allowed"
ssh-keygen -Y verify -f "$WORK/allowed" -I outpost-release -n outpost-release -s "$WORK/pub/SHA256SUMS.sig" < "$WORK/pub/SHA256SUMS" >/dev/null \
  || { echo "FAIL  the checksum list is not signed by the release key" >&2; exit 1; }
echo "ok    checksum list is signed by the release key"

echo "==> dashboard"
(cd "$WORK/src/node/web" && npm ci --silent && node build.mjs >/dev/null)

while read -r want name; do
  t="${name#outpost-"$VERSION"-}"; t="${t%.tar.gz}"; os="${t%-*}"; arch="${t#*-}"
  curl -fsSL "$SITE/releases/v$VERSION/$name" -o "$WORK/pub/$name"
  [ "$(sha "$WORK/pub/$name" | cut -d' ' -f1)" = "$want" ] || { echo "FAIL  $name does not match the signed checksum"; bad=1; continue; }
  mkdir -p "$WORK/pub/$t" "$WORK/built/$t"
  tar -xzf "$WORK/pub/$name" -C "$WORK/pub/$t"
  # The same command scripts/release.sh uses.
  (cd "$WORK/src/node" && CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -buildvcs=false \
     -ldflags "-s -w -buildid= -X main.version=$VERSION" -o "$WORK/built/$t/outpost" ./cmd/outpost)
  a="$(sha "$WORK/pub/$t/outpost" | cut -d' ' -f1)"; b="$(sha "$WORK/built/$t/outpost" | cut -d' ' -f1)"
  if [ "$a" = "$b" ]; then echo "ok    $os/$arch  $a"; else echo "FAIL  $os/$arch  served $a  built $b"; bad=1; fi
done < "$WORK/pub/SHA256SUMS"

echo "==> installer"
a="$(curl -fsSL "$SITE/install.sh" | sha | cut -d' ' -f1)"; b="$(sha "$ROOT/install/install.sh" | cut -d' ' -f1)"
if [ "$a" = "$b" ]; then echo "ok    install.sh  $a"
else echo "DIFF  install.sh  served $a  this checkout $b  (expected only if this checkout is not what is deployed)"; fi

[ "$bad" = 0 ] && echo "==> release $VERSION matches its source" || { echo "==> release $VERSION does NOT match" >&2; exit 1; }
