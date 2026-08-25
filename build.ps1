<#
.SYNOPSIS
    Cross-compiles Graphite's example programs, or runs a check target.

.DESCRIPTION
    Mirrors the Makefile in this repo for machines without `make` installed.
    Cross-compiled binaries land in dist/<goos>_<goarch>/<name>[.exe].

.PARAMETER Target
    One of: All (default), windows-amd64, windows-386, linux-amd64, linux-386,
    darwin-amd64, darwin-arm64, Build, Test, Vet, Fmt, Lint, Tidy, Clean.

.EXAMPLE
    ./build.ps1
    ./build.ps1 -Target linux-amd64
    ./build.ps1 -Target darwin-arm64
    ./build.ps1 -Target Test
#>
param(
    [ValidateSet(
        "All", "windows-amd64", "windows-386", "linux-amd64", "linux-386",
        "darwin-amd64", "darwin-arm64",
        "Build", "Test", "Vet", "Fmt", "Lint", "Tidy", "Clean"
    )]
    [string]$Target = "All"
)

$ErrorActionPreference = "Stop"

$Dist = "dist"
# name => package path, one entry per `package main` program in this repo.
$Programs = @{ "gphedit" = "./gphedit"; "showcase" = "./showcase"; "promo" = "./promo" }

function Build-Target {
    param([string]$Goos, [string]$Goarch, [string]$Ext)

    foreach ($name in $Programs.Keys) {
        $path = $Programs[$name]
        $out = Join-Path (Join-Path $Dist "${Goos}_${Goarch}") "$name$Ext"
        Write-Output "building $out"

        $env:CGO_ENABLED = "0"
        $env:GOOS = $Goos
        $env:GOARCH = $Goarch
        try {
            go build -o $out $path
            if ($LASTEXITCODE -ne 0) { throw "go build failed for $path ($Goos/$Goarch)" }
        } finally {
            Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
        }
    }
}

switch ($Target) {
    "All" {
        Build-Target -Goos "windows" -Goarch "amd64" -Ext ".exe"
        Build-Target -Goos "windows" -Goarch "386" -Ext ".exe"
        Build-Target -Goos "linux" -Goarch "amd64" -Ext ""
        Build-Target -Goos "linux" -Goarch "386" -Ext ""
        Build-Target -Goos "darwin" -Goarch "amd64" -Ext ""
        Build-Target -Goos "darwin" -Goarch "arm64" -Ext ""
    }
    "windows-amd64" { Build-Target -Goos "windows" -Goarch "amd64" -Ext ".exe" }
    "windows-386"   { Build-Target -Goos "windows" -Goarch "386" -Ext ".exe" }
    "linux-amd64"   { Build-Target -Goos "linux" -Goarch "amd64" -Ext "" }
    "linux-386"     { Build-Target -Goos "linux" -Goarch "386" -Ext "" }
    "darwin-amd64"  { Build-Target -Goos "darwin" -Goarch "amd64" -Ext "" }
    "darwin-arm64"  { Build-Target -Goos "darwin" -Goarch "arm64" -Ext "" }
    "Build" {
        go build ./...
    }
    "Test" {
        go test ./...
    }
    "Vet" {
        go vet ./...
    }
    "Fmt" {
        $unformatted = gofmt -l .
        if ($unformatted) {
            Write-Output "gofmt needed on:"
            Write-Output $unformatted
            exit 1
        }
    }
    "Lint" {
        $lintCmd = Get-Command golangci-lint -ErrorAction SilentlyContinue
        if ($lintCmd) {
            & golangci-lint run ./...
        } else {
            Write-Output "golangci-lint not installed, skipping (see https://golangci-lint.run)"
        }
    }
    "Tidy" {
        go mod tidy
    }
    "Clean" {
        Remove-Item -Recurse -Force $Dist -ErrorAction SilentlyContinue
    }
}
