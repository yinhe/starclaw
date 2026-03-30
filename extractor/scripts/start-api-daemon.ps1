$ErrorActionPreference = "Stop"
$extractorHome = "C:\extractor"
$logDir = Join-Path $extractorHome "logs"
$envFile = Join-Path $extractorHome "env.ps1"
$preferredExe = Join-Path $extractorHome "extractor-api-300.exe"
$defaultExe = Join-Path $extractorHome "extractor-api.exe"

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

if (Test-Path $envFile) {
    . $envFile
}

if (-not $env:EXTRACTOR_DATABASE_DSN) {
    $env:EXTRACTOR_DATABASE_DSN = "sqlite:C:\extractor\data\extractor.db"
}
if (-not $env:EXTRACTOR_BRIDGE_URL) {
    $env:EXTRACTOR_BRIDGE_URL = "http://localhost:8098"
}
if (-not $env:EXTRACTOR_PORT) {
    $env:EXTRACTOR_PORT = "8097"
}

$apiExe = if (Test-Path $preferredExe) { $preferredExe } elseif (Test-Path $defaultExe) { $defaultExe } else { $null }
if (-not $apiExe) {
    throw "No Go API binary found at $preferredExe or $defaultExe"
}

$existing = Get-CimInstance Win32_Process | Where-Object {
    $_.Name -match '^extractor-api.*\.exe$' -or ($_.CommandLine -and $_.CommandLine -match 'extractor-api(-300)?\.exe')
}
foreach ($proc in $existing) {
    Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
}

Set-Location $extractorHome
$logFile = Join-Path $logDir "api.log"
$cmd = 'cd /d "' + $extractorHome + '" && "' + $apiExe + '" >> "' + $logFile + '" 2>&1'
Start-Process -FilePath "cmd.exe" -ArgumentList "/c", $cmd -WindowStyle Hidden
