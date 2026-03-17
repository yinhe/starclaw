# StarClaw Claw — Windows One-Click Installer via Spore
# Usage: irm https://nydus.starclaw.net/spore/install.ps1 | iex
$ErrorActionPreference = "Stop"

$SporeUrl = "https://nydus.starclaw.net/spore/releases/spore-windows-amd64.exe"
$ClawUrl = "https://nydus.starclaw.net/spore/releases/claw-v1.0.0-windows-amd64.spore"

Write-Host ""
Write-Host "  +========================================+" -ForegroundColor Cyan
Write-Host "  |   StarClaw Claw - Quick Install         |" -ForegroundColor Cyan
Write-Host "  |   Windows One-Click via Spore           |" -ForegroundColor Cyan
Write-Host "  +========================================+" -ForegroundColor Cyan
Write-Host ""

# Install directory
$InstallDir = "$env:LOCALAPPDATA\spore\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Download spore.exe
Write-Host "[spore] Downloading Spore runtime..." -ForegroundColor Green
$SporeExe = "$InstallDir\spore.exe"
try {
    Invoke-WebRequest -Uri $SporeUrl -OutFile $SporeExe -UseBasicParsing
} catch {
    Write-Host "[spore] Download failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host "[spore] Spore installed to $SporeExe" -ForegroundColor Green

# Download claw.spore
$TmpSpore = "$env:TEMP\claw-$([guid]::NewGuid().ToString().Substring(0,8)).spore"
Write-Host "[spore] Downloading Claw package (11.5 MB)..." -ForegroundColor Green
try {
    Invoke-WebRequest -Uri $ClawUrl -OutFile $TmpSpore -UseBasicParsing
} catch {
    Write-Host "[spore] Download failed: $_" -ForegroundColor Red
    exit 1
}

# Add to PATH if not already
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "[spore] Added $InstallDir to PATH" -ForegroundColor Yellow
}

# Install claw
Write-Host "[spore] Installing Claw..." -ForegroundColor Green
& $SporeExe install $TmpSpore
Remove-Item $TmpSpore -ErrorAction SilentlyContinue

# Interactive configuration
$SporeHome = if ($env:SPORE_HOME) { $env:SPORE_HOME } else { "$env:USERPROFILE\.spore" }
$ConfigDir = "$SporeHome\installed\claw\current\config"
New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null

Write-Host ""
Write-Host "[spore] === Quick Configuration ===" -ForegroundColor Green
Write-Host ""

# AI Provider
$Provider = Read-Host "  AI Provider [openai/deepseek/qwen/ollama] (default: openai)"
if (!$Provider) { $Provider = "openai" }

if ($Provider -eq "ollama") {
    $ApiUrl = Read-Host "  Ollama URL (default: http://localhost:11434)"
    if (!$ApiUrl) { $ApiUrl = "http://localhost:11434" }
    $ApiKey = ""
} else {
    $ApiKey = Read-Host "  API Key"
    $ApiUrl = ""
}

$Port = Read-Host "  Server port (default: 8080)"
if (!$Port) { $Port = "8080" }

# Generate JWT secret
$JwtSecret = -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 32 | ForEach-Object { [char]$_ })

# Write config
$ConfigContent = @"
server:
  host: 0.0.0.0
  port: $Port

database:
  driver: sqlite
  dsn: "./data/claw.db"

jwt:
  secret: "$JwtSecret"
"@
Set-Content -Path "$ConfigDir\config.yaml" -Value $ConfigContent -Encoding UTF8

# Write .env
$EnvContent = @"
GIN_MODE=release
CLAW_DATA_DIR=./data
CLAW_PORT=$Port
"@
Set-Content -Path "$SporeHome\installed\claw\current\.env" -Value $EnvContent -Encoding UTF8

# Start
Write-Host ""
Write-Host "[spore] Starting Claw..." -ForegroundColor Green
& $SporeExe start claw

Write-Host ""
Write-Host "[spore] === Installation Complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "  Web UI:  http://localhost:$Port" -ForegroundColor White
Write-Host "  API:     http://localhost:$Port/v1" -ForegroundColor White
Write-Host ""
Write-Host "  Commands:" -ForegroundColor Gray
Write-Host "    spore status        # Check status" -ForegroundColor Gray
Write-Host "    spore logs claw     # View logs" -ForegroundColor Gray
Write-Host "    spore stop claw     # Stop" -ForegroundColor Gray
Write-Host "    spore restart claw  # Restart" -ForegroundColor Gray
Write-Host ""
