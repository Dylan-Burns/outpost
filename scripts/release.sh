#!/usr/bin/env bash
# Build, sign and stage an Outpost release into site/public/releases/.
#   scripts/release.sh 1.0.0
# Needs Go, Node and the release key (~/.config/outpost-release/release_ed25519).
set -euo pipefail
VERSION="${1:?usage: scripts/release.sh VERSION}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like 1.2.3" >&2; exit 1; }
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KEY="${OUTPOST_RELEASE_KEY:-$HOME/.config/outpost-release/release_ed25519}"
OUT="$ROOT/site/public/releases/v$VERSION"
[ -f "$KEY" ] || { echo "release key not found at $KEY" >&2; exit 1; }

echo "==> dashboard"
(cd "$ROOT/node/web" && npm ci --silent && npx tsc --noEmit && node build.mjs)
echo "==> tests"
(cd "$ROOT/node" && go vet ./... && go test -count=1 ./...)

rm -rf "$OUT" && mkdir -p "$OUT"
BUILD="$(mktemp -d)"; trap 'rm -rf "$BUILD"' EXIT
for arch in amd64 arm64; do
  echo "==> linux/$arch"
  mkdir -p "$BUILD/$arch"
  (cd "$ROOT/node" && CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath -buildvcs=false \
     -ldflags "-s -w -buildid= -X main.version=$VERSION" -o "$BUILD/$arch/outpost" ./cmd/outpost)
  # Reproducible archive: fixed owner, mode and time.
  touch -t 200001010000 "$BUILD/$arch/outpost"
  tar --format=ustar --uid 0 --gid 0 --uname "" --gname "" --numeric-owner -C "$BUILD/$arch" -cf - outpost 2>/dev/null \
    | gzip -9n > "$OUT/outpost-$VERSION-linux-$arch.tar.gz" || {
      # BSD tar spelling
      (cd "$BUILD/$arch" && touch -t 200001010000 outpost && tar --uid 0 --gid 0 --uname root --gname root -cf - outpost) | gzip -9n > "$OUT/outpost-$VERSION-linux-$arch.tar.gz"; }
done
(cd "$OUT" && shasum -a 256 outpost-*.tar.gz > SHA256SUMS)
ssh-keygen -Y sign -q -f "$KEY" -n outpost-release "$OUT/SHA256SUMS"
printf 'outpost-release %s\n' "$(cat "$KEY.pub")" > "$BUILD/allowed"
ssh-keygen -Y verify -f "$BUILD/allowed" -I outpost-release -n outpost-release -s "$OUT/SHA256SUMS.sig" < "$OUT/SHA256SUMS" >/dev/null
cat > "$ROOT/site/public/releases/latest.json" <<JSON
{"version":"$VERSION","date":"$(date -u +%Y-%m-%d)","files":["outpost-$VERSION-linux-amd64.tar.gz","outpost-$VERSION-linux-arm64.tar.gz"]}
JSON
rm -f "$ROOT/site/public/releases/latest.json.sig"
ssh-keygen -Y sign -q -f "$KEY" -n outpost-release-index "$ROOT/site/public/releases/latest.json"
cp "$ROOT/install/install.sh" "$ROOT/site/public/install.sh"
echo "==> staged $OUT"
cat "$OUT/SHA256SUMS"
