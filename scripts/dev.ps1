#Requires -Version 5.1
<#
.SYNOPSIS
  Windows helper for `wails dev` with CGO + MinGW (WinLibs) on PATH.

.DESCRIPTION
  Sets CGO_ENABLED / CGO_CFLAGS_ALLOW and prepends mingw64\bin when needed.
  MinGW resolution order:
    1. $env:RAY_MINGW_BIN (explicit mingw64\bin)
    2. gcc already on PATH
    3. WinGet WinLibs package under %LOCALAPPDATA%\Microsoft\WinGet\Packages

.EXAMPLE
  .\scripts\dev.ps1
  just dev-win
#>

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $repoRoot

$env:CGO_ENABLED = "1"
$env:CGO_CFLAGS_ALLOW = "-Xpreprocessor"

function Test-GccInDir([string]$dir) {
	return (Test-Path -LiteralPath (Join-Path $dir "gcc.exe"))
}

function Find-MingwBin {
	if ($env:RAY_MINGW_BIN) {
		$explicit = $env:RAY_MINGW_BIN.TrimEnd("\", "/")
		if (-not (Test-GccInDir $explicit)) {
			throw "RAY_MINGW_BIN is set but gcc.exe not found: $explicit"
		}
		return $explicit
	}

	$existing = Get-Command gcc -ErrorAction SilentlyContinue
	if ($existing) {
		return $null
	}

	$wingetRoot = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages"
	if (-not (Test-Path -LiteralPath $wingetRoot)) {
		return $null
	}

	$packages = Get-ChildItem -LiteralPath $wingetRoot -Directory -ErrorAction SilentlyContinue |
		Where-Object { $_.Name -like "BrechtSanders.WinLibs*" }

	foreach ($pkg in $packages) {
		$bin = Join-Path $pkg.FullName "mingw64\bin"
		if (Test-GccInDir $bin) {
			return $bin
		}
	}

	return $null
}

$mingwBin = Find-MingwBin
if ($mingwBin) {
	$env:Path = "$mingwBin;$env:Path"
	Write-Host "Using MinGW: $mingwBin"
} elseif (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
	Write-Error @"
gcc not found. Install WinLibs (MinGW-w64) via WinGet, or set RAY_MINGW_BIN to your mingw64\bin folder.

  winget install BrechtSanders.WinLibs.POSIX.UCRT
  `$env:RAY_MINGW_BIN = 'C:\path\to\mingw64\bin'
"@
	exit 1
}

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
	Write-Error "wails CLI not found on PATH. Run: just wails-install"
	exit 1
}

Write-Host "CGO_ENABLED=$env:CGO_ENABLED  CGO_CFLAGS_ALLOW=$env:CGO_CFLAGS_ALLOW"
Write-Host "Starting: wails dev $($args -join ' ')"
& wails dev @args
exit $LASTEXITCODE
