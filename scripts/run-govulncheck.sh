#!/usr/bin/env sh
set -eu

govulncheck_version="v1.6.0"

exec go run "golang.org/x/vuln/cmd/govulncheck@${govulncheck_version}" -show verbose ./...
