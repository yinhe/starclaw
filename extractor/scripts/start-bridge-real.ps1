# Find the correct QMT userdata_mini path and start bridge with it

# Kill existing python
Get-Process -Name python -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.Id -Force }
Start-Sleep 2

# Find all QMT dirs with userdata_mini (use ASCII-safe wildcard)
$qmtDirs = Get-ChildItem C:\ -Directory | Where-Object { $_.Name -like '*QMT*' }
$qmtPath = $null
$allPaths = @()

foreach ($d in $qmtDirs) {
    $udm = Join-Path $d.FullName "userdata_mini"
    if (Test-Path $udm) {
        $allPaths += $udm
        Write-Host "Found: $udm"
        # Use the first match that has userdata_mini
        if (-not $qmtPath) { $qmtPath = $udm }
    }
}

if (-not $qmtPath) {
    Write-Host "ERROR: No QMT userdata_mini found!"
    exit 1
}

Write-Host "Using QMT_PATH=$qmtPath"

# Write env file for bridge to pick up, and also set system env
[System.Environment]::SetEnvironmentVariable("QMT_PATH", $qmtPath, "Machine")
$env:QMT_PATH = $qmtPath

# Write a .env file as backup
Set-Content -Path "C:\extractor\bridge\.env" -Value "QMT_PATH=$qmtPath"

Set-Location C:\extractor\bridge
# Use cmd /c to pass env var to child process
$cmd = "set QMT_PATH=$qmtPath && cd /d C:\extractor\bridge && C:\Python311\python.exe main.py"
Start-Process cmd.exe -ArgumentList "/c", $cmd -WindowStyle Hidden

# Wait and check
Write-Host "Bridge starting with real QMT connection..."
Start-Sleep 8

try {
    $resp = Invoke-RestMethod http://localhost:8098/health
    Write-Host "Health: $($resp | ConvertTo-Json -Compress)"
} catch {
    Write-Host "Health check failed, checking logs..."
    # Try reading the last few lines of any log
    if (Test-Path C:\extractor\bridge\bridge.log) {
        Get-Content C:\extractor\bridge\bridge.log -Tail 20
    }
}
