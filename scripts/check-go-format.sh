#!/usr/bin/env sh
set -eu

files=$(git ls-files '*.go')
[ -z "$files" ] && exit 0

unformatted=$(printf '%s\n' "$files" | xargs gofmt -l)
if [ -n "$unformatted" ]; then
  echo "Go files are not gofmt-formatted:" >&2
  printf '%s\n' "$unformatted" >&2
  echo "Run: just format" >&2
  exit 1
fi
