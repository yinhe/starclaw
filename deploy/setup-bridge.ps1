# StarClaw MCP Bridge - Auto Install Script (Windows)
# Usage: irm https://raw.githubusercontent.com/yinhe/starclaw/main/deploy/setup-bridge.ps1 | iex

$ErrorActionPreference = "Stop"

$BridgePort = 9101
$Repo = "yinhe/starclaw"
$InstallDir = "$env:LOCALAPPDATA\StarClaw"
$BinaryName = "mcp-bridge.exe"
$TaskName = "StarClaw MCP Bridge"

Write-Host "`n* StarClaw MCP Bridge Installer" -ForegroundColor Cyan
Write-Host "=================================" -ForegroundColor Cyan

# Detect architecture
$Arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
switch ($Arch) {
    "X64"   { $GoArch = "amd64" }
    "Arm64" { $GoArch = "arm64" }
    default { Write-Error "Unsupported architecture: $Arch"; exit 1 }
}

$AssetName = "mcp-bridge-windows-${GoArch}.exe"
$DownloadUrl = "https://github.com/$Repo/releases/latest/download/$AssetName"

Write-Host "  OS: Windows, Arch: $GoArch" -ForegroundColor Gray
Write-Host "  Downloading from: $DownloadUrl" -ForegroundColor Gray

# Create install directory
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetPath = Join-Path $InstallDir $BinaryName

# Stop existing process if running
$existing = Get-Process -Name "mcp-bridge" -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "  Stopping existing MCP Bridge..." -ForegroundColor Yellow
    Stop-Process -Name "mcp-bridge" -Force
    Start-Sleep -Seconds 1
}

# Download
Write-Host "  Downloading..." -ForegroundColor Gray
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TargetPath -UseBasicParsing
} catch {
    # Fallback: check if binary exists in dist/ (local development)
    $localBin = Join-Path $PSScriptRoot "..\dist\$AssetName"
    if (Test-Path $localBin) {
        Write-Host "  Using local binary from dist/" -ForegroundColor Yellow
        Copy-Item $localBin $TargetPath -Force
    } else {
        Write-Error "Download failed and no local binary found: $_"
        exit 1
    }
}

Write-Host "  Installed to: $TargetPath" -ForegroundColor Green

# Setup scheduled task for auto-start
Write-Host "  Setting up auto-start task..." -ForegroundColor Gray
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

$action = New-ScheduledTaskAction -Execute $TargetPath -Argument "-port $BridgePort"
$trigger = New-ScheduledTaskTrigger -AtLogon
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -RunLevel Limited

try {
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description "StarClaw MCP Bridge for host control" | Out-Null
    Write-Host "  Auto-start task registered (runs at logon)" -ForegroundColor Green
} catch {
    Write-Host "  Warning: Could not register auto-start task (non-admin). Bridge will need manual start." -ForegroundColor Yellow
}

# Start now
Write-Host "  Starting MCP Bridge on port $BridgePort..." -ForegroundColor Gray
Start-Process -FilePath $TargetPath -ArgumentList "-port",$BridgePort -WindowStyle Hidden

Start-Sleep -Seconds 2

# Verify
try {
    $health = Invoke-RestMethod -Uri "http://localhost:$BridgePort/health" -Method GET -ErrorAction Stop
    Write-Host "`n  MCP Bridge is running on port $BridgePort" -ForegroundColor Green
    Write-Host "  The Claw API will auto-detect and connect within 30 seconds." -ForegroundColor Gray
} catch {
    Write-Host "`n  Warning: Bridge started but health check failed. Check port $BridgePort." -ForegroundColor Yellow
}

Write-Host "`n  Done!`n" -ForegroundColor Cyan
