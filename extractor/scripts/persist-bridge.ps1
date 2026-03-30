# Create a persistent bridge process that survives SSH disconnect
# Step 1: Kill any existing
Get-Process -Name python -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.Id -Force }
Start-Sleep 2

# Step 2: Create a wrapper batch file
$bat = @"
@echo off
cd /d C:\extractor\bridge
C:\Python311\python.exe main.py >> C:\extractor\bridge.log 2>&1
"@
Set-Content -Path "C:\extractor\run-bridge.bat" -Value $bat

# Step 3: Register as scheduled task (runs as Administrator, persists)
$existingTask = Get-ScheduledTask -TaskName "ExtractorBridge" -ErrorAction SilentlyContinue
if ($existingTask) {
    Unregister-ScheduledTask -TaskName "ExtractorBridge" -Confirm:$false
}

$action = New-ScheduledTaskAction -Execute "C:\extractor\run-bridge.bat"
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "Administrator" -LogonType S4U -RunLevel Highest
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
Register-ScheduledTask -TaskName "ExtractorBridge" -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force
Start-ScheduledTask -TaskName "ExtractorBridge"

Write-Host "Task registered and started"
Start-Sleep 10

# Step 4: Verify
try {
    $h = Invoke-RestMethod http://localhost:8098/health
    Write-Host "Health: $($h | ConvertTo-Json -Compress)"
} catch {
    Write-Host "Health failed, checking log..."
    if (Test-Path C:\extractor\bridge.log) {
        Get-Content C:\extractor\bridge.log -Tail 20
    }
}
