$ErrorActionPreference = "Stop"
$extractorHome = "C:\extractor"
$bridgeDir = Join-Path $extractorHome "bridge"
$pythonExe = "C:\Python311\python.exe"
$logDir = Join-Path $extractorHome "logs"
$envFile = Join-Path $extractorHome "env.ps1"

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

if (Test-Path $envFile) {
    . $envFile
}

$existing = Get-CimInstance Win32_Process | Where-Object {
    $_.Name -match '^python(.exe)?$' -and $_.CommandLine -match 'extractor\\bridge\\main.py'
}
foreach ($proc in $existing) {
    Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
}

Set-Location $bridgeDir
$logFile = Join-Path $logDir "bridge.log"
$cmd = 'cd /d "' + $bridgeDir + '" && "' + $pythonExe + '" main.py >> "' + $logFile + '" 2>&1'
Start-Process -FilePath "cmd.exe" -ArgumentList "/c", $cmd -WindowStyle Hidden
