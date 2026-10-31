#!/bin/sh
# magpie installer: curl -fsSL https://usemagpie.ai/install.sh | sh
#
# macOS: puts magpie.app in /Applications (~/Applications when that is not
# writable) and links the magpie command into ~/.local/bin.
# Linux: puts magpie in ~/.local/bin — the desktop app when GTK 3 and
# WebKitGTK 4.1 are installed (with a menu entry), the terminal build
# otherwise or with MAGPIE_CLI=1.
# Termux: installs the Android terminal build into $PREFIX/bin.
# Every download is checked against the release's SHA-256.
#
# Options (curl -fsSL https://usemagpie.ai/install.sh | sh -s -- --proxy …):
#   --proxy <url>      a proxy for every download: http://, https://,
#                      socks5:// or socks5h:// (MAGPIE_PROXY= does the same;
#                      without either, curl's own https_proxy / HTTPS_PROXY /
#                      ALL_PROXY are used as usual)
#   --mirror <prefix>  a GitHub download mirror: the release file is fetched
#                      from <prefix><its github.com URL> (MAGPIE_MIRROR= does
#                      the same). None is used unless given. The release
#                      list and its SHA-256 still come from usemagpie.ai,
#                      never from the mirror, so a file the mirror changed is
#                      refused.
#   --uninstall        takes off what this script put there: the magpie
#                      command, and the app (macOS) or its menu entry, icon
#                      and magpie:// handler (Linux), and the service and
#                      opening at login that would start a magpie no longer
#                      there. Your settings, providers and usage in
#                      ~/.config/magpie stay, for when you install again.
#   --purge            --uninstall, and before it `magpie uninstall --yes`:
#                      every agent disconnected and put back as it was, and
#                      magpie's settings, providers and usage deleted.
set -eu

site=https://usemagpie.ai
bin="${MAGPIE_BIN_DIR:-$HOME/.local/bin}"
proxy="${MAGPIE_PROXY:-}"
mirror="${MAGPIE_MIRROR:-}"
uninstall=

say() { printf '  %s\n' "$*"; }
die() { printf 'magpie: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --proxy) [ $# -ge 2 ] || die "--proxy needs a value"; proxy=$2; shift 2 ;;
    --proxy=*) proxy=${1#--proxy=}; shift ;;
    --mirror) [ $# -ge 2 ] || die "--mirror needs a value"; mirror=$2; shift 2 ;;
    --mirror=*) mirror=${1#--mirror=}; shift ;;
    --uninstall) [ "$uninstall" = purge ] || uninstall=1; shift ;;
    --purge) uninstall=purge; shift ;;
    -h|--help)
      printf '%s\n' "usage: curl -fsSL $site/install.sh | sh -s -- [--proxy <url>] [--mirror <prefix>]" \
        "       curl -fsSL $site/install.sh | sh -s -- --uninstall [--purge]" \
        "  --proxy <url>      http://, https://, socks5:// or socks5h:// proxy for the downloads (or MAGPIE_PROXY=)" \
        "  --mirror <prefix>  GitHub download mirror put before the release's github.com URL (or MAGPIE_MIRROR=);" \
        "                     the SHA-256 is still checked against $site's" \
        "  --uninstall        take off what this script installed; your settings in ~/.config/magpie stay" \
        "  --purge            with --uninstall: also disconnect every agent and delete magpie's settings (magpie uninstall)"
      exit 0 ;;
    *) die "unknown option $1 (--proxy <url>, --mirror <prefix>, --uninstall, --purge)" ;;
  esac
done
case "$proxy" in
  ''|http://?*|https://?*|socks5://?*|socks5h://?*) ;;
  *) die "--proxy is an address like http://127.0.0.1:7890 or socks5://127.0.0.1:1080, not $proxy" ;;
esac
case "$mirror" in
  '') ;;
  http://?*|https://?*) case "$mirror" in */) ;; *) mirror="$mirror/" ;; esac ;;
  *) die "--mirror is an http(s) address put before the github.com URL, like https://mirror.example/, not $mirror" ;;
esac

# get: curl, through the proxy when one was given
get() {
  if [ -n "$proxy" ]; then curl -fsSL --proxy "$proxy" "$@"; else curl -fsSL "$@"; fi
}

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux)
    termux="${TERMUX_VERSION:-}"
    case "${PREFIX:-}" in */com.termux/files/usr) termux=1 ;; esac
    if [ -n "$termux" ]; then
      [ -n "${PREFIX:-}" ] || die "Termux requires PREFIX to locate its bin directory"
      os=android
      bin="${MAGPIE_BIN_DIR:-$PREFIX/bin}"
    else
      os=linux
    fi ;;
  *) die "unsupported system: $(uname -s); see $site" ;;
