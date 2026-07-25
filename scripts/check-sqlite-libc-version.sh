#!/usr/bin/env sh
set -eu

sqlite_module="modernc.org/sqlite"
libc_module="modernc.org/libc"

sqlite_version=$(go list -m -f '{{.Version}}' "$sqlite_module")
selected_libc=$(go list -m -f '{{.Version}}' "$libc_module")
required_libc=$(
	go mod graph |
		awk -v parent="$sqlite_module@$sqlite_version" -v child="$libc_module@" '
			$1 == parent && index($2, child) == 1 {
				dependency = $2
				sub(/^[^@]+@/, "", dependency)
				print dependency
				exit
			}
		'
)

if [ -z "$required_libc" ]; then
	echo "Could not resolve $libc_module required by $sqlite_module@$sqlite_version" >&2
	exit 1
fi

if [ "$selected_libc" != "$required_libc" ]; then
	echo "$sqlite_module@$sqlite_version requires $libc_module@$required_libc, selected $selected_libc" >&2
	exit 1
fi

echo "SQLite/libc versions are aligned: $sqlite_module@$sqlite_version -> $libc_module@$selected_libc"
