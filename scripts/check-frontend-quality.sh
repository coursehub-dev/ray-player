#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
frontend_dir="$repo_root/frontend"

if ! command -v npm >/dev/null 2>&1; then
	echo "npm is required for frontend quality checks." >&2
	exit 1
fi

if [ ! -d "$frontend_dir/node_modules/@biomejs/biome" ]; then
	echo "Frontend dependencies are not installed." >&2
	echo "Run: npm ci --prefix frontend --strict-peer-deps" >&2
	exit 1
fi

exec npm --prefix "$frontend_dir" run check
