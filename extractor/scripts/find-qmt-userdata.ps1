# Find QMT userdata_mini path
$qmtDirs = Get-ChildItem C:\ -Directory | Where-Object { $_.Name -like '*QMT*' }
foreach ($d in $qmtDirs) {
    Write-Host "QMT dir: $($d.FullName)"
    # Look for userdata_mini
    $udm = Get-ChildItem $d.FullName -Directory -Recurse -ErrorAction SilentlyContinue | Where-Object { $_.Name -eq 'userdata_mini' }
    if ($udm) {
        foreach ($u in $udm) {
            Write-Host "  FOUND userdata_mini: $($u.FullName)"
        }
    }
    # Also list top-level dirs
    Write-Host "  Top-level contents:"
    Get-ChildItem $d.FullName -Directory -ErrorAction SilentlyContinue | ForEach-Object { Write-Host "    [DIR] $($_.Name)" }
    Get-ChildItem $d.FullName -File -ErrorAction SilentlyContinue | Select-Object -First 5 | ForEach-Object { Write-Host "    [FILE] $($_.Name)" }
}
