#!/usr/bin/env sh
set -eu

module="github.com/wailsapp/wails/v2"
version=$(go list -m -f '{{.Version}}' "$module")

case "$version" in
	v2.*)
		;;
	*)
		echo "Unsupported Wails module version: $version" >&2
		exit 1
		;;
esac

echo "Installing Wails CLI $version"
go install "$module/cmd/wails@$version"
wails version
