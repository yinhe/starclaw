# Find QMT directories and copy xtquant to Python site-packages
$qmtDirs = Get-ChildItem C:\ -Directory | Where-Object { $_.Name -like '*QMT*' }
Write-Host "Found QMT dirs:"
$qmtDirs | ForEach-Object { Write-Host "  $_" }

$copied = $false
foreach ($d in $qmtDirs) {
    $xtPath = Join-Path $d.FullName "bin.x64\Lib\site-packages\xtquant"
    if (Test-Path $xtPath) {
        Write-Host "Found xtquant at: $xtPath"
        $dest = "C:\Python311\Lib\site-packages\xtquant"
        if (Test-Path $dest) { Remove-Item $dest -Recurse -Force }
        Copy-Item $xtPath $dest -Recurse -Force
        Write-Host "Copied xtquant to $dest"
        $copied = $true

        # Also copy xtdata if exists
        $xtdataPath = Join-Path $d.FullName "bin.x64\Lib\site-packages\xtdata"
        if (Test-Path $xtdataPath) {
            $destData = "C:\Python311\Lib\site-packages\xtdata"
            if (Test-Path $destData) { Remove-Item $destData -Recurse -Force }
            Copy-Item $xtdataPath $destData -Recurse -Force
            Write-Host "Copied xtdata to $destData"
        }

        # Also copy qmt_api if exists
        $qmtApiPath = Join-Path $d.FullName "bin.x64\Lib\site-packages\qmt_api"
        if (Test-Path $qmtApiPath) {
            $destApi = "C:\Python311\Lib\site-packages\qmt_api"
            if (Test-Path $destApi) { Remove-Item $destApi -Recurse -Force }
            Copy-Item $qmtApiPath $destApi -Recurse -Force
            Write-Host "Copied qmt_api to $destApi"
        }
        break
    }
}

if (-not $copied) {
    Write-Host "ERROR: xtquant not found in any QMT directory!"
    Write-Host "Checking bin.x64\Lib\site-packages in each QMT dir..."
    foreach ($d in $qmtDirs) {
        $spPath = Join-Path $d.FullName "bin.x64\Lib\site-packages"
        if (Test-Path $spPath) {
            Write-Host "Contents of ${spPath}:"
            Get-ChildItem $spPath | ForEach-Object { Write-Host "  $_" }
        }
    }
}

# Verify
Write-Host "`n--- Verification ---"
$verifyPath = "C:\Python311\Lib\site-packages\xtquant"
if (Test-Path $verifyPath) {
    Write-Host "OK: xtquant exists at $verifyPath"
    Write-Host "Files:"
    Get-ChildItem $verifyPath -Name | Select-Object -First 10
} else {
    Write-Host "FAIL: xtquant NOT found at $verifyPath"
}
