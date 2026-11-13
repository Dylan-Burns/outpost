#!/usr/bin/env bash
# Outpost installer: turns a Mac or a Linux computer you own into an offline
# knowledge server. Safe to run again: it upgrades in place and keeps your
# content.
#
#   curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash
#   curl -fsSL https://outpost-offline.vercel.app/install.sh | sudo bash -s -- --with-ai
#
# Options:
#   --with-ai            set up the AI assistant (installs Ollama on Linux)
#   --data-dir DIR       where content is stored (default /var/lib/outpost)
#   --port N             dashboard port (default 8080)
#   --library-port N     library port (default 8081)
#   --version V          install a specific release instead of the latest
#   --no-firewall        do not open the ports in the firewall
#   --uninstall          remove Outpost (keeps content unless --purge)
#   --purge              with --uninstall, also delete the data directory
#   -y, --yes            do not ask questions
# Everything runs inside main(), called on the last line, so a download cut
# off part way through runs nothing at all.
main() {
set -Eeuo pipefail

SITE="${OUTPOST_SITE:-https://outpost-offline.vercel.app}"
RELEASE_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBrMpCCBsGNt2QYPvv84kSXqzkIMJIvTJAgadKprEcLV outpost-release"
KIWIX_VERSION="3.8.2"
PMTILES_VERSION="1.31.2"
PREFIX="/opt/outpost"
CLI_DIR="$PREFIX/cli"
ETC="/etc/outpost"
OS="$(uname -s)"
# What differs between the two systems is kept in these few names and in the
# functions marked "per system" below; the rest of the installer is shared.
SERVICE_USER="outpost"
LABEL="org.outpost-offline.node"                 # macOS: launchd job
PLIST="/Library/LaunchDaemons/$LABEL.plist"      # macOS
LOG="/var/log/outpost.log"                       # macOS
MAC_FIREWALL="/usr/libexec/ApplicationFirewall/socketfilterfw"
[ "$OS" != "Darwin" ] || SERVICE_USER="_outpost"

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

[ "$(id -u)" -eq 0 ] || die "run the installer as an administrator, for example: curl -fsSL $SITE/install.sh | sudo bash"
case "$OS" in
  Linux)
    command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ] || die "this system does not run systemd, which Outpost uses to start at boot."
    ;;
  Darwin)
    MAC_VERSION="$(sw_vers -productVersion 2>/dev/null || echo 0)"
    MAC_MAJOR="${MAC_VERSION%%.*}"
    [[ "$MAC_MAJOR" =~ ^[0-9]+$ ]] && [ "$MAC_MAJOR" -ge 13 ] || die "Outpost needs macOS 13 (Ventura) or later. This Mac runs macOS $MAC_VERSION."
    ;;
  *) die "Outpost installs on macOS and Linux. For a Windows PC, see $SITE/install#windows." ;;
esac

ask() { # ask "question" default(y/n)
  local q="$1" def="$2" ans
  if [ "$YES" -eq 1 ] || ! [ -r /dev/tty ]; then [ "$def" = y ]; return; fi
  read -r -p "$q [$( [ "$def" = y ] && echo Y/n || echo y/N )] " ans </dev/tty || ans=""
  ans="${ans:-$def}"; [[ "$ans" =~ ^[Yy] ]]
}