esac
share="${XDG_DATA_HOME:-$HOME/.local/share}"
config="${XDG_CONFIG_HOME:-$HOME/.config}"

# runs <file> <key>: the program an entry's Exec= or a unit's ExecStart=
# starts, its first word, quoted or not
runs() {
  sed -n "s/^$2=//p" "$1" 2>/dev/null | head -n 1 | sed -e 's/^"\([^"]*\)".*/\1/' -e t -e 's/ .*//'
}

# --uninstall: what the install below puts there, and nothing of the
# user's. The service and login entry go too while they start this
# magpie: left, they would start a program that is gone at each login.
if [ -n "$uninstall" ]; then
  exe="$bin/magpie"
  if [ "$uninstall" = purge ]; then
    # magpie disconnects the agents it connected and deletes its own
    # folders; deleting them here by hand would leave the agents pointed
    # at a gateway that is gone
    [ -x "$exe" ] || die "no magpie at $exe to disconnect your agents and delete its settings: install it again and run this again, or run --uninstall alone to keep them"
    "$exe" uninstall --yes || die "magpie uninstall didn't finish (see above), so the magpie command and app are left: run this again once it does"
  fi
  removed=
  gone() { rm -rf "$1"; removed=1; say "removed $1"; }
  if [ "$os" = darwin ]; then
    for apps in /Applications "$HOME/Applications"; do
      [ ! -d "$apps/magpie.app" ] || gone "$apps/magpie.app"
    done
    case "$(readlink "$exe" 2>/dev/null)" in
      */magpie.app/Contents/MacOS/magpie) gone "$exe" ;;
    esac
  else
    if [ "$os" = linux ]; then
      unit="$config/systemd/user/magpie.service"
      if [ "$(runs "$unit" ExecStart)" = "$exe" ]; then
        if ! "$exe" daemon uninstall >/dev/null 2>&1; then
          command -v systemctl >/dev/null && systemctl --user disable --now magpie.service >/dev/null 2>&1 || true
          rm -f "$unit"
        fi
        removed=1; say "removed magpie's service ($unit)"
      fi
      login="$config/autostart/magpie.desktop"
      [ "$(runs "$login" Exec)" != "$exe" ] || gone "$login"
    fi
    [ ! -f "$exe" ] || gone "$exe"
    if [ "$os" = linux ]; then
      apps="$share/applications"
      # the menu entry and its icon while they start this magpie, or one
      # that is gone; another magpie's stays
      run=$(runs "$apps/magpie.desktop" Exec)
      if [ -f "$apps/magpie.desktop" ] && { [ "$run" = "$exe" ] || [ ! -e "$run" ]; }; then
        gone "$apps/magpie.desktop"
      fi
      # with no menu entry (this one, or the one magpie uninstall took),
      # what came with it: the icon, the magpie:// default xdg-mime set
      # (that line only) and the menu's cache when it lists nothing else
      if [ ! -e "$apps/magpie.desktop" ]; then
        icons="$share/icons/hicolor/256x256/apps"
        [ ! -f "$icons/magpie.png" ] || gone "$icons/magpie.png"
        rmdir "$icons" "$share/icons/hicolor/256x256" "$share/icons/hicolor" "$share/icons" 2>/dev/null || true
        list="$config/mimeapps.list"
        if [ -f "$list" ] && grep -qx 'x-scheme-handler/magpie=magpie\.desktop;\{0,1\}' "$list"; then
          grep -vx 'x-scheme-handler/magpie=magpie\.desktop;\{0,1\}' "$list" > "$list.magpie-new" || true
          mv "$list.magpie-new" "$list"
          # a list left with no entry at all (the one xdg-mime made) goes
          grep -qv -e '^\[.*\]$' -e '^[[:space:]]*$' "$list" || rm -f "$list"
          removed=1; say "removed magpie:// from $list"
        fi
        if [ -d "$apps" ] && ! ls "$apps" | grep -q '\.desktop$'; then
          rm -f "$apps/mimeinfo.cache"
          rmdir "$apps" 2>/dev/null || true
        elif [ -d "$apps" ]; then
          command -v update-desktop-database >/dev/null && update-desktop-database "$apps" 2>/dev/null || true
        fi
      fi
    fi
  fi
  [ "$removed" ] || say "no magpie installed by this script at $exe"
  if [ "$uninstall" != purge ]; then
    say "kept your settings, providers and usage in $config/magpie: install again to have them back, or run with --purge to delete them"
    if pid=$(pgrep -f "^$exe( |\$)" 2>/dev/null) && [ -n "$pid" ]; then
      say "magpie is still running (pid $(printf '%s' "$pid" | tr '\n' ' ' | sed 's/ $//')) and ends when you quit it; agents connected to it reach its gateway until then"
    fi
  fi
  # magpie uninstall said so already
  [ "$uninstall" = purge ] || say "magpie is uninstalled"
  exit 0
