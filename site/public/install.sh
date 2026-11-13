#!/usr/bin/env bash
# Outpost installer: turns a Linux computer you own into an offline knowledge
# server. Safe to run again: it upgrades in place and keeps your content.
#
#   curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash
#   curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash -s -- --with-ai
#
# Options:
#   --with-ai            also install Ollama for the AI assistant
#   --data-dir DIR       where content is stored (default /var/lib/outpost)
#   --port N             dashboard port (default 8080)
#   --library-port N     library port (default 8081)
#   --version V          install a specific release instead of the latest
#   --no-firewall        do not open the ports in ufw/firewalld
#   --uninstall          remove Outpost (keeps content unless --purge)
#   --purge              with --uninstall, also delete the data directory
#   -y, --yes            do not ask questions
set -Eeuo pipefail

SITE="${OUTPOST_SITE:-https://outpost-offline.vercel.app}"
RELEASE_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBrMpCCBsGNt2QYPvv84kSXqzkIMJIvTJAgadKprEcLV outpost-release"
KIWIX_VERSION="3.8.2"
PMTILES_VERSION="1.31.2"
PREFIX="/opt/outpost"
ETC="/etc/outpost"
SERVICE_USER="outpost"

WITH_AI=0; DATA_DIR=""; PORT=""; LIB_PORT=""; VERSION=""; FIREWALL=1; UNINSTALL=0; PURGE=0; YES=0

c_bold=$'\e[1m'; c_dim=$'\e[2m'; c_green=$'\e[32m'; c_red=$'\e[31m'; c_off=$'\e[0m'
[ -t 1 ] || { c_bold=""; c_dim=""; c_green=""; c_red=""; c_off=""; }
say()  { printf '%s==>%s %s\n' "$c_green" "$c_off" "$*"; }
note() { printf '    %s%s%s\n' "$c_dim" "$*" "$c_off"; }
die()  { printf '%sError:%s %s\n' "$c_red" "$c_off" "$*" >&2; exit 1; }
trap 'die "the installer stopped unexpectedly at line $LINENO. Nothing was left half-installed that a re-run will not fix."' ERR

while [ $# -gt 0 ]; do
  case "$1" in
    --with-ai) WITH_AI=1 ;;
    --data-dir) DATA_DIR="${2:?--data-dir needs a value}"; shift ;;
    --port) PORT="${2:?--port needs a value}"; shift ;;
    --library-port) LIB_PORT="${2:?--library-port needs a value}"; shift ;;
    --version) VERSION="${2:?--version needs a value}"; shift ;;
    --no-firewall) FIREWALL=0 ;;
    --uninstall) UNINSTALL=1 ;;
    --purge) PURGE=1 ;;
    -y|--yes) YES=1 ;;
    -h|--help) sed -n '2,20p' "$0" 2>/dev/null || true; exit 0 ;;
    *) die "unknown option: $1 (see --help)" ;;
  esac
  shift
done

[ "$(id -u)" -eq 0 ] || die "run the installer as root, for example: curl -fsSL $SITE/install.sh | sudo bash"
[ "$(uname -s)" = "Linux" ] || die "Outpost installs on Linux (Ubuntu, Debian, Fedora and similar). See $SITE/docs for other systems."
command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ] || die "this system does not run systemd, which Outpost uses to start at boot."

ask() { # ask "question" default(y/n)
  local q="$1" def="$2" ans
  if [ "$YES" -eq 1 ] || ! [ -r /dev/tty ]; then [ "$def" = y ]; return; fi
  read -r -p "$q [$( [ "$def" = y ] && echo Y/n || echo y/N )] " ans </dev/tty || ans=""
  ans="${ans:-$def}"; [[ "$ans" =~ ^[Yy] ]]
}

