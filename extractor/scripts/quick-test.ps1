$ErrorActionPreference = "Continue"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

Write-Host "=== Health ==="
(Invoke-RestMethod http://localhost:8098/health) | ConvertTo-Json -Compress

Write-Host "`n=== Kline 000001.SZ (3 bars) ==="
$k = Invoke-RestMethod "http://localhost:8098/market/kline?code=000001.SZ&period=1d&count=3"
$k | ConvertTo-Json -Depth 3

Write-Host "`n=== Quote 000001.SZ ==="
$q = Invoke-RestMethod "http://localhost:8098/market/quote?codes=000001.SZ"
$q | ConvertTo-Json -Depth 3

Write-Host "`n=== Scan ==="
$s = Invoke-RestMethod -Method POST "http://localhost:8098/scan" -ContentType "application/json" -Body "{}"
$s | ConvertTo-Json -Depth 4