fi

case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) die "unsupported CPU: $(uname -m)" ;;
esac
case "$os" in
  darwin) file="magpie-darwin-$arch.zip" ;;
  android) file="magpie-cli-android-$arch" ;;
  *)
    file="magpie-cli-linux-$arch"
    if [ -z "${MAGPIE_CLI:-}" ] && { ldconfig -p 2>/dev/null | grep -q 'libwebkit2gtk-4\.1\.so\.0'; }; then
      file="magpie-linux-$arch"
    fi ;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# the release list and its checksums: always from the site, never a mirror
feed=$(get "$site/api/latest") || die "could not reach $site${proxy:+ through $proxy}"
# The feed is one line of JSON; pull this file's url and hash out of it.
entry=$(printf '%s' "$feed" | sed -n "s/.*\"$file\":{\([^}]*\)}.*/\1/p")
url=$(printf '%s' "$entry" | sed -n 's/.*"url":"\([^"]*\)".*/\1/p')
sum=$(printf '%s' "$entry" | sed -n 's/.*"sha256":"\([^"]*\)".*/\1/p')
version=$(printf '%s' "$feed" | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')
if [ "$os" = android ] && { [ -z "$url" ] || [ -z "$sum" ]; }; then
  die "the newest release has no $file; build in Termux with go build -tags nogui (see $site/docs/start)"
fi
[ -n "$url" ] && [ -n "$sum" ] || die "the newest release has no $file"

from=$url
case "$url" in
  https://github.com/*) [ -z "$mirror" ] || from="$mirror$url" ;;
esac
if [ "$from" != "$url" ]; then say "downloading magpie $version ($file) through $mirror"
else say "downloading magpie $version ($file)"; fi
get -o "$tmp/$file" "$from" || die "could not download $from"
if command -v sha256sum >/dev/null; then got=$(sha256sum "$tmp/$file" | cut -d' ' -f1)
else got=$(shasum -a 256 "$tmp/$file" | cut -d' ' -f1); fi
if [ "$got" != "$sum" ]; then
  [ "$from" = "$url" ] || die "$file from $mirror does not match its checksum from $site; not installed"
  die "$file does not match its checksum"
fi

mkdir -p "$bin"
if [ "$os" = darwin ]; then
  apps=/Applications
  [ -w "$apps" ] || { apps="$HOME/Applications"; mkdir -p "$apps"; }
  ditto -x -k "$tmp/$file" "$tmp/x"
  # the new app beside the old one first: a move failing (a full disk)
  # leaves the old app in place
  rm -rf "$apps/.magpie.app.new"
  mv "$tmp/x/magpie.app" "$apps/.magpie.app.new"
  rm -rf "$apps/magpie.app"
  mv "$apps/.magpie.app.new" "$apps/magpie.app"
  ln -sf "$apps/magpie.app/Contents/MacOS/magpie" "$bin/magpie"
  say "installed $apps/magpie.app"
else
  install -m 755 "$tmp/$file" "$bin/magpie"
  if [ "$file" = "magpie-linux-$arch" ]; then
    mkdir -p "$share/applications" "$share/icons/hicolor/256x256/apps"
    get -o "$share/icons/hicolor/256x256/apps/magpie.png" "$site/img/icon-256.png" || true
    cat > "$share/applications/magpie.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=magpie
Comment=Every agent's model. One place.
Exec=$bin/magpie %u
Icon=magpie
StartupWMClass=magpie
Categories=Development;Utility;
MimeType=x-scheme-handler/magpie;
Terminal=false
EOF
    # magpie://import links open in magpie
    command -v update-desktop-database >/dev/null && update-desktop-database "$share/applications" 2>/dev/null || true
    command -v xdg-mime >/dev/null && xdg-mime default magpie.desktop x-scheme-handler/magpie 2>/dev/null || true
    say "installed the magpie desktop app (menu entry: magpie)"
  elif [ "$os" = android ]; then
    say "installed the Termux build; use magpie web for the browser UI or magpie tui for the terminal UI"
  else
    say "installed the terminal build; for the desktop app, install WebKitGTK 4.1 (libwebkit2gtk-4.1-0) and run this again"
  fi
fi
say "installed $bin/magpie"
case ":$PATH:" in
  *":$bin:"*) ;;
  *) say "add $bin to your PATH to run magpie from a terminal" ;;
esac
case "$file" in
  *.zip) say "open it: open -a magpie" ;;
  magpie-cli-android-*) say "run it: magpie web" ;;
  magpie-linux-*) say "open it from your app menu, or run: magpie app" ;;
  *) say "run it: magpie" ;;
esac
