# Spore Release Build Script
# Cross-compiles spore runtime + setup installer for all platforms
# Builds platform-specific claw.spore packages (cross-compiles Claw API)
# Outputs: .exe (Windows), .tar.gz (Linux), .dmg placeholder (macOS - final DMG on server)
# Version synced from git tag (same as Docker/Nydus releases)

param(
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$SporeDir = Join-Path $Root "spore"
$ClawDir = Join-Path (Join-Path $Root "claw") "api"
$EmbedDir = Join-Path $SporeDir "cmd\setup\embed"
$Dist = Join-Path $SporeDir $OutDir

# ── Resolve version from git tag (syncs with Docker releases) ──
$gitVersion = $null
try {
    $tagOut = & git -C $Root describe --tags --abbrev=0 2>&1
    if ($LASTEXITCODE -eq 0 -and $tagOut) {
        $gitVersion = ($tagOut | Out-String).Trim() -replace '^v', ''
    }
} catch {}
if (-not $gitVersion) {
    try {
        $resp = Invoke-RestMethod -Uri 'https://nydus.starclaw.net/releases/latest' -TimeoutSec 5
        $gitVersion = $resp.tag_name -replace '^v', ''
    } catch {}
}
if (-not $gitVersion) {
    $gitVersion = Get-Date -Format 'yyyy.MMdd.HHmm'
}
$vTag = "v$gitVersion"
Write-Host "Version: $vTag" -ForegroundColor Yellow

# Write .version file for Dockerfile builds (belt-and-suspenders with ldflags)
$versionFile = Join-Path $ClawDir ".version"
[IO.File]::WriteAllText($versionFile, $gitVersion)
Write-Host "  Wrote api/.version = $gitVersion"

# Clean
if (Test-Path $Dist) { Remove-Item -Recurse -Force $Dist }
New-Item -ItemType Directory -Path $Dist -Force | Out-Null

$platforms = @(
    @{ GOOS="windows"; GOARCH="amd64"; Ext=".exe"; Label="windows-amd64" },
    @{ GOOS="linux";   GOARCH="amd64"; Ext="";     Label="linux-amd64" },
    @{ GOOS="darwin";  GOARCH="arm64"; Ext="";     Label="darwin-arm64" },
    @{ GOOS="darwin";  GOARCH="amd64"; Ext="";     Label="darwin-amd64" }
)

foreach ($p in $platforms) {
    $label = $p.Label
    Write-Host "`n=== Building $label ===" -ForegroundColor Cyan
    $env:GOOS = $p.GOOS; $env:GOARCH = $p.GOARCH; $env:CGO_ENABLED = "0"

    # 1. Build spore runtime
    $sporeBin = Join-Path $Dist "spore-$label$($p.Ext)"
    Write-Host "  [1/5] Compiling spore runtime..."
    Push-Location $SporeDir
    go build -ldflags="-s -w -X main.version=$gitVersion" -o $sporeBin ./cmd/spore
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Failed to build spore for $label" }
    Pop-Location
    $sz = '{0:N1}' -f ((Get-Item $sporeBin).Length / 1MB)
    Write-Host "  -> spore ($sz MB)"

    # 2. Cross-compile Claw API (with version stamp matching Docker builds)
    # Sync web dist into go:embed directory so the binary serves the latest frontend
    $webSrc = Join-Path (Join-Path $Root "claw") "web\dist"
    $webEmbed = Join-Path $ClawDir "internal\web\dist"
    if (Test-Path $webSrc) {
        if (Test-Path $webEmbed) { Remove-Item -Recurse -Force $webEmbed }
        Copy-Item -Recurse $webSrc $webEmbed
        Write-Host "  [2/5] Synced web dist -> api/internal/web/dist"
    }
    $clawBinName = "claw-api$($p.Ext)"
    $clawBin = Join-Path $Dist "claw-api-$label$($p.Ext)"
    Write-Host "  [2/5] Cross-compiling Claw API..."
    Push-Location $ClawDir
    go build -ldflags="-s -w -X github.com/yinhe/starclaw/internal/molt.Version=$gitVersion" -o $clawBin ./cmd/server
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Failed to build claw for $label" }
    Pop-Location
    $sz = '{0:N1}' -f ((Get-Item $clawBin).Length / 1MB)
    Write-Host "  -> claw-api ($sz MB)"

    # 2b. Cross-compile MCP Bridge
    $bridgeName = "mcp-bridge-$($p.GOOS)-$($p.GOARCH)$($p.Ext)"
    $bridgeBin = Join-Path $Dist $bridgeName
    Write-Host "  [2b/5] Cross-compiling MCP Bridge..."
    Push-Location $ClawDir
    go build -ldflags="-s -w -X main.version=$gitVersion" -o $bridgeBin ./cmd/mcp-bridge
    if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Host "  WARN: MCP Bridge build failed, skipping" }
    Pop-Location
    if (Test-Path $bridgeBin) {
        $sz = '{0:N1}' -f ((Get-Item $bridgeBin).Length / 1MB)
        Write-Host "  -> mcp-bridge ($sz MB)"
    }

    # 3. Create platform-specific claw.spore
    Write-Host "  [3/5] Creating claw.spore [$($p.GOOS)/$($p.GOARCH)]..."
    $pkgDir = Join-Path $Dist "_claw-pkg-$label"
    New-Item -ItemType Directory -Path $pkgDir -Force | Out-Null
    Copy-Item $clawBin (Join-Path $pkgDir $clawBinName)

    # Include MCP Bridge binary in claw.spore
    if (Test-Path $bridgeBin) {
        $bridgeDir = Join-Path $pkgDir "mcp-bridge"
        New-Item -ItemType Directory -Path $bridgeDir -Force | Out-Null
        Copy-Item $bridgeBin (Join-Path $bridgeDir $bridgeName)
    }

    $manifest = @{
        name = "claw"; version = $gitVersion; description = "StarClaw AI Agent Node"
        platform = @{ os = $p.GOOS; arch = $p.GOARCH }; binary = $clawBinName; args = @()
        resources = @{ min_memory_mb = 256; min_disk_mb = 100; recommended_memory_mb = 1024 }
        network = @{ ports = @(@{ port = 80; protocol = "tcp"; description = "HTTP" }) }
        health = @{ endpoint = "http://localhost:80/health"; interval_seconds = 30; timeout_seconds = 5 }
        update = @{ channel = "stable"; auto_update = $false; delta_enabled = $true }
        built_at = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"); built_by = "build-release/$gitVersion"
    } | ConvertTo-Json -Depth 4
    [IO.File]::WriteAllText((Join-Path $pkgDir "manifest.json"), $manifest)

    $webDist = Join-Path (Join-Path (Join-Path $Root "claw") "web") "dist"
    if (Test-Path $webDist) { Copy-Item -Recurse $webDist (Join-Path $pkgDir "web") }

    $clawSpore = Join-Path $EmbedDir "claw.spore"
    tar -czf $clawSpore -C $pkgDir .
    $sz = '{0:N1}' -f ((Get-Item $clawSpore).Length / 1MB)
    Write-Host "  -> claw.spore ($sz MB)"

    # 4. Embed into setup
    Write-Host "  [4/5] Embedding..."
    Copy-Item -Force $sporeBin (Join-Path $EmbedDir "spore_bin")

    # 5. Build setup — raw binary name (will be renamed below)
    $rawSetup = Join-Path $Dist "setup-raw-$label$($p.Ext)"
    Write-Host "  [5/5] Compiling setup installer..."
    Push-Location $SporeDir
    go build -ldflags="-s -w -X main.version=$gitVersion" -o $rawSetup ./cmd/setup
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "Failed to build setup for $label" }
    Pop-Location
    $sz = '{0:N1}' -f ((Get-Item $rawSetup).Length / 1MB)
    Write-Host "  -> setup ($sz MB)"

    Remove-Item -Recurse -Force $pkgDir
}

