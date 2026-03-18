# Spore Release Build Script
# Cross-compiles spore runtime + setup installer for all platforms
# Builds platform-specific claw.spore packages (cross-compiles Claw API)
# Version synced from git tag (same as Docker/Nydus releases)

param(
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$SporeDir = Join-Path $Root "spore"
$ClawDir = Join-Path $Root "claw" "api"
$EmbedDir = Join-Path $SporeDir "cmd\setup\embed"
$Dist = Join-Path $SporeDir $OutDir

# ── Resolve version from git tag (syncs with Docker releases) ──
try {
    $gitVersion = (git -C $Root describe --tags --abbrev=0 2>$null)
    if ($gitVersion) {
        $gitVersion = $gitVersion -replace '^v', ''
    } else {
        $gitVersion = (Invoke-RestMethod -Uri 'https://nydus.starclaw.net/releases/latest' -TimeoutSec 5).tag_name -replace '^v', ''
    }
} catch {
    $gitVersion = $null
}
if (-not $gitVersion) {
    $gitVersion = Get-Date -Format 'yyyy.MMdd.HHmm'
}
Write-Host "Version: $gitVersion" -ForegroundColor Yellow

# Patch version const in setup/main.go
$setupMain = Join-Path $SporeDir "cmd\setup\main.go"
$content = Get-Content $setupMain -Raw
$content = $content -replace 'const version = "[^"]+"', "const version = `"$gitVersion`""
$content | Set-Content $setupMain -NoNewline

# Patch version const in spore/main.go
$sporeMain = Join-Path $SporeDir "cmd\spore\main.go"
$content2 = Get-Content $sporeMain -Raw
$content2 = $content2 -replace 'const version = "[^"]+"', "const version = `"$gitVersion`""
$content2 | Set-Content $sporeMain -NoNewline

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
    $env:GOOS = $p.GOOS; $env:GOARCH = $p.GOARCH; $env:CGO_ENABLED = "0"

    # 1. Build spore runtime for this platform
    $sporeBin = Join-Path $Dist "spore-$label$($p.Ext)"
    Write-Host "  [1/5] Compiling spore runtime..."
    go build -ldflags="-s -w" -o $sporeBin ./cmd/spore
    if ($LASTEXITCODE -ne 0) { throw "Failed to build spore for $label" }
    Write-Host "  -> $sporeBin ($('{0:N1}' -f ((Get-Item $sporeBin).Length / 1MB)) MB)"

    # 2. Cross-compile Claw API binary for this platform
    $clawBinName = "claw-api$($p.Ext)"
    $clawBin = Join-Path $Dist "claw-api-$label$($p.Ext)"
    Write-Host "  [2/5] Cross-compiling Claw API..."
    Push-Location $ClawDir
    go build -ldflags="-s -w" -o $clawBin ./cmd/server
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Failed to build claw for $label" }
    Pop-Location
    Write-Host "  -> $clawBin ($('{0:N1}' -f ((Get-Item $clawBin).Length / 1MB)) MB)"

    # 3. Create platform-specific claw.spore package
    Write-Host "  [3/5] Creating claw.spore package..."
    $pkgDir = Join-Path $Dist "_claw-pkg-$label"
    New-Item -ItemType Directory -Path $pkgDir -Force | Out-Null
    Copy-Item $clawBin (Join-Path $pkgDir $clawBinName)

    # Create manifest.json
    $manifest = @{
        name = "claw"
        version = $gitVersion
        description = "StarClaw AI Agent Node"
        platform = @{ os = $p.GOOS; arch = $p.GOARCH }
        binary = $clawBinName
        args = @()
        resources = @{ min_memory_mb = 256; min_disk_mb = 100; recommended_memory_mb = 1024 }
        network = @{ ports = @(@{ port = 80; protocol = "tcp"; description = "HTTP" }) }
        health = @{ endpoint = "http://localhost:80/health"; interval_seconds = 30; timeout_seconds = 5 }
        update = @{ channel = "stable"; auto_update = $false; delta_enabled = $true }
        built_at = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
        built_by = "build-release/$gitVersion"
    } | ConvertTo-Json -Depth 4
    $manifest | Set-Content (Join-Path $pkgDir "manifest.json") -Encoding UTF8

    # Also copy web assets if they exist
    $webDist = Join-Path $Root "claw" "web" "dist"
    if (Test-Path $webDist) {
        Copy-Item -Recurse $webDist (Join-Path $pkgDir "web")
    }

    # Pack into .spore (tar.gz)
    $clawSpore = Join-Path $EmbedDir "claw.spore"
    tar -czf $clawSpore -C $pkgDir .
    Write-Host "  -> claw.spore ($('{0:N1}' -f ((Get-Item $clawSpore).Length / 1MB)) MB) [$($p.GOOS)/$($p.GOARCH)]"

    # 4. Copy spore binary into setup embed directory
    Write-Host "  [4/5] Embedding spore runtime + claw.spore..."
    Copy-Item -Force $sporeBin (Join-Path $EmbedDir "spore_bin")

    # 5. Build setup installer for this platform
    $setupBin = Join-Path $Dist $p.SetupName
    Write-Host "  [5/5] Compiling setup installer..."
    Push-Location $SporeDir
    go build -ldflags="-s -w" -o $setupBin ./cmd/setup
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Failed to build setup for $label" }
    Pop-Location
    Write-Host "  -> $setupBin ($('{0:N1}' -f ((Get-Item $setupBin).Length / 1MB)) MB)"

    # Cleanup temp package dir
    Remove-Item -Recurse -Force $pkgDir
}

# Reset env
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

# Copy static assets
Copy-Item (Join-Path $EmbedDir "icon.ico") $Dist

# Copy install scripts
if (Test-Path (Join-Path $SporeDir "install.sh")) { Copy-Item (Join-Path $SporeDir "install.sh") $Dist }
if (Test-Path (Join-Path $SporeDir "install.ps1")) { Copy-Item (Join-Path $SporeDir "install.ps1") $Dist }
if (Test-Path (Join-Path $SporeDir "scripts\quick-install.ps1")) { Copy-Item (Join-Path $SporeDir "scripts\quick-install.ps1") $Dist }

Write-Host "`n=== Build Complete (v$gitVersion) ===" -ForegroundColor Green
Get-ChildItem $Dist -Exclude "_*","claw-api-*" | Format-Table Name, @{N="Size(MB)";E={'{0:N1}' -f ($_.Length / 1MB)}} -AutoSize