valid_port() { [[ "$1" =~ ^[0-9]+$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]; }
[ -z "$PORT" ] || valid_port "$PORT" || die "--port must be a number from 1 to 65535"
[ -z "$LIB_PORT" ] || valid_port "$LIB_PORT" || die "--library-port must be a number from 1 to 65535"
[ -z "$VERSION" ] || [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "--version looks wrong: $VERSION"
if [ -n "$DATA_DIR" ]; then
  case "$DATA_DIR" in /*) ;; *) die "--data-dir must be an absolute path" ;; esac
  case "$DATA_DIR" in *[!A-Za-z0-9/._-]*) die "--data-dir may only contain letters, digits, / . _ -" ;; esac
fi

# ------------------------------------------------------------ per system
svc_active() { # true while Outpost's own service is loaded and running
  case "$OS" in
    Linux) systemctl is-active --quiet outpost ;;
    Darwin) launchctl print "system/$LABEL" >/dev/null 2>&1 ;;
  esac
}

svc_remove() { # stop the service and forget it
  case "$OS" in
    Linux)
      systemctl disable --now outpost.service >/dev/null 2>&1 || true
      rm -f /etc/systemd/system/outpost.service
      systemctl daemon-reload
      ;;
    Darwin)
      mac_bootout
      rm -f "$PLIST"
      ;;
  esac
}

mac_bootout() { # unload the job and wait until launchd has let go of it
  local i
  launchctl bootout "system/$LABEL" >/dev/null 2>&1 || true
  for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    launchctl print "system/$LABEL" >/dev/null 2>&1 || return 0
    sleep 0.5
  done
  return 0
}

port_busy() { # true when another program listens on the port
  local out=""
  if svc_active; then return 1; fi
  case "$OS" in
    Linux)
      command -v ss >/dev/null 2>&1 || return 1
      out="$(ss -Hltn "( sport = :$1 )" 2>/dev/null || true)"
      ;;
    Darwin)
      out="$(lsof -nP -iTCP:"$1" -sTCP:LISTEN 2>/dev/null || true)"
      ;;
  esac
  [ -n "$out" ]
}

make_user() { # an account that owns the content and nothing else
  if id "$SERVICE_USER" >/dev/null 2>&1; then return 0; fi
  say "Creating the $SERVICE_USER service account"
  case "$OS" in
    Linux)
      useradd --system --home-dir "$DATA_DIR" --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER" 2>/dev/null \
        || useradd -r -d "$DATA_DIR" -M -s /sbin/nologin "$SERVICE_USER"
      ;;
    Darwin)
      # The first number free as both a user and a group, in the range macOS
      # keeps for services.
      local used n=300
      used="$(dscl . -list /Users UniqueID 2>/dev/null | awk '{print $NF}'; dscl . -list /Groups PrimaryGroupID 2>/dev/null | awk '{print $NF}')"
      while grep -qx "$n" <<<"$used"; do
        n=$((n + 1))
        [ "$n" -lt 500 ] || die "no free service account number was found on this Mac"
      done
      dscl . -create "/Groups/$SERVICE_USER"
      dscl . -create "/Groups/$SERVICE_USER" PrimaryGroupID "$n"
      dscl . -create "/Groups/$SERVICE_USER" RealName "Outpost"
      dscl . -create "/Groups/$SERVICE_USER" Password "*"
      dscl . -create "/Users/$SERVICE_USER"
      dscl . -create "/Users/$SERVICE_USER" UniqueID "$n"
      dscl . -create "/Users/$SERVICE_USER" PrimaryGroupID "$n"
      dscl . -create "/Users/$SERVICE_USER" UserShell /usr/bin/false
      dscl . -create "/Users/$SERVICE_USER" NFSHomeDirectory /var/empty
      dscl . -create "/Users/$SERVICE_USER" RealName "Outpost"
      dscl . -create "/Users/$SERVICE_USER" Password "*"
      dscl . -create "/Users/$SERVICE_USER" IsHidden 1
      id "$SERVICE_USER" >/dev/null 2>&1 || die "could not create the $SERVICE_USER account"
      ;;
  esac
}

remove_user() {
  id "$SERVICE_USER" >/dev/null 2>&1 || return 0
  case "$OS" in
    Linux) userdel "$SERVICE_USER" 2>/dev/null || true ;;
    Darwin)
      dscl . -delete "/Users/$SERVICE_USER" >/dev/null 2>&1 || true
      dscl . -delete "/Groups/$SERVICE_USER" >/dev/null 2>&1 || true
      ;;
  esac
}

root_only_dir() { # true when DIR is a real folder that only root can change
  local d="$1" owner="" mode=""
  if [ ! -e "$d" ] && [ ! -L "$d" ]; then mkdir -p "$d" 2>/dev/null || return 1; fi
  if [ -L "$d" ] || [ ! -d "$d" ]; then return 1; fi
  case "$OS" in
    Linux) owner="$(stat -c %u "$d" 2>/dev/null || true)"; mode="$(stat -c %a "$d" 2>/dev/null || true)" ;;
    Darwin) owner="$(stat -f %u "$d" 2>/dev/null || true)"; mode="$(stat -f %Lp "$d" 2>/dev/null || true)" ;;
  esac
  [ "$owner" = "0" ] || return 1
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || return 1
  # The last two digits are the group's and everyone else's rights; 2 is "write".
  case "${mode: -2}" in *[2367]*) return 1 ;; esac
  # An access list can grant rights the mode does not show; "+" marks one.
  case "$(ls -ld "$d" 2>/dev/null | cut -c11)" in +) return 1 ;; esac
  return 0
}

root_owned() { # root_owned PATH...: root's, and nobody else may change it
  # Folders are otherwise created with whatever the system's defaults give,
  # and some systems default to letting everyone write.
  chown 0:0 "$@"
  chmod 755 "$@"
}

unlink_cli() { # take away only the links and files this installer put in place
  local target=""
  if [ -L /usr/local/bin/outpost ]; then
    target="$(readlink /usr/local/bin/outpost 2>/dev/null || true)"
    [ "$target" != "$CLI_DIR/outpost" ] || unlink /usr/local/bin/outpost
  elif [ -f /usr/local/bin/outpost ] && grep -q '^exec /opt/outpost/bin/outpost "\$@"$' /usr/local/bin/outpost 2>/dev/null; then
    unlink /usr/local/bin/outpost # the wrapper file that releases before 1.1.0 wrote
  fi
}

lan_ip() {
  local ip="" dev=""
  case "$OS" in
    Linux)
      ip="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p' | head -1 || true)"
      [ -n "$ip" ] || ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
      ;;
    Darwin)
      dev="$(route -n get default 2>/dev/null | awk '/interface:/ {print $2}' || true)"
      [ -z "$dev" ] || ip="$(ipconfig getifaddr "$dev" 2>/dev/null || true)"
      ;;
  esac
  printf '%s' "$ip"
}

# ---------------------------------------------------------------- uninstall
if [ "$UNINSTALL" -eq 1 ]; then
  say "Removing Outpost"
  existing_data="/var/lib/outpost"
  if [ -f "$ETC/outpost.env" ]; then existing_data="$(sed -n 's/^OUTPOST_DATA_DIR=//p' "$ETC/outpost.env" | tail -1)"; fi
  svc_remove
  if [ "$OS" = "Darwin" ] && [ -x "$MAC_FIREWALL" ] && [ -e "$PREFIX/bin/outpost" ]; then
    "$MAC_FIREWALL" --remove "$PREFIX/bin/outpost" >/dev/null 2>&1 || true
  fi
  unlink_cli
  rm -rf "$PREFIX"
  if [ "$PURGE" -eq 1 ]; then
    if [ -n "$existing_data" ] && [ "$existing_data" != "/" ] && { [ -f "$existing_data/state.json" ] || [ -d "$existing_data/content" ]; }; then
      rm -rf "$existing_data"; note "deleted $existing_data"
    fi
    rm -rf "$ETC"
    [ "$OS" != "Darwin" ] || rm -f "$LOG"
    remove_user
  else
    note "your content is still in $existing_data (use --uninstall --purge to delete it)"
  fi
  note "Ollama, if installed, was left in place."
  say "Done."
  exit 0
fi

# ---------------------------------------------------------------- platform
MACHINE="$(uname -m)"
# A terminal running under Rosetta reports Intel on an Apple Silicon Mac.
if [ "$OS" = "Darwin" ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = "1" ]; then MACHINE=arm64; fi
case "$OS-$MACHINE" in
  Linux-x86_64|Linux-amd64) GOOS=linux; ARCH=amd64
    KT="kiwix-tools_linux-x86_64-$KIWIX_VERSION.tar.gz"
    KIWIX_SHA="b0ae98dd344aa0469a15ab42feff6d5aafb79541a82fb4e2647c74b073123815"
    PT="go-pmtiles_${PMTILES_VERSION}_Linux_x86_64.tar.gz"
    PMT_SHA="3ed7dbf4ec2e6dfe5e25b6f70d1ffc932729f93c86db353bf514dd71010a312f" ;;
  Linux-aarch64|Linux-arm64) GOOS=linux; ARCH=arm64
    KT="kiwix-tools_linux-aarch64-$KIWIX_VERSION.tar.gz"
    KIWIX_SHA="a9898fc5363437327350db669bbc03dcc7ecfd498dae727d7184bebe77019514"
    PT="go-pmtiles_${PMTILES_VERSION}_Linux_arm64.tar.gz"
    PMT_SHA="f8bd47e7ea866863489cad588fbaf2f31f42e5821f7a03f009b3769f05801cb1" ;;
  Darwin-x86_64) GOOS=darwin; ARCH=amd64
    KT="kiwix-tools_macos-x86_64-$KIWIX_VERSION.tar.gz"
    KIWIX_SHA="ae341280e97ad5902e7a4282a24340eac7d7061c267661c0ef2a88a34547f247"
    PT="go-pmtiles-${PMTILES_VERSION}_Darwin_x86_64.zip"
    PMT_SHA="1f0dc02eee6c58312dd6c509faee1b5c32f0596568af1bf51f1b034e7a88a65b" ;;
  Darwin-arm64) GOOS=darwin; ARCH=arm64
    KT="kiwix-tools_macos-arm64-$KIWIX_VERSION.tar.gz"
    KIWIX_SHA="5c64d43176627e558a117146b02ea36b7da2b0cd3a332cbff075cde05d9585f9"
    PT="go-pmtiles-${PMTILES_VERSION}_Darwin_arm64.zip"
    PMT_SHA="40528f7f616fcbf91207cd48c8fc023d213f6d86c0cbf1f748732803d1880f3d" ;;
  Linux-*) die "this processor ($MACHINE) is not supported yet. Outpost runs on 64-bit Intel/AMD and ARM (e.g. Raspberry Pi 4/5 with a 64-bit OS)." ;;
  *) die "this processor ($MACHINE) is not supported yet. Outpost runs on Intel and Apple Silicon Macs." ;;
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
if [ "$OS" = "Linux" ]; then
  need=()
  command -v curl >/dev/null 2>&1 || need+=(curl)
  command -v tar >/dev/null 2>&1 || need+=(tar)
  command -v gzip >/dev/null 2>&1 || need+=(gzip)
  command -v sha256sum >/dev/null 2>&1 || need+=(coreutils)
  if ! command -v ssh-keygen >/dev/null 2>&1; then
    if command -v pacman >/dev/null 2>&1; then need+=(openssh); elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then need+=(openssh); else need+=(openssh-client); fi
  fi
  if [ ${#need[@]} -gt 0 ]; then say "Installing ${need[*]}"; pkg_install "${need[@]}"; fi
else
  # Every Mac ships these; say so plainly if one has been removed.
  for tool in curl tar unzip shasum ssh-keygen launchctl dscl; do
    command -v "$tool" >/dev/null 2>&1 || die "this Mac is missing '$tool', which is part of macOS."
  done
fi
ssh_ver="$(ssh -V 2>&1 | sed -n 's/^OpenSSH_\([0-9]*\)\.\([0-9]*\).*/\1 \2/p' | head -1)"
if [ -n "$ssh_ver" ]; then
  set -- $ssh_ver
  if [ "$1" -lt 8 ] || { [ "$1" -eq 8 ] && [ "$2" -lt 1 ]; }; then
    die "this system's OpenSSH ($1.$2) is too old to check signatures (need 8.1 or later)."
  fi
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
verify_sig() { # verify_sig NAMESPACE FILE SIGFILE
  printf 'outpost-release %s\n' "$RELEASE_KEY" > "$WORK/allowed_signers"
  ssh-keygen -Y verify -f "$WORK/allowed_signers" -I outpost-release -n "$1" -s "$3" < "$2" >/dev/null 2>&1
}
fetch() { curl -fsSL --proto '=https' --tlsv1.2 --retry 4 --retry-delay 2 --connect-timeout 20 -o "$2" "$1"; }
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}
sha256_is() { # sha256_is FILE EXPECTED: true only for an exact, full-length match
  local got
  [[ "$2" =~ ^[0-9a-f]{64}$ ]] || return 1
  got="$(sha256_of "$1")"
  [ "$got" = "$2" ]
}
newer() { # newer A B: true when version A is later than version B
  local a b i
  IFS=. read -r -a a <<<"$1"
  IFS=. read -r -a b <<<"$2"
  for i in 0 1 2; do
    if [ "${a[$i]}" -gt "${b[$i]}" ]; then return 0; fi
    if [ "${a[$i]}" -lt "${b[$i]}" ]; then return 1; fi
  done
  return 1
}

# ---------------------------------------------------------------- release
if [ -z "$VERSION" ]; then
  fetch "$SITE/releases/latest.json" "$WORK/latest.json" || die "cannot reach $SITE. Is this machine online?"
  fetch "$SITE/releases/latest.json.sig" "$WORK/latest.json.sig" || die "cannot download the release list signature"
  verify_sig outpost-release-index "$WORK/latest.json" "$WORK/latest.json.sig" \
    || die "the release list signature does not verify. Do not continue; report it at $SITE."
  VERSION="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$WORK/latest.json" | head -1)"
  [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "the release list looks wrong"
  if [ -x "$PREFIX/bin/outpost" ]; then
    INSTALLED="$("$PREFIX/bin/outpost" version 2>/dev/null || true)"
    if [[ "$INSTALLED" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] && newer "$INSTALLED" "$VERSION"; then
      die "Outpost $INSTALLED is newer than the published $VERSION. To go back on purpose, pass --version $VERSION."
    fi
  fi
fi
REL="$SITE/releases/v$VERSION"
TARBALL="outpost-$VERSION-$GOOS-$ARCH.tar.gz"
say "Downloading Outpost $VERSION ($GOOS, $ARCH)"
fetch "$REL/SHA256SUMS" "$WORK/SHA256SUMS" || die "cannot download $REL/SHA256SUMS"
fetch "$REL/SHA256SUMS.sig" "$WORK/SHA256SUMS.sig" || die "cannot download the release signature"
verify_sig outpost-release "$WORK/SHA256SUMS" "$WORK/SHA256SUMS.sig" \
  || die "the release signature does not verify. Do not use this download; report it at $SITE."
note "release signature verified"
WANT="$(awk -v f="$TARBALL" '$2 == f { print $1; exit }' "$WORK/SHA256SUMS")"
if [ -z "$WANT" ]; then
  if [ "$GOOS" = "darwin" ]; then die "Outpost $VERSION was not published for Macs. Mac support starts with version 1.1.0."; fi
  die "Outpost $VERSION was not published for this kind of machine."
fi
fetch "$REL/$TARBALL" "$WORK/$TARBALL" || die "cannot download $REL/$TARBALL"
sha256_is "$WORK/$TARBALL" "$WANT" || die "the download does not match its signed checksum"
note "checksum verified"

say "Downloading kiwix-tools $KIWIX_VERSION and pmtiles $PMTILES_VERSION"
fetch "https://download.kiwix.org/release/kiwix-tools/$KT" "$WORK/$KT" || die "cannot download kiwix-tools"
sha256_is "$WORK/$KT" "$KIWIX_SHA" || die "kiwix-tools does not match its pinned checksum"
fetch "https://github.com/protomaps/go-pmtiles/releases/download/v$PMTILES_VERSION/$PT" "$WORK/$PT" || die "cannot download pmtiles"
sha256_is "$WORK/$PT" "$PMT_SHA" || die "pmtiles does not match its pinned checksum"
note "checksums verified"

mkdir -p "$WORK/x/outpost" "$WORK/x/kiwix" "$WORK/x/pmtiles"
tar -xzf "$WORK/$TARBALL" -C "$WORK/x/outpost"
tar -xzf "$WORK/$KT" -C "$WORK/x/kiwix" --strip-components=1
case "$PT" in
  *.zip) unzip -q -o "$WORK/$PT" -d "$WORK/x/pmtiles" ;;
  *) tar -xzf "$WORK/$PT" -C "$WORK/x/pmtiles" ;;