# Reset env
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue

# ── Package into final release artifacts with version in filename ──
Write-Host "`n=== Packaging releases ===" -ForegroundColor Cyan

# Windows: .exe with version
$winRaw = Join-Path $Dist "setup-raw-windows-amd64.exe"
$winFinal = Join-Path $Dist "StarClaw-Setup-$vTag.exe"
Move-Item $winRaw $winFinal
Write-Host "  -> $( Split-Path -Leaf $winFinal )"

# Linux: tar.gz with install script
$linRaw = Join-Path $Dist "setup-raw-linux-amd64"
$linDir = Join-Path $Dist "_linux-pkg"
New-Item -ItemType Directory -Path $linDir -Force | Out-Null
Copy-Item $linRaw (Join-Path $linDir "StarClaw-Setup")
# Create install.sh wrapper
$installSh = @"
#!/bin/bash
cd "`$(dirname "`$0")"
chmod +x StarClaw-Setup
./StarClaw-Setup "`$@"
"@
[IO.File]::WriteAllText((Join-Path $linDir "install.sh"), $installSh.Replace("`r`n","`n"))
$linFinal = Join-Path $Dist "StarClaw-Setup-$vTag-linux-amd64.tar.gz"
tar -czf $linFinal -C $linDir .
Remove-Item -Recurse -Force $linDir
Remove-Item $linRaw
Write-Host "  -> $( Split-Path -Leaf $linFinal )"

# macOS: raw binaries (DMG created on server with genisoimage)
foreach ($arch in @("darwin-arm64", "darwin-amd64")) {
    $raw = Join-Path $Dist "setup-raw-$arch"
    $final = Join-Path $Dist "StarClaw-Setup-$vTag-$arch"
    Move-Item $raw $final
    Write-Host "  -> $( Split-Path -Leaf $final ) (DMG created on server)"
}

# Copy static assets
Copy-Item (Join-Path $EmbedDir "icon.ico") $Dist -ErrorAction SilentlyContinue

# Copy install scripts
foreach ($s in @("install.sh", "install.ps1")) {
    $sp = Join-Path $SporeDir $s
    if (Test-Path $sp) { Copy-Item $sp $Dist }
}
$qp = Join-Path (Join-Path $SporeDir "scripts") "quick-install.ps1"
if (Test-Path $qp) { Copy-Item $qp $Dist }

Write-Host "`n=== Build Complete ($vTag) ===" -ForegroundColor Green
Get-ChildItem $Dist -Exclude "_*","claw-api-*","spore-*" | Format-Table Name, @{N="Size(MB)";E={'{0:N1}' -f ($_.Length / 1MB)}} -AutoSize
