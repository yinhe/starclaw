# Kill, restart, and test bridge in one session
Get-Process -Name python -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.Id -Force }
Start-Sleep 2

# Start bridge as a background job so it persists in this session
Set-Location C:\extractor\bridge
$job = Start-Job -ScriptBlock {
    Set-Location C:\extractor\bridge
    & C:\Python311\python.exe main.py 2>&1
}
Write-Host "Bridge job started: $($job.Id)"
Start-Sleep 8

# Health
Write-Host "`n=== Health ==="
try {
    $h = Invoke-RestMethod http://localhost:8098/health
    Write-Host ($h | ConvertTo-Json -Compress)
} catch { Write-Host "FAIL: $_" }

# Quote
Write-Host "`n=== Quote: 000001.SZ ==="
try {
    $q = Invoke-RestMethod "http://localhost:8098/market/quote?codes=000001.SZ"
    Write-Host ($q | ConvertTo-Json -Depth 3)
} catch { Write-Host "FAIL: $_" }

# Kline
Write-Host "`n=== Kline: 000001.SZ ==="
try {
    $k = Invoke-RestMethod "http://localhost:8098/market/kline?code=000001.SZ&period=1d&count=3"
    Write-Host ($k | ConvertTo-Json -Depth 3)
} catch { Write-Host "FAIL: $_" }

# Scan
Write-Host "`n=== Scan ==="
try {
    $s = Invoke-RestMethod -Method POST http://localhost:8098/scan -ContentType "application/json" -Body '{}'
    Write-Host ($s | ConvertTo-Json -Depth 4 -Compress)
} catch { Write-Host "FAIL: $_" }

# Check job output for errors
Write-Host "`n=== Bridge log (last lines) ==="
$output = Receive-Job -Job $job -ErrorAction SilentlyContinue
if ($output) { $output | Select-Object -Last 15 }

# Now register as a scheduled task so it persists
Write-Host "`n=== Registering as scheduled task ==="
$action = New-ScheduledTaskAction -Execute "C:\Python311\python.exe" -Argument "main.py" -WorkingDirectory "C:\extractor\bridge"
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName "ExtractorBridge" -Action $action -Trigger $trigger -Settings $settings -User "SYSTEM" -RunLevel Highest -Force
Start-ScheduledTask -TaskName "ExtractorBridge"
Write-Host "Scheduled task registered and started"
Start-Sleep 3

# Final health check via scheduled task
try {
    $h2 = Invoke-RestMethod http://localhost:8098/health
    Write-Host "Final health: $($h2 | ConvertTo-Json -Compress)"
} catch { Write-Host "Final health FAIL: $_" }
