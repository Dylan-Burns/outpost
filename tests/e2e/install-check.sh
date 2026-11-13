#!/usr/bin/env bash
# Installs the release staged in site/public with the real installer on this
# machine, drives the node over its API, then upgrades, restarts, resets and
# removes it. It checks what a person would see, on Linux and on macOS.
#
# It needs sudo without a password and leaves nothing behind when it passes:
# it is meant for CI runners and scratch machines, not a computer you use.
#
#   tests/e2e/install-check.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OS="$(uname -s)"
WORK="$(mktemp -d)"
SITE_PORT=8443
DASH="http://127.0.0.1:8080"
LIB="http://127.0.0.1:8081"
LABEL="org.outpost-offline.node"
SERVER=""
case "$OS" in
  Linux) SERVICE_USER=outpost; SYSTEM_CA=/etc/ssl/certs/ca-certificates.crt ;;
  Darwin) SERVICE_USER=_outpost; SYSTEM_CA=/etc/ssl/cert.pem ;;
  *) echo "unsupported system: $OS" >&2; exit 2 ;;
esac

cleanup() { [ -z "$SERVER" ] || kill "$SERVER" 2>/dev/null || true; rm -rf "$WORK"; }
trap cleanup EXIT
step() { printf '\n=== %s\n' "$*"; }
ok() { printf 'ok  %s\n' "$*"; }
fail() {
  printf 'FAIL: %s\n' "$*" >&2
  if [ "$OS" = "Darwin" ]; then sudo tail -n 40 /var/log/outpost.log 2>/dev/null || true; else sudo journalctl -u outpost -n 40 --no-pager 2>/dev/null || true; fi
  exit 1
}
json() { # json FIELD: read one top-level field from JSON on stdin
  node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const v=JSON.parse(s)[process.argv[1]];console.log(typeof v==="object"?JSON.stringify(v):String(v))})' "$1"
}
wait_up() {
  for _ in $(seq 1 40); do
    if curl -fs -o /dev/null "$DASH/api/status"; then return 0; fi
    sleep 1
  done
  return 1
}
service_running() {
  if [ "$OS" = "Darwin" ]; then sudo launchctl print "system/$LABEL" >/dev/null 2>&1; else systemctl is-active --quiet outpost; fi
}

cmp -s "$ROOT/install/install.sh" "$ROOT/site/public/install.sh" || fail "site/public/install.sh is not the installer in install/ (run scripts/release.sh)"
VERSION="$(json version < "$ROOT/site/public/releases/latest.json")"

step "Serve the staged site over HTTPS on this machine"
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$WORK/key.pem" -out "$WORK/cert.pem" -days 2 \
  -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost" >/dev/null 2>&1
# The installer also downloads from kiwix.org and github.com, so it needs the
# usual authorities as well as this one.
cat "$SYSTEM_CA" "$WORK/cert.pem" > "$WORK/bundle.pem"
node "$ROOT/tests/e2e/serve-site.mjs" "$ROOT/site/public" "$SITE_PORT" "$WORK/key.pem" "$WORK/cert.pem" &
SERVER=$!
sleep 2
curl -fs --cacert "$WORK/bundle.pem" -o /dev/null "https://localhost:$SITE_PORT/releases/latest.json" || fail "the local site is not answering"
run_installer() { sudo env OUTPOST_SITE="https://localhost:$SITE_PORT" CURL_CA_BUNDLE="$WORK/bundle.pem" bash "$ROOT/install/install.sh" --yes "$@"; }
# This test installs three times in a few minutes, and each install fetches
# kiwix-tools from download.kiwix.org, which answers 429 ("too many requests")
# to callers that come too fast, CI machines above all. That is the server
# being reasonable, not the installer failing: wait and try again.
installer() {
  local try
  for try in 1 2 3 4; do
    if run_installer "$@" > "$WORK/attempt.log" 2>&1; then cat "$WORK/attempt.log"; return 0; fi
    if [ "$try" -lt 4 ] && grep -q "error: 429" "$WORK/attempt.log"; then
      echo "note: download.kiwix.org asked us to slow down; waiting $((try * 90)) seconds" >&2
      sleep $((try * 90))
      continue
    fi
    cat "$WORK/attempt.log"
    return 1
  done
}

