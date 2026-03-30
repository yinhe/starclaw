# Check if bridge is running
$procs = Get-Process -Name python -ErrorAction SilentlyContinue
if ($procs) {
    Write-Host "Python processes running:"
    $procs | Format-Table Id, ProcessName, StartTime -AutoSize
} else {
    Write-Host "No python process found!"
}

# Check port 8098
$conn = Get-NetTCPConnection -LocalPort 8098 -ErrorAction SilentlyContinue
if ($conn) {
    Write-Host "Port 8098 is in use:"
    $conn | Format-Table OwningProcess, State -AutoSize
} else {
    Write-Host "Port 8098 is NOT in use - bridge is down"
    Write-Host "Restarting bridge..."
    Set-Location C:\extractor\bridge
    Start-Process -FilePath "C:\Python311\python.exe" -ArgumentList "main.py" -WindowStyle Hidden
    Start-Sleep 8
    try {
        $h = Invoke-RestMethod http://localhost:8098/health
        Write-Host "Health after restart: $($h | ConvertTo-Json -Compress)"
    } catch {
        Write-Host "Still failed: $_"
        Write-Host "Trying to run bridge in foreground to see errors..."
        & C:\Python311\python.exe C:\extractor\bridge\main.py 2>&1 | Select-Object -First 30
    }
}
