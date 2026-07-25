#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
dist_dir="$repo_root/frontend/dist"
index_file="$dist_dir/index.html"

if [ ! -s "$index_file" ]; then
	echo "Embedded frontend artifact is missing or empty: frontend/dist/index.html" >&2
	echo "Run: npm --prefix frontend run build" >&2
	exit 1
fi

if ! find "$dist_dir/assets" -type f -print -quit 2>/dev/null | grep -q .; then
	echo "Embedded frontend asset bundle is missing: frontend/dist/assets" >&2
	echo "Run: npm --prefix frontend run build" >&2
	exit 1
fi

exit 0