esac
for f in "$WORK/x/outpost/outpost" "$WORK/x/kiwix/kiwix-serve" "$WORK/x/kiwix/kiwix-manage" "$WORK/x/pmtiles/pmtiles"; do
  [ -f "$f" ] || die "a downloaded archive is missing $(basename "$f")"
done

# ---------------------------------------------------------------- settings
mkdir -p "$ETC"
root_owned "$ETC"
ENVF="$ETC/outpost.env"
cur() { [ -f "$ENVF" ] && sed -n "s/^$1=//p" "$ENVF" | tail -1 || true; }
DATA_DIR="${DATA_DIR:-$(cur OUTPOST_DATA_DIR)}"; DATA_DIR="${DATA_DIR:-/var/lib/outpost}"
PORT="${PORT:-$(cur OUTPOST_LISTEN | sed 's/.*://')}"; PORT="${PORT:-8080}"
LIB_PORT="${LIB_PORT:-$(cur OUTPOST_LIBRARY_LISTEN | sed 's/.*://')}"; LIB_PORT="${LIB_PORT:-8081}"
[ "$PORT" != "$LIB_PORT" ] || die "the dashboard and library ports must differ"

for p in "$PORT" "$LIB_PORT"; do
  if port_busy "$p"; then die "port $p is already in use. Re-run with --port/--library-port to pick free ones."; fi
