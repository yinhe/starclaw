# Spore Release Script — Build locally + Upload to Nydus
# Usage: .\spore\scripts\release-spore.ps1
# Prerequisites: Go toolchain, SSH key (~/.ssh/queen_deploy)

param(
    [switch]$SkipBuild,      # Skip build, only upload existing dist/
    [switch]$SkipUpload,     # Skip upload, only build
    [string]$OutDir = "dist"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$SporeDir = Join-Path $Root "spore"
$Dist = Join-Path $SporeDir $OutDir
$SSHKey = Join-Path $env:USERPROFILE ".ssh\queen_deploy"
$NydusHost = "root@43.106.158.26"
$ReleasesPath = "/data/nydus/repos/releases"

Write-Host "`n========================================" -ForegroundColor Magenta
Write-Host "  StarClaw Spore Release Pipeline" -ForegroundColor Magenta
Write-Host "========================================`n" -ForegroundColor Magenta

# ── Step 1: Build ──
if (-not $SkipBuild) {
    Write-Host "[1/3] Building Spore for all platforms..." -ForegroundColor Cyan
    & (Join-Path $PSScriptRoot "build-release.ps1") -OutDir $OutDir
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }
} else {
    Write-Host "[1/3] Skipping build (using existing $Dist)" -ForegroundColor Yellow
}

# Verify dist has release artifacts
$artifacts = Get-ChildItem $Dist -Filter "StarClaw-Setup-*" -ErrorAction SilentlyContinue
if (-not $artifacts -or $artifacts.Count -eq 0) {
    throw "No StarClaw-Setup-* artifacts found in $Dist"
}

# Extract version from artifact name
$vTag = ($artifacts[0].Name -replace '^StarClaw-Setup-', '' -replace '\.exe$', '' -replace '-linux-amd64\.tar\.gz$', '' -replace '-darwin-(arm64|amd64)$', '')
Write-Host "`nVersion: $vTag" -ForegroundColor Yellow
Write-Host "Artifacts:" -ForegroundColor Yellow
$artifacts | ForEach-Object {
    $sz = '{0:N1}' -f ($_.Length / 1MB)
    Write-Host "  $($_.Name)  ($sz MB)"
}

if ($SkipUpload) {
    Write-Host "`n[2/3] Skipping upload" -ForegroundColor Yellow
    Write-Host "[3/3] Skipping manifest" -ForegroundColor Yellow
    Write-Host "`n=== Build Only Complete ($vTag) ===" -ForegroundColor Green
    exit 0
}

# ── Step 2: Upload artifacts via SCP ──
Write-Host "`n[2/3] Uploading to Nydus ($NydusHost)..." -ForegroundColor Cyan

$sshArgs = @("-i", $SSHKey, "-o", "StrictHostKeyChecking=no")

foreach ($f in $artifacts) {
    $remotePath = "$($NydusHost):$ReleasesPath/$($f.Name)"
    $sz = '{0:N1}' -f ($f.Length / 1MB)
    Write-Host "  Uploading $($f.Name) ($sz MB)..."
    & scp @sshArgs $f.FullName $remotePath
    if ($LASTEXITCODE -ne 0) { throw "SCP failed for $($f.Name)" }
}

# Also upload install scripts if present
foreach ($s in @("install.sh", "install.ps1", "quick-install.ps1")) {
    $sp = Join-Path $Dist $s
    if (Test-Path $sp) {
        Write-Host "  Uploading $s..."
        & scp @sshArgs $sp "$($NydusHost):$ReleasesPath/$s"
    }
}

# ── Step 3: Update spore-latest.json on server ──
Write-Host "`n[3/3] Updating spore-latest.json..." -ForegroundColor Cyan

$version = $vTag -replace '^v', ''
$assetsJson = ($artifacts | ForEach-Object {
    $platform = "windows-amd64"
    if ($_.Name -match 'linux-amd64') { $platform = "linux-amd64" }
    elseif ($_.Name -match 'darwin-arm64') { $platform = "darwin-arm64" }
    elseif ($_.Name -match 'darwin-amd64') { $platform = "darwin-amd64" }
    "    {`"name`": `"$($_.Name)`", `"platform`": `"$platform`", `"size`": $($_.Length), `"url`": `"/releases/download/$($_.Name)`"}"
}) -join ",`n"

$manifestJson = @"
{
  "tag_name": "$vTag",
  "version": "$version",
  "name": "StarClaw Spore $vTag",
  "published_at": "$(Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ')",
  "assets": [
$assetsJson
  ]
}
"@

$tmpManifest = Join-Path $env:TEMP "spore-latest.json"
[IO.File]::WriteAllText($tmpManifest, $manifestJson.Replace("`r`n", "`n"))
& scp @sshArgs $tmpManifest "$($NydusHost):$ReleasesPath/spore-latest.json"
Remove-Item $tmpManifest -ErrorAction SilentlyContinue

# ── Done ──
Write-Host "`n========================================" -ForegroundColor Green
Write-Host "  Spore $vTag Released!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "`nDownload URLs:"
foreach ($f in $artifacts) {
    Write-Host "  https://nydus.starclaw.net/releases/download/$($f.Name)"
}
Write-Host "  https://nydus.starclaw.net/releases/download/spore-latest.json"
Write-Host ""
