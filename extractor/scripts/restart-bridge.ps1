# Kill existing python processes
$procs = Get-Process -Name python -ErrorAction SilentlyContinue
if ($procs) {
    $procs | ForEach-Object { Stop-Process -Id $_.Id -Force }
    Write-Host "Killed existing python processes"
    Start-Sleep 2
}

# Start bridge
Set-Location C:\extractor\bridge
Start-Process -FilePath "C:\Python311\python.exe" -ArgumentList "main.py" -WindowStyle Hidden
Write-Host "Bridge starting..."
Start-Sleep 6

# Health check
try {
    $resp = Invoke-RestMethod http://localhost:8098/health
    Write-Host "Health: $($resp | ConvertTo-Json -Compress)"
} catch {
    Write-Host "Health check failed: $_"
    Write-Host "Checking if python is running..."
    Get-Process -Name python -ErrorAction SilentlyContinue | Format-Table Id, ProcessName, StartTime -AutoSize
}
