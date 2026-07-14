#!/usr/bin/env sh
set -eu

REPO_URL="${REDFETCH_REPO:-https://github.com/TechnoRed2026/redfetch.git}"
INSTALL_ROOT="${REDFETCH_HOME:-$HOME/.local/share/redfetch}"
SRC_DIR="$INSTALL_ROOT/src"
BIN_DIR="${REDFETCH_BIN:-$HOME/.local/bin}"

say() { printf '\033[31;1mredfetch ->\033[0m %s\n' "$*" >&2; }
need() { command -v "$1" >/dev/null 2>&1; }

fetch() {
    if need curl; then curl -fsSL "$1" -o "$2"
    elif need wget; then wget -qO "$2" "$1"
    else echo "redfetch -> curl or wget required" >&2; exit 1
    fi
}

fetch_text() {
    if need curl; then curl -fsSL "$1"
    elif need wget; then wget -qO - "$1"
    else echo "redfetch -> curl or wget required" >&2; exit 1
    fi
}

sync_repo() {
    mkdir -p "$INSTALL_ROOT"
    if need git; then
        if [ -d "$SRC_DIR/.git" ]; then
            say "update repo"
            git -C "$SRC_DIR" pull --ff-only
        else
            rm -rf "$SRC_DIR"
            say "clone repo"
            git clone --depth 1 "$REPO_URL" "$SRC_DIR"
        fi
        return
    fi

    say "git not found; using GitHub tarball"
    tgz="$INSTALL_ROOT/redfetch-main.tar.gz"
    work="$INSTALL_ROOT/tar"
    fetch "https://github.com/TechnoRed2026/redfetch/archive/refs/heads/main.tar.gz" "$tgz"
    rm -rf "$work" "$SRC_DIR"
    mkdir -p "$work"
    tar -C "$work" -xzf "$tgz"
    found=$(find "$work" -mindepth 1 -maxdepth 1 -type d | sed -n '1p')
    [ -n "$found" ] || { echo "redfetch -> repo unpack failed" >&2; exit 1; }
    mv "$found" "$SRC_DIR"
}

ensure_go() {
    if need go; then command -v go; return; fi

    say "Go not found; downloading portable Go"
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in linux|darwin) ;; *) echo "redfetch -> unsupported OS for auto Go install: $os" >&2; exit 1 ;; esac

    machine=$(uname -m)
    case "$machine" in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) echo "redfetch -> unsupported arch for auto Go install: $machine" >&2; exit 1 ;;
    esac

    version=$(fetch_text "https://go.dev/VERSION?m=text" | sed -n '1p')
    case "$version" in go*) ;; *) echo "redfetch -> cannot detect latest Go version" >&2; exit 1 ;; esac

    tarball="$version.$os-$arch.tar.gz"
    tgz="$INSTALL_ROOT/$tarball"
    fetch "https://go.dev/dl/$tarball" "$tgz"
    rm -rf "$INSTALL_ROOT/go"
    tar -C "$INSTALL_ROOT" -xzf "$tgz"
    printf '%s\n' "$INSTALL_ROOT/go/bin/go"
}

add_path() {
    mkdir -p "$BIN_DIR"
    case ":$PATH:" in *":$BIN_DIR:"*) return ;; esac
    rc="$HOME/.profile"
    touch "$rc"
    if ! grep -F "$BIN_DIR" "$rc" >/dev/null 2>&1; then
        printf '\n# redfetch\nexport PATH="$PATH:%s"\n' "$BIN_DIR" >> "$rc"
        say "added to PATH in $rc"
    fi
    PATH="$BIN_DIR:$PATH"
    export PATH
}

sync_repo
GO_BIN=$(ensure_go)
add_path

say "build"
(cd "$SRC_DIR" && "$GO_BIN" build -trimpath -ldflags="-s -w" -o "$BIN_DIR/redfetch" .)
chmod +x "$BIN_DIR/redfetch"

say "installed: $BIN_DIR/redfetch"
printf '\n'
"$BIN_DIR/redfetch"
printf '\nOpen a new terminal, then run: redfetch\n'
