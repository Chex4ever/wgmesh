#!/usr/bin/env bash
# install.sh — собрать wgmesh из исходников и установить в $PREFIX/bin.
# Использование: ./scripts/install.sh [префикс, по умолчанию /usr/local]
set -euo pipefail

cd "$(dirname "$0")/.."
PREFIX="${1:-/usr/local}"
BIN="$PREFIX/bin/wgmesh"

if ! command -v go >/dev/null 2>&1; then
    echo "Ошибка: не найден go (установите Go >= 1.19: https://go.dev/dl)" >&2
    exit 1
fi

VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"
echo "Сборка wgmesh $VERSION -> $BIN"

TMP="$(mktemp /tmp/wgmesh.XXXXXX)"
go build -trimpath \
    -ldflags "-s -w -X github.com/wgmesh/wgmesh/internal/cli.Version=$VERSION" \
    -o "$TMP" ./cmd/wgmesh

mkdir -p "$PREFIX/bin" 2>/dev/null || true

if mv -f "$TMP" "$BIN" 2>/dev/null; then
    chmod 0755 "$BIN"
else
    echo "Недостаточно прав для $BIN — повторяю с sudo"
    sudo install -m 0755 "$TMP" "$BIN"
    rm -f "$TMP"
fi

echo "✔ Установлено: $BIN"
"$BIN" version

