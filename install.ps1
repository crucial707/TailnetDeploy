# TailnetDeploy one-liner launcher (Windows)
# Run in PowerShell as Administrator.
#
# Usage:
#   .\install.ps1
#   $env:TS_AUTH_KEY = "tskey-auth-xxxxx"; .\install.ps1
#
# If you host tailnetdeploy.exe as a release, set $ReleaseUrl below and uncomment
# the download block so this script can fetch and run it (one-liner from web).

$ErrorActionPreference = "Stop"
$exeName = "tailnetdeploy.exe"
$localExe = Join-Path $PSScriptRoot $exeName

# Optional: set when publishing releases for irm ... | iex one-liner
# $ReleaseUrl = "https://github.com/your-org/TailnetDeploy/releases/latest/download/tailnetdeploy-windows-amd64.exe"

if (Test-Path $localExe) {
    & $localExe @args
    exit $LASTEXITCODE
}

# Uncomment to download from release when $ReleaseUrl is set:
# $dir = Join-Path $env:TEMP "tailnetdeploy"
# if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
# $exePath = Join-Path $dir $exeName
# Invoke-WebRequest -Uri $ReleaseUrl -OutFile $exePath -UseBasicParsing
# & $exePath @args
# exit $LASTEXITCODE

Write-Host "Build first: go build -o tailnetdeploy.exe ."
Write-Host "Then run: .\tailnetdeploy.exe -authkey=YOUR_KEY"
exit 1
