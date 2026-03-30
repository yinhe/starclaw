$ErrorActionPreference = "Stop"
$extractorHome = "C:\extractor"
$scriptDir = Join-Path $extractorHome "scripts"
$bridgeScript = Join-Path $scriptDir "start-bridge-daemon.ps1"
$apiScript = Join-Path $scriptDir "start-api-daemon.ps1"

if (-not (Test-Path $bridgeScript)) {
    throw "Missing $bridgeScript"
}
if (-not (Test-Path $apiScript)) {
    throw "Missing $apiScript"
}

$tasks = @(
    @{ Name = "ExtractorBridge"; Script = $bridgeScript },
    @{ Name = "ExtractorAPI"; Script = $apiScript }
)

foreach ($task in $tasks) {
    $existingTask = Get-ScheduledTask -TaskName $task.Name -ErrorAction SilentlyContinue
    if ($existingTask) {
        Unregister-ScheduledTask -TaskName $task.Name -Confirm:$false
    }

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$($task.Script)`""
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId "Administrator" -LogonType S4U -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
    Register-ScheduledTask -TaskName $task.Name -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
}

Start-ScheduledTask -TaskName "ExtractorBridge"
Start-Sleep 10
Start-ScheduledTask -TaskName "ExtractorAPI"
Start-Sleep 8

Write-Host "Bridge health:" -ForegroundColor Cyan
try {
    Invoke-RestMethod http://localhost:8098/health | ConvertTo-Json -Compress
} catch {
    Write-Host $_
}

Write-Host "API health:" -ForegroundColor Cyan
try {
    Invoke-RestMethod http://localhost:8097/health | ConvertTo-Json -Compress
} catch {
    Write-Host $_
}