done

make_user
[ ! -L "$DATA_DIR" ] || die "$DATA_DIR is a symbolic link. Give --data-dir the real folder."
mkdir -p "$DATA_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
chmod 750 "$DATA_DIR"
if [ "$OS" = "Darwin" ]; then
  # Content can be downloaded again; keep it out of Time Machine backups.
  tmutil addexclusion "$DATA_DIR" >/dev/null 2>&1 || true
  case "$DATA_DIR" in /Volumes/*) note "content is on an external drive: if downloads fail with 'operation not permitted', allow $PREFIX/bin/outpost under System Settings > Privacy & Security > Full Disk Access" ;; esac
fi

RESTART_HINT="sudo systemctl restart outpost"
[ "$OS" != "Darwin" ] || RESTART_HINT="sudo launchctl kickstart -k system/$LABEL"
cat > "$ENVF.tmp" <<ENV
# Outpost settings. Edit, then: $RESTART_HINT
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
root_owned "$PREFIX" "$PREFIX/bin.new"
install -m 0755 "$WORK/x/outpost/outpost" "$PREFIX/bin.new/outpost"
install -m 0755 "$WORK/x/kiwix/kiwix-serve" "$PREFIX/bin.new/kiwix-serve"
install -m 0755 "$WORK/x/kiwix/kiwix-manage" "$PREFIX/bin.new/kiwix-manage"
install -m 0755 "$WORK/x/pmtiles/pmtiles" "$PREFIX/bin.new/pmtiles"
"$PREFIX/bin.new/outpost" version >/dev/null || die "the downloaded program does not run on this machine"
"$PREFIX/bin.new/kiwix-serve" --version >/dev/null 2>&1 || die "the library engine (kiwix-serve) does not run on this machine"
"$PREFIX/bin.new/pmtiles" version >/dev/null 2>&1 || die "the map tool (pmtiles) does not run on this machine"
rm -rf "$PREFIX/bin.old"
[ -d "$PREFIX/bin" ] && mv "$PREFIX/bin" "$PREFIX/bin.old"
mv "$PREFIX/bin.new" "$PREFIX/bin"
rm -rf "$PREFIX/bin.old"

# The command people run with sudo. It lives in a folder only root can
# change. /usr/local/bin gets a link to it only when that folder is just as
# safe. On many Macs it belongs to the user, and a short name found there
# could be swapped for something else before the next "sudo outpost"; those
# machines are given the full path and nothing is added to their PATH.
mkdir -p "$CLI_DIR"
root_owned "$CLI_DIR"
cat > "$CLI_DIR/outpost.new" <<'WRAP'
#!/bin/sh
# Runs the Outpost command with the node's settings.
set -a
[ -r /etc/outpost/outpost.env ] && . /etc/outpost/outpost.env
set +a
exec /opt/outpost/bin/outpost "$@"
WRAP
root_owned "$CLI_DIR/outpost.new"
mv -f "$CLI_DIR/outpost.new" "$CLI_DIR/outpost"
CLI="$CLI_DIR/outpost"
unlink_cli
if root_only_dir /usr/local && root_only_dir /usr/local/bin; then
  ln -s "$CLI_DIR/outpost" /usr/local/bin/outpost
  CLI="outpost"
else
  note "/usr/local/bin can be changed without a password on this machine, so no short command was put there."
  note "Always use the full name: sudo $CLI_DIR/outpost"
  if [ -e /usr/local/bin/outpost ] || [ -L /usr/local/bin/outpost ]; then
    note "Careful: /usr/local/bin/outpost exists but is not Outpost's command. Do not run it with sudo."
  fi
fi

if [ "$OS" = "Linux" ]; then
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
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX AF_NETLINK
CapabilityBoundingSet=
AmbientCapabilities=
SystemCallArchitectures=native
UMask=0022

[Install]
WantedBy=multi-user.target
UNIT
chown 0:0 /etc/systemd/system/outpost.service
chmod 644 /etc/systemd/system/outpost.service
else
# macOS: a launchd job that starts at boot, before anyone signs in, and runs
# as the service account. It reads the same settings file as on Linux.
touch "$LOG"
chown "$SERVICE_USER:$SERVICE_USER" "$LOG"
chmod 640 "$LOG"
cat > "$PLIST.tmp" <<JOB
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-c</string>
    <string>set -a; . $ENVF; set +a; exec $PREFIX/bin/outpost serve</string>
  </array>
  <key>UserName</key>
  <string>$SERVICE_USER</string>
  <key>GroupName</key>
  <string>$SERVICE_USER</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
  <key>ThrottleInterval</key>
  <integer>5</integer>
  <key>StandardOutPath</key>
  <string>$LOG</string>
  <key>StandardErrorPath</key>
  <string>$LOG</string>
  <key>SoftResourceLimits</key>
  <dict>
    <key>NumberOfFiles</key>
    <integer>8192</integer>
  </dict>
  <key>Umask</key>
  <integer>18</integer>
</dict>
</plist>
JOB
plutil -lint "$PLIST.tmp" >/dev/null || die "could not write the launchd job"
fi

# ---------------------------------------------------------------- AI
if [ "$OS" = "Linux" ]; then
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
elif [ "$WITH_AI" -eq 1 ]; then
  # On a Mac, Ollama is an app the owner installs; Outpost finds it once it runs.
  if command -v ollama >/dev/null 2>&1 || [ -d /Applications/Ollama.app ]; then
    note "Ollama is already installed; keep it open and the assistant will find it"
  else
    note "The AI assistant uses the Ollama app. Download it from https://ollama.com/download and open it once;"
    note "the assistant then appears in the Outpost dashboard."
  fi
fi

# ---------------------------------------------------------------- firewall
if [ "$FIREWALL" -eq 1 ]; then
  if [ "$OS" = "Linux" ]; then
    ufw_state=""; command -v ufw >/dev/null 2>&1 && ufw_state="$(ufw status 2>/dev/null || true)"
    case "$ufw_state" in
      *"Status: active"*)
        ufw allow "$PORT/tcp" comment Outpost >/dev/null && ufw allow "$LIB_PORT/tcp" comment "Outpost library" >/dev/null && note "opened ports $PORT and $LIB_PORT in ufw" ;;
      *)
        if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
          firewall-cmd -q --permanent --add-port="$PORT/tcp" --add-port="$LIB_PORT/tcp" && firewall-cmd -q --reload && note "opened ports $PORT and $LIB_PORT in firewalld"
        fi ;;
    esac
  elif [ -x "$MAC_FIREWALL" ]; then
    # The macOS firewall works per program, not per port.
    fw_state="$("$MAC_FIREWALL" --getglobalstate 2>/dev/null || true)"
    case "$fw_state" in
      *"State = 2"*)
        note "the macOS firewall is set to block all incoming connections, so other devices cannot reach Outpost."
        note "Change it under System Settings > Network > Firewall > Options." ;;
      *"State = 1"*|*enabled*)
        "$MAC_FIREWALL" --add "$PREFIX/bin/outpost" >/dev/null 2>&1 || true
        "$MAC_FIREWALL" --unblockapp "$PREFIX/bin/outpost" >/dev/null 2>&1 || true
        note "allowed Outpost through the macOS firewall" ;;
    esac
  fi
fi

# ---------------------------------------------------------------- start
say "Starting Outpost"
if [ "$OS" = "Linux" ]; then
  systemctl daemon-reload
  systemctl enable outpost.service >/dev/null 2>&1
  systemctl restart outpost.service
  LOGS_HINT="journalctl -u outpost -f"
  FAIL_HINT="journalctl -u outpost -n 50"
else
  mac_bootout
  mv "$PLIST.tmp" "$PLIST"
  chown root:wheel "$PLIST"
  chmod 644 "$PLIST"
  launchctl enable "system/$LABEL" >/dev/null 2>&1 || true
  launchctl bootstrap system "$PLIST" || die "macOS would not start the Outpost job. See: tail -n 50 $LOG"
  LOGS_HINT="tail -f $LOG"
  FAIL_HINT="tail -n 50 $LOG"
fi
ok=0
for _ in $(seq 1 30); do
  if curl -fs -o /dev/null "http://127.0.0.1:$PORT/api/status"; then ok=1; break; fi
  sleep 1
done
[ "$ok" -eq 1 ] || die "Outpost did not start. See: $FAIL_HINT"

IP="$(lan_ip)"
CODE=""
for _ in 1 2 3 4 5; do [ -r "$DATA_DIR/setup-code" ] && CODE="$(cat "$DATA_DIR/setup-code")" && break; sleep 1; done

printf '\n%sOutpost %s is running.%s\n\n' "$c_bold" "$VERSION" "$c_off"
printf '  Open the dashboard:  %shttp://%s:%s%s\n' "$c_bold" "${IP:-this-machine}" "$PORT" "$c_off"
printf '  On this machine:     http://localhost:%s\n' "$PORT"
if [ -n "$CODE" ]; then
  printf '\n  Setup code:          %s%s%s\n' "$c_bold" "$CODE" "$c_off"
  printf '  %s(You need it once, to claim the node. See it again with: sudo %s setup-code)%s\n' "$c_dim" "$CLI" "$c_off"
fi
printf '\n  Content is stored in %s\n' "$DATA_DIR"
printf '  Logs: %s    Update: re-run this installer\n\n' "$LOGS_HINT"
}

main "$@"
