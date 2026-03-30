# Test real market data from Bridge
Write-Host "=== Health ==="
try {
    $h = Invoke-RestMethod http://localhost:8098/health
    Write-Host ($h | ConvertTo-Json -Compress)
} catch { Write-Host "Health failed: $_" }

Write-Host "`n=== Quote: 000001.SZ (Ping An Bank) ==="
try {
    $q = Invoke-RestMethod "http://localhost:8098/market/quote?codes=000001.SZ"
    Write-Host ($q | ConvertTo-Json -Depth 3)
} catch { Write-Host "Quote failed: $_" }

Write-Host "`n=== Kline: 000001.SZ (last 5 bars) ==="
try {
    $k = Invoke-RestMethod "http://localhost:8098/market/kline?code=000001.SZ&period=1d&count=5"
    Write-Host ($k | ConvertTo-Json -Depth 3)
} catch { Write-Host "Kline failed: $_" }

Write-Host "`n=== Scan (real data) ==="
try {
    $s = Invoke-RestMethod -Method POST http://localhost:8098/scan -ContentType "application/json" -Body '{}'
    Write-Host ($s | ConvertTo-Json -Depth 4 -Compress)
} catch { Write-Host "Scan failed: $_" }
