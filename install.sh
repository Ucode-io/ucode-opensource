#!/bin/sh
# Installs the ucode CLI.
#
#   curl -fsSL https://raw.githubusercontent.com/Ucode-io/ucode-opensource/main/install.sh | sh
#
# This downloads one small binary. The platform itself — four services, the
# admin panel, Postgres, Redis, MinIO — arrives as container images the first
# time you run `ucode start`, so Docker is the only prerequisite.

set -eu

REPO="Ucode-io/ucode-opensource"
BASE="https://github.com/${REPO}/releases/latest/download"

die() { printf 'ucode: %s\n' "$1" >&2; exit 1; }
note() { printf '%s\n' "$1" >&2; }

# ---- what machine is this -------------------------------------------------

os=$(uname -s)
arch=$(uname -m)

case "$os" in
    Darwin) os=darwin ;;
    Linux)  os=linux ;;
    *) die "unsupported operating system: $os (macOS and Linux only)" ;;
esac

case "$arch" in
    x86_64 | amd64)  arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) die "unsupported architecture: $arch (amd64 and arm64 only)" ;;
esac

archive="ucode_${os}_${arch}.tar.gz"

# ---- where it goes --------------------------------------------------------
#
# /usr/local/bin when it is writable without sudo, otherwise ~/.local/bin,
# which needs no privileges. Asking for sudo in a piped shell script is worse
# than installing somewhere the user owns.

if [ -n "${UCODE_INSTALL_DIR:-}" ]; then
    dest="$UCODE_INSTALL_DIR"
elif [ -w /usr/local/bin ] 2>/dev/null; then
    dest=/usr/local/bin
else
    dest="$HOME/.local/bin"
fi
mkdir -p "$dest" || die "cannot create $dest"
[ -w "$dest" ] || die "$dest is not writable (set UCODE_INSTALL_DIR to choose another)"

# ---- fetch ----------------------------------------------------------------

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar  >/dev/null 2>&1 || die "tar is required"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

note "Downloading ${archive}"
curl -fsSL -o "$tmp/$archive" "$BASE/$archive" \
    || die "download failed — is there a published release yet? $BASE/$archive"

# ---- verify ---------------------------------------------------------------
#
# The archive arrives over TLS from GitHub, but a checksum still catches a
# truncated download and a swapped asset, and it costs one more request.

if curl -fsSL -o "$tmp/checksums.txt" "$BASE/checksums.txt" 2>/dev/null; then
    if command -v sha256sum >/dev/null 2>&1; then
        sum=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
    elif command -v shasum >/dev/null 2>&1; then
        sum=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
    else
        sum=""
        note "No sha256 tool found; skipping checksum verification."
    fi
    if [ -n "$sum" ]; then
        want=$(grep " ${archive}\$" "$tmp/checksums.txt" | cut -d' ' -f1)
        [ -n "$want" ] || die "no checksum listed for $archive"
        [ "$sum" = "$want" ] || die "checksum mismatch for $archive — refusing to install"
        note "Checksum verified."
    fi
else
    note "No checksums.txt published; skipping verification."
fi

# ---- install --------------------------------------------------------------

tar -xzf "$tmp/$archive" -C "$tmp" ucode || die "archive did not contain a ucode binary"
chmod +x "$tmp/ucode"
mv "$tmp/ucode" "$dest/ucode" || die "could not write $dest/ucode"

note ""
note "Installed $("$dest/ucode" --version 2>/dev/null || echo ucode) to $dest/ucode"

# ---- what is still missing ------------------------------------------------

case ":${PATH}:" in
    *":${dest}:"*) ;;
    *)
        note ""
        note "$dest is not on your PATH. Add it:"
        note "    echo 'export PATH=\"$dest:\$PATH\"' >> ~/.zshrc && exec \$SHELL"
        ;;
esac

if ! command -v docker >/dev/null 2>&1; then
    note ""
    note "Docker is not installed. ucode needs it to run the platform:"
    note "    https://docs.docker.com/get-docker/"
elif ! docker info >/dev/null 2>&1; then
    note ""
    note "Docker is installed but not running. Start it, then:"
    note "    ucode start"
else
    note ""
    note "Next:  ucode start"
fi
