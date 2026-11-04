#!/bin/sh
set -eu

data_dir=${CANVAS_BACKEND_DATA_DIR:-/data}
seed=${CANVAS_SEED_BINARY:-/opt/canvas/yingce-backend}
binary=${CANVAS_MANAGED_BINARY_PATH:-$data_dir/system/bin/yingce-backend}
export CANVAS_MANAGED_BINARY_PATH="$binary"
mkdir -p "$data_dir/system" "$(dirname "$binary")"
if [ "$(id -u)" = 0 ]; then
    chown app:app "$data_dir" "$data_dir/system" "$(dirname "$binary")"
fi
if [ ! -e "$binary" ]; then
    if [ -f "$binary.backup" ]; then
        if [ "$(id -u)" = 0 ]; then
            su-exec app "$seed" --restore-program
        else
            "$seed" --restore-program
        fi
    elif [ -e "$data_dir/system/update.json" ]; then
        echo 'Update state exists but both application and backup are missing; restore the program explicitly.' >&2
        exit 1
    fi
fi
if [ ! -e "$binary" ]; then
    stage=$(mktemp "$(dirname "$binary")/.seed.XXXXXX")
    trap 'rm -f "$stage"' EXIT HUP INT TERM
    cp "$seed" "$stage"
    chmod 0755 "$stage"
    mv "$stage" "$binary"
    trap - EXIT HUP INT TERM
fi
test -f "$binary" && test -x "$binary" || { echo "Managed application is not executable: $binary" >&2; exit 1; }
if [ "$(id -u)" = 0 ]; then
    chown app:app "$data_dir" "$data_dir/system" "$(dirname "$binary")" "$binary"
    exec su-exec app "$binary" "$@"
fi
exec "$binary" "$@"
