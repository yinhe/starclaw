# Spore Release Build Script
# Cross-compiles spore runtime + setup installer for all platforms
# Output: dist/ directory with all release artifacts

param(
    [string]$OutDir = "dist",
    [switch]$SkipSporePackages  # Skip rebuilding .spore packages (use existing)
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$SporeDir = Join-Path $Root "spore"
$EmbedDir = Join-Path $SporeDir "cmd\setup\embed"
$Dist = Join-Path $SporeDir $OutDir

# Clean
if (Test-Path $Dist) { Remove-Item -Recurse -Force $Dist }
New-Item -ItemType Directory -Path $Dist -Force | Out-Null

$platforms = @(
    @{ GOOS="windows"; GOARCH="amd64"; Ext=".exe"; Label="windows-amd64"; SetupName="StarClaw-Setup.exe" },
    @{ GOOS="linux";   GOARCH="amd64"; Ext="";     Label="linux-amd64";   SetupName="StarClaw-Setup-linux-amd64" },
    @{ GOOS="darwin";  GOARCH="arm64"; Ext="";     Label="darwin-arm64";  SetupName="StarClaw-Setup-darwin-arm64" },
    @{ GOOS="darwin";  GOARCH="amd64"; Ext="";     Label="darwin-amd64";  SetupName="StarClaw-Setup-darwin-amd64" }
)

foreach ($p in $platforms) {
    $label = $p.Label
    Write-Host "`n=== Building $label ===" -ForegroundColor Cyan

    # 1. Build spore runtime for this platform
    $sporeBin = Join-Path $Dist "spore-$label$($p.Ext)"
    Write-Host "  [1/3] Compiling spore runtime..."
    $env:GOOS = $p.GOOS; $env:GOARCH = $p.GOARCH; $env:CGO_ENABLED = "0"
    go build -ldflags="-s -w" -o $sporeBin ./cmd/spore
    if ($LASTEXITCODE -ne 0) { throw "Failed to build spore for $label" }
    Write-Host "  -> $sporeBin ($('{0:N1}' -f ((Get-Item $sporeBin).Length / 1MB)) MB)"

    # 2. Copy spore binary into setup embed directory
    Write-Host "  [2/3] Embedding spore runtime..."
    Copy-Item -Force $sporeBin (Join-Path $EmbedDir "spore_bin")

    # 3. Build setup installer for this platform
    $setupBin = Join-Path $Dist $p.SetupName
    Write-Host "  [3/3] Compiling setup installer..."
    go build -ldflags="-s -w" -o $setupBin ./cmd/setup
    if ($LASTEXITCODE -ne 0) { throw "Failed to build setup for $label" }
    Write-Host "  -> $setupBin ($('{0:N1}' -f ((Get-Item $setupBin).Length / 1MB)) MB)"
}

# Reset env
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

# Copy static assets
Copy-Item (Join-Path $EmbedDir "icon.ico") $Dist
Copy-Item (Join-Path $EmbedDir "claw.spore") $Dist

# Copy install scripts
if (Test-Path (Join-Path $SporeDir "install.sh")) { Copy-Item (Join-Path $SporeDir "install.sh") $Dist }
if (Test-Path (Join-Path $SporeDir "install.ps1")) { Copy-Item (Join-Path $SporeDir "install.ps1") $Dist }
if (Test-Path (Join-Path $SporeDir "scripts\quick-install.ps1")) { Copy-Item (Join-Path $SporeDir "scripts\quick-install.ps1") $Dist }

Write-Host "`n=== Build Complete ===" -ForegroundColor Green
Get-ChildItem $Dist | Format-Table Name, @{N="Size(MB)";E={'{0:N1}' -f ($_.Length / 1MB)}} -AutoSize