if [ "$OS" = "Darwin" ]; then
  # Most Macs that turn the firewall on leave it on; test that case.
  sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on >/dev/null 2>&1 || true
fi

step "Install Outpost $VERSION"
installer 2>&1 | tee "$WORK/install.log"
grep -q "Outpost $VERSION is running" "$WORK/install.log" || fail "the installer did not report a running node"
grep -q "release signature verified" "$WORK/install.log" || fail "the installer did not verify the release signature"
grep -q "Setup code:" "$WORK/install.log" || fail "the installer did not print a setup code"
[ "$(curl -fs "$DASH/api/status" | json version)" = "$VERSION" ] || fail "the node reports a different version"
service_running || fail "the service is not registered with the system"
OWNER="$(ps -axo user=,comm= | awk '$2 ~ /\/outpost$/ || $2 == "outpost" {print $1; exit}')"
[ "$OWNER" = "$SERVICE_USER" ] || fail "the node runs as '$OWNER', not as $SERVICE_USER"
ok "runs as $SERVICE_USER, not as an administrator"
DATA_MODE="$(sudo ls -ld /var/lib/outpost | awk '{print $1, $3}')"
case "$DATA_MODE" in "drwxr-x---"*" $SERVICE_USER") ok "content folder is private to the service account" ;; *) fail "content folder has the wrong owner or mode: $DATA_MODE" ;; esac
if [ "$OS" = "Darwin" ]; then
  sudo test -f "/Library/LaunchDaemons/$LABEL.plist" || fail "the launchd job file is missing"
  sudo /usr/libexec/ApplicationFirewall/socketfilterfw --listapps 2>/dev/null | grep -q "/opt/outpost/bin/outpost" \
    && ok "allowed through the macOS firewall" || echo "note: the firewall list does not show Outpost (firewall may be off on this machine)"
else
  [ "$(systemctl is-enabled outpost)" = "enabled" ] || fail "the service is not enabled at boot"
fi
# The command people run with sudo must come from a folder only root can change.
[ -x /opt/outpost/cli/outpost ] || fail "the outpost command is missing"
for d in /opt/outpost /opt/outpost/bin /opt/outpost/cli /etc/outpost; do
  D_OWNER="$(ls -ld "$d" | awk '{print $1, $3}')"
  case "$D_OWNER" in "drwxr-xr-x"*" root") ;; *) fail "$d is not root-only: $D_OWNER" ;; esac
done
for f in /opt/outpost/bin/outpost /opt/outpost/bin/kiwix-serve /opt/outpost/bin/pmtiles /opt/outpost/cli/outpost; do
  F_OWNER="$(ls -l "$f" | awk '{print $1, $3}')"
  case "$F_OWNER" in "-rwxr-xr-x"*" root") ;; *) fail "$f is not root-only: $F_OWNER" ;; esac
done
ok "program folders and files can only be changed by root"
if [ -L /usr/local/bin/outpost ] || [ -e /usr/local/bin/outpost ]; then
  [ "$(readlink /usr/local/bin/outpost)" = "/opt/outpost/cli/outpost" ] || fail "/usr/local/bin/outpost is not a link to the installed command"
  BIN_OWNER="$(ls -ld /usr/local/bin | awk '{print $1, $3}')"
  case "$BIN_OWNER" in "drwxr-xr-x"*" root") ok "linked from /usr/local/bin, which only root can change" ;; *) fail "a link was placed in a folder others can change: $BIN_OWNER" ;; esac
else
  # No short name: that is only right when /usr/local/bin is not root-only,
  # and then the installer must have said to use the full path and must not
  # have added anything to the PATH.
  BIN_OWNER="$(ls -ld /usr/local/bin 2>/dev/null | awk '{print $1, $3}')"
  case "$BIN_OWNER" in "drwxr-xr-x"*" root") fail "/usr/local/bin is root-only but the command was not linked there" ;; esac
  grep -q "sudo /opt/outpost/cli/outpost" "$WORK/install.log" || fail "the installer did not give the full path of the command"
  [ ! -e /etc/paths.d/outpost ] || fail "a PATH entry was added below a folder others can change"
  ok "/usr/local/bin is user-writable here ($BIN_OWNER): no short command, full path given"
