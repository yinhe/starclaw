# Spore — Windows installer for StarClaw ultra-lightweight deployment system
# Usage: irm https://spore.starclaw.me/install.ps1 | iex
#   or:  .\install.ps1

$ErrorActionPreference = "Stop"

$Repo = "yinhe/starclaw-spore"
$Version = if ($env:SPORE_VERSION) { $env:SPORE_VERSION } else { "latest" }

Write-Host ""
Write-Host "  +=======================================+" -ForegroundColor Cyan
Write-Host "  |   Spore - StarClaw Deployment System  |" -ForegroundColor Cyan
Write-Host "  |   Ultra-lightweight. Any device.      |" -ForegroundColor Cyan
Write-Host "  +=======================================+" -ForegroundColor Cyan
Write-Host ""

# Detect architecture
$Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Host "[spore] Platform: windows/$Arch" -ForegroundColor Green

# Install directory
$InstallDir = "$env:LOCALAPPDATA\spore\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Download spore.exe
$Url = "https://github.com/$Repo/releases/download/$Version/spore-windows-$Arch.exe"
$SporeExe = "$InstallDir\spore.exe"

Write-Host "[spore] Downloading spore..." -ForegroundColor Green
try {
    Invoke-WebRequest -Uri $Url -OutFile $SporeExe -UseBasicParsing
} catch {
    # If release download fails, try building from source
    Write-Host "[spore] Release not available, building from source..." -ForegroundColor Yellow
    if (Get-Command go -ErrorAction SilentlyContinue) {
        go install "github.com/$Repo/cmd/spore@latest"
        go install "github.com/$Repo/cmd/hatchery@latest"
        Write-Host "[spore] Built from source via go install" -ForegroundColor Green
        Write-Host "[spore] Binaries in: $(go env GOPATH)\bin" -ForegroundColor Green
        return
    } else {
        Write-Host "[spore] Error: Download failed and Go is not installed." -ForegroundColor Red
        Write-Host "[spore] Install Go from https://go.dev or download a release manually." -ForegroundColor Red
        exit 1
    }
}

# Download hatchery.exe (optional)
$HatcheryUrl = "https://github.com/$Repo/releases/download/$Version/hatchery-windows-$Arch.exe"
$HatcheryExe = "$InstallDir\hatchery.exe"
try {
    Invoke-WebRequest -Uri $HatcheryUrl -OutFile $HatcheryExe -UseBasicParsing
    Write-Host "[spore] Installed hatchery to $HatcheryExe" -ForegroundColor Green
} catch {
    Write-Host "[spore] Hatchery download skipped (optional build tool)" -ForegroundColor Yellow
}

# Add to PATH if not already
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Host "[spore] Added $InstallDir to PATH" -ForegroundColor Green
}

# Create SPORE_HOME
$SporeHome = if ($env:SPORE_HOME) { $env:SPORE_HOME } else { "$env:USERPROFILE\.spore" }
New-Item -ItemType Directory -Path "$SporeHome\cache" -Force | Out-Null
New-Item -ItemType Directory -Path "$SporeHome\installed" -Force | Out-Null
New-Item -ItemType Directory -Path "$SporeHome\registry" -Force | Out-Null

Write-Host "[spore] Spore home: $SporeHome" -ForegroundColor Green
Write-Host "[spore] Installed to: $SporeExe" -ForegroundColor Green
Write-Host ""
Write-Host "[spore] Quick start:" -ForegroundColor Green
Write-Host "  spore install .\claw.spore    # Install a .spore package"
Write-Host "  spore start claw              # Start the service"
Write-Host "  spore status                  # Check status"
Write-Host ""
Write-Host "[spore] NOTE: Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