valid_port() { [[ "$1" =~ ^[0-9]+$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]; }
[ -z "$PORT" ] || valid_port "$PORT" || die "--port must be a number from 1 to 65535"
[ -z "$LIB_PORT" ] || valid_port "$LIB_PORT" || die "--library-port must be a number from 1 to 65535"
[ -z "$VERSION" ] || [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || die "--version looks wrong: $VERSION"
if [ -n "$DATA_DIR" ]; then
  case "$DATA_DIR" in /*) ;; *) die "--data-dir must be an absolute path" ;; esac
  case "$DATA_DIR" in *[!A-Za-z0-9/._-]*) die "--data-dir may only contain letters, digits, / . _ -" ;; esac
fi

# ---------------------------------------------------------------- uninstall
if [ "$UNINSTALL" -eq 1 ]; then
  say "Removing Outpost"
  existing_data="/var/lib/outpost"
  if [ -f "$ETC/outpost.env" ]; then existing_data="$(sed -n 's/^OUTPOST_DATA_DIR=//p' "$ETC/outpost.env" | tail -1)"; fi
  systemctl disable --now outpost.service >/dev/null 2>&1 || true
  rm -f /etc/systemd/system/outpost.service /usr/local/bin/outpost
  systemctl daemon-reload
  rm -rf "$PREFIX"
  if [ "$PURGE" -eq 1 ]; then
    if [ -n "$existing_data" ] && [ "$existing_data" != "/" ] && { [ -f "$existing_data/state.json" ] || [ -d "$existing_data/content" ]; }; then
      rm -rf "$existing_data"; note "deleted $existing_data"
    fi
    rm -rf "$ETC"
    id "$SERVICE_USER" >/dev/null 2>&1 && userdel "$SERVICE_USER" 2>/dev/null || true
  else
    note "your content is still in $existing_data (use --uninstall --purge to delete it)"
  fi
  note "Ollama, if installed, was left in place."
  say "Done."
  exit 0
fi

# ---------------------------------------------------------------- platform
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64; KIWIX_ARCH=x86_64; PMT_ARCH=x86_64
    KIWIX_SHA="b0ae98dd344aa0469a15ab42feff6d5aafb79541a82fb4e2647c74b073123815"
    PMT_SHA="3ed7dbf4ec2e6dfe5e25b6f70d1ffc932729f93c86db353bf514dd71010a312f" ;;
  aarch64|arm64) ARCH=arm64; KIWIX_ARCH=aarch64; PMT_ARCH=arm64
    KIWIX_SHA="a9898fc5363437327350db669bbc03dcc7ecfd498dae727d7184bebe77019514"
    PMT_SHA="f8bd47e7ea866863489cad588fbaf2f31f42e5821f7a03f009b3769f05801cb1" ;;
  *) die "this processor ($(uname -m)) is not supported yet. Outpost runs on 64-bit Intel/AMD and ARM (e.g. Raspberry Pi 4/5 with a 64-bit OS)." ;;
esac

printf '\n%sOutpost installer%s\n\n' "$c_bold" "$c_off"

pkg_install() {
  if command -v apt-get >/dev/null 2>&1; then DEBIAN_FRONTEND=noninteractive apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$@" >/dev/null
  elif command -v dnf >/dev/null 2>&1; then dnf install -y -q "$@" >/dev/null
  elif command -v yum >/dev/null 2>&1; then yum install -y -q "$@" >/dev/null
  elif command -v zypper >/dev/null 2>&1; then zypper -q install -y "$@" >/dev/null
  elif command -v pacman >/dev/null 2>&1; then pacman -Sy --noconfirm --quiet "$@" >/dev/null
  else die "please install: $*"; fi
}
need=()
command -v curl >/dev/null 2>&1 || need+=(curl)
command -v tar >/dev/null 2>&1 || need+=(tar)
command -v gzip >/dev/null 2>&1 || need+=(gzip)
command -v sha256sum >/dev/null 2>&1 || need+=(coreutils)
if ! command -v ssh-keygen >/dev/null 2>&1; then
  if command -v pacman >/dev/null 2>&1; then need+=(openssh); elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then need+=(openssh); else need+=(openssh-client); fi
fi
if [ ${#need[@]} -gt 0 ]; then say "Installing ${need[*]}"; pkg_install "${need[@]}"; fi
ssh-keygen -Y verify 2>&1 | grep -qi 'usage\|missing\|option' || die "this system's OpenSSH is too old to check signatures (need 8.1 or later)."

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fetch() { curl -fsSL --proto '=https' --tlsv1.2 --retry 4 --retry-delay 2 --connect-timeout 20 -o "$2" "$1"; }

# ---------------------------------------------------------------- release
if [ -z "$VERSION" ]; then
  fetch "$SITE/releases/latest.json" "$WORK/latest.json" || die "cannot reach $SITE. Is this machine online?"
  VERSION="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$WORK/latest.json" | head -1)"
  [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+ ]] || die "the release list looks wrong"
fi
REL="$SITE/releases/v$VERSION"
TARBALL="outpost-$VERSION-linux-$ARCH.tar.gz"
say "Downloading Outpost $VERSION ($ARCH)"
fetch "$REL/$TARBALL" "$WORK/$TARBALL" || die "cannot download $REL/$TARBALL"
fetch "$REL/SHA256SUMS" "$WORK/SHA256SUMS"
fetch "$REL/SHA256SUMS.sig" "$WORK/SHA256SUMS.sig"
printf 'outpost-release %s\n' "$RELEASE_KEY" > "$WORK/allowed_signers"
ssh-keygen -Y verify -f "$WORK/allowed_signers" -I outpost-release -n outpost-release -s "$WORK/SHA256SUMS.sig" < "$WORK/SHA256SUMS" >/dev/null 2>&1 \
  || die "the release signature does not verify. Do not use this download; report it at $SITE."
note "release signature verified"
( cd "$WORK" && grep -E "  $TARBALL\$" SHA256SUMS | sha256sum -c --status - ) || die "the download does not match its signed checksum"
note "checksum verified"

say "Downloading kiwix-tools $KIWIX_VERSION and pmtiles $PMTILES_VERSION"
KT="kiwix-tools_linux-$KIWIX_ARCH-$KIWIX_VERSION.tar.gz"
PT="go-pmtiles_${PMTILES_VERSION}_Linux_$PMT_ARCH.tar.gz"
fetch "https://download.kiwix.org/release/kiwix-tools/$KT" "$WORK/$KT"
echo "$KIWIX_SHA  $WORK/$KT" | sha256sum -c --status - || die "kiwix-tools does not match its pinned checksum"
fetch "https://github.com/protomaps/go-pmtiles/releases/download/v$PMTILES_VERSION/$PT" "$WORK/$PT"
echo "$PMT_SHA  $WORK/$PT" | sha256sum -c --status - || die "pmtiles does not match its pinned checksum"
note "checksums verified"

mkdir -p "$WORK/x/outpost" "$WORK/x/kiwix" "$WORK/x/pmtiles"
tar -xzf "$WORK/$TARBALL" -C "$WORK/x/outpost"
tar -xzf "$WORK/$KT" -C "$WORK/x/kiwix" --strip-components=1
tar -xzf "$WORK/$PT" -C "$WORK/x/pmtiles"
for f in "$WORK/x/outpost/outpost" "$WORK/x/kiwix/kiwix-serve" "$WORK/x/kiwix/kiwix-manage" "$WORK/x/pmtiles/pmtiles"; do
  [ -f "$f" ] || die "a downloaded archive is missing $(basename "$f")"
done

# ---------------------------------------------------------------- settings
mkdir -p "$ETC"
ENVF="$ETC/outpost.env"
cur() { [ -f "$ENVF" ] && sed -n "s/^$1=//p" "$ENVF" | tail -1 || true; }
DATA_DIR="${DATA_DIR:-$(cur OUTPOST_DATA_DIR)}"; DATA_DIR="${DATA_DIR:-/var/lib/outpost}"
PORT="${PORT:-$(cur OUTPOST_LISTEN | sed 's/.*://')}"; PORT="${PORT:-8080}"
LIB_PORT="${LIB_PORT:-$(cur OUTPOST_LIBRARY_LISTEN | sed 's/.*://')}"; LIB_PORT="${LIB_PORT:-8081}"
[ "$PORT" != "$LIB_PORT" ] || die "the dashboard and library ports must differ"

port_busy() { # true when another program listens on the port
  command -v ss >/dev/null 2>&1 || return 1
  ss -Hltn "( sport = :$1 )" 2>/dev/null | grep -q . && ! systemctl is-active --quiet outpost
}
for p in "$PORT" "$LIB_PORT"; do
  if port_busy "$p"; then die "port $p is already in use. Re-run with --port/--library-port to pick free ones."; fi
done

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  say "Creating the $SERVICE_USER service account"
  useradd --system --home-dir "$DATA_DIR" --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null \
    || useradd -r -d "$DATA_DIR" -M -s /sbin/nologin "$SERVICE_USER"
fi
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
chmod 750 "$DATA_DIR"

cat > "$ENVF.tmp" <<ENV
# Outpost settings. Edit, then: sudo systemctl restart outpost
OUTPOST_DATA_DIR=$DATA_DIR
OUTPOST_BIN_DIR=$PREFIX/bin
OUTPOST_LISTEN=:$PORT
OUTPOST_LIBRARY_LISTEN=:$LIB_PORT
OUTPOST_OLLAMA_URL=http://127.0.0.1:11434
ENV
mv "$ENVF.tmp" "$ENVF"
chmod 644 "$ENVF"

# ---------------------------------------------------------------- install
say "Installing to $PREFIX"
mkdir -p "$PREFIX/bin.new"
install -m 0755 "$WORK/x/outpost/outpost" "$PREFIX/bin.new/outpost"
install -m 0755 "$WORK/x/kiwix/kiwix-serve" "$PREFIX/bin.new/kiwix-serve"
install -m 0755 "$WORK/x/kiwix/kiwix-manage" "$PREFIX/bin.new/kiwix-manage"
install -m 0755 "$WORK/x/pmtiles/pmtiles" "$PREFIX/bin.new/pmtiles"
"$PREFIX/bin.new/outpost" version >/dev/null || die "the downloaded program does not run on this machine"
rm -rf "$PREFIX/bin.old"
[ -d "$PREFIX/bin" ] && mv "$PREFIX/bin" "$PREFIX/bin.old"
mv "$PREFIX/bin.new" "$PREFIX/bin"
rm -rf "$PREFIX/bin.old"

cat > /usr/local/bin/outpost <<'WRAP'
#!/bin/sh
# Runs the Outpost command with the node's settings.
set -a
[ -r /etc/outpost/outpost.env ] && . /etc/outpost/outpost.env
set +a
exec /opt/outpost/bin/outpost "$@"
WRAP
chmod 755 /usr/local/bin/outpost

PROTECT_HOME=yes
case "$DATA_DIR" in /home/*|/root/*|/run/user/*) PROTECT_HOME=read-only ;; esac

cat > /etc/systemd/system/outpost.service <<UNIT
[Unit]
Description=Outpost offline knowledge server
Documentation=$SITE/docs
After=network-online.target ollama.service
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
EnvironmentFile=$ENVF
ExecStart=$PREFIX/bin/outpost serve
Restart=on-failure
RestartSec=3
LimitNOFILE=65536
# Hardening: the service can write only to its data directory.
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=$PROTECT_HOME
ReadWritePaths=$DATA_DIR
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes
RestrictSUIDSGID=yes
RestrictRealtime=yes
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
CapabilityBoundingSet=
AmbientCapabilities=
SystemCallArchitectures=native
UMask=0022

[Install]
WantedBy=multi-user.target
UNIT

# ---------------------------------------------------------------- AI
if [ "$WITH_AI" -eq 0 ] && ! command -v ollama >/dev/null 2>&1; then
  if ask "Install the optional AI assistant (Ollama, about 1.5 GB plus models)?" n; then WITH_AI=1; fi
fi
if [ "$WITH_AI" -eq 1 ]; then
  if command -v ollama >/dev/null 2>&1; then
    note "Ollama is already installed"
  else
    say "Installing Ollama (from ollama.com)"
    fetch "https://ollama.com/install.sh" "$WORK/ollama-install.sh" || die "cannot download the Ollama installer"
    sh "$WORK/ollama-install.sh" </dev/null || die "the Ollama installer failed; Outpost works without it. Re-run without --with-ai to skip it."
  fi
  systemctl enable --now ollama >/dev/null 2>&1 || true
fi

# ---------------------------------------------------------------- firewall
if [ "$FIREWALL" -eq 1 ]; then
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    ufw allow "$PORT/tcp" comment Outpost >/dev/null && ufw allow "$LIB_PORT/tcp" comment "Outpost library" >/dev/null && note "opened ports $PORT and $LIB_PORT in ufw"
  elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    firewall-cmd -q --permanent --add-port="$PORT/tcp" --add-port="$LIB_PORT/tcp" && firewall-cmd -q --reload && note "opened ports $PORT and $LIB_PORT in firewalld"
  fi
fi

# ---------------------------------------------------------------- start
say "Starting Outpost"
systemctl daemon-reload
systemctl enable outpost.service >/dev/null 2>&1
systemctl restart outpost.service
ok=0
for _ in $(seq 1 30); do
  if curl -fsS -o /dev/null "http://127.0.0.1:$PORT/api/status"; then ok=1; break; fi
  sleep 1
done
[ "$ok" -eq 1 ] || die "Outpost did not start. See: journalctl -u outpost -n 50"

IP="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p' | head -1)"
[ -n "$IP" ] || IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
CODE=""
for _ in 1 2 3 4 5; do [ -r "$DATA_DIR/setup-code" ] && CODE="$(cat "$DATA_DIR/setup-code")" && break; sleep 1; done

printf '\n%sOutpost %s is running.%s\n\n' "$c_bold" "$VERSION" "$c_off"
printf '  Open the dashboard:  %shttp://%s:%s%s\n' "$c_bold" "${IP:-this-machine}" "$PORT" "$c_off"
printf '  On this machine:     http://localhost:%s\n' "$PORT"
if [ -n "$CODE" ]; then
  printf '\n  Setup code:          %s%s%s\n' "$c_bold" "$CODE" "$c_off"
  printf '  %s(You need it once, to claim the node. See it again with: sudo outpost setup-code)%s\n' "$c_dim" "$c_off"
fi
printf '\n  Content is stored in %s\n' "$DATA_DIR"
printf '  Logs: journalctl -u outpost -f    Update: re-run this installer\n\n'