fi
ok "installed and running"

step "Use it: claim, download, read, search, map"
CODE="$(sudo /opt/outpost/cli/outpost setup-code)"
node "$ROOT/tests/e2e/smoke.mjs" "$DASH" "$LIB" "$CODE" || fail "the smoke test failed"
BOOK="$LIB/content/devdocs_en_python/"

step "Run the installer again (an update keeps content and settings)"
installer > "$WORK/again.log" 2>&1 || { cat "$WORK/again.log"; fail "re-running the installer failed"; }
wait_up || fail "the node did not come back after an update"
[ "$(curl -fs "$DASH/api/status" | json claimed)" = "true" ] || fail "the node forgot its administrator after an update"
sleep 3
curl -fs -o /dev/null "$BOOK" || fail "the library lost its content after an update"
ok "update kept the administrator and the content"

step "Restart the service (stands in for a reboot)"
if [ "$OS" = "Darwin" ]; then sudo launchctl kickstart -k "system/$LABEL"; else sudo systemctl restart outpost; fi
sleep 2
wait_up || fail "the node did not come back after a restart"
for _ in $(seq 1 30); do curl -fs -o /dev/null "$BOOK" && break; sleep 1; done
curl -fs -o /dev/null "$BOOK" || fail "the library did not come back after a restart"
ok "back up with its library after a restart"

step "Reset the administrator"
sudo /opt/outpost/cli/outpost reset-admin 2>&1 | tee "$WORK/reset.log" >/dev/null || { cat "$WORK/reset.log"; fail "reset-admin failed"; }
grep -Eq '^ +[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$' "$WORK/reset.log" || { cat "$WORK/reset.log"; fail "reset-admin printed no setup code"; }
wait_up || fail "the node did not come back after reset-admin"
[ "$(curl -fs "$DASH/api/status" | json claimed)" = "false" ] || fail "the node still has an administrator after reset-admin"
service_running || fail "the service was not started again after reset-admin"
ok "administrator removed, service back, new setup code issued"

step "Uninstall, keeping content"
installer --uninstall > "$WORK/remove.log" 2>&1 || { cat "$WORK/remove.log"; fail "uninstall failed"; }
if curl -fs -o /dev/null "$DASH/api/status"; then fail "the node still answers after uninstall"; fi
[ ! -e /opt/outpost ] || fail "/opt/outpost is still there after uninstall"
sudo test -f /var/lib/outpost/state.json || fail "uninstall deleted the content without --purge"
ok "program removed, content kept"

step "Install again over the kept content"
installer > "$WORK/third.log" 2>&1 || { cat "$WORK/third.log"; fail "reinstall failed"; }
wait_up || fail "the node did not start after reinstall"
for _ in $(seq 1 30); do curl -fs -o /dev/null "$BOOK" && break; sleep 1; done
curl -fs -o /dev/null "$BOOK" || fail "the kept content was not picked up after reinstall"
ok "kept content is served again"

step "Uninstall and delete everything"
installer --uninstall --purge > "$WORK/purge.log" 2>&1 || { cat "$WORK/purge.log"; fail "purge failed"; }
[ ! -e /var/lib/outpost ] || fail "the content folder is still there after --purge"
[ ! -e /etc/outpost ] || fail "/etc/outpost is still there after --purge"
if [ -e /usr/local/bin/outpost ] || [ -L /usr/local/bin/outpost ] || [ -e /etc/paths.d/outpost ]; then fail "the outpost command is still there after --purge"; fi
if id "$SERVICE_USER" >/dev/null 2>&1; then fail "the $SERVICE_USER account is still there after --purge"; fi
if [ "$OS" = "Darwin" ]; then
  [ ! -e "/Library/LaunchDaemons/$LABEL.plist" ] || fail "the launchd job file is still there after --purge"
else
  [ ! -e /etc/systemd/system/outpost.service ] || fail "the systemd unit is still there after --purge"
fi
ok "nothing left behind"

printf '\nPASS: install, use, update, restart, reset, uninstall on %s %s\n' "$OS" "$(uname -m)"
