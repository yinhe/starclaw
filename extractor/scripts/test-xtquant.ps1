# Test xtquant connection directly
$qmtDirs = Get-ChildItem C:\ -Directory | Where-Object { $_.Name -like '*QMT*' }
$paths = @()
foreach ($d in $qmtDirs) {
    $udm = Join-Path $d.FullName "userdata_mini"
    if (Test-Path $udm) {
        $paths += $udm
        Write-Host "QMT userdata_mini: $udm"
    }
}

$testScript = @'
import sys, os
print("Python:", sys.version)
print("sys.path:", sys.path[:3])

try:
    from xtquant import xtdata
    print("xtquant imported OK")
    print("xtdata module:", xtdata)
except Exception as e:
    print(f"xtquant import error: {e}")
    sys.exit(1)

try:
    from xtquant.xttrader import XtQuantTrader
    print("XtQuantTrader imported OK")
except Exception as e:
    print(f"XtQuantTrader import error: {e}")
    sys.exit(1)

# Try connecting with each QMT path
qmt_path = os.environ.get("QMT_PATH", "")
print(f"\nQMT_PATH env: {qmt_path}")

# Try all paths from args
import json
paths_str = sys.argv[1] if len(sys.argv) > 1 else "[]"
paths = json.loads(paths_str)
print(f"Testing {len(paths)} paths...")

for p in paths:
    print(f"\n--- Testing path: {p} ---")
    if not os.path.exists(p):
        print(f"  Path does not exist!")
        continue
    print(f"  Contents: {os.listdir(p)[:10]}")
    try:
        trader = XtQuantTrader(p, 123456)
        trader.start()
        result = trader.connect()
        print(f"  Connect result: {result}")
        if result == 0:
            print("  SUCCESS! Connected to miniQMT")
            trader.stop()
            break
        trader.stop()
    except Exception as e:
        print(f"  Connect error: {e}")

# Also test xtdata directly (no trader needed for market data)
print("\n--- Testing xtdata directly ---")
try:
    xtdata.connect()
    print("xtdata.connect() OK")
except Exception as e:
    print(f"xtdata.connect() error: {e}")

try:
    codes = xtdata.get_stock_list_in_sector("沪深A股")
    print(f"Stock list count: {len(codes)}")
    if codes:
        print(f"First 5: {codes[:5]}")
except Exception as e:
    print(f"get_stock_list error: {e}")
'@

Set-Content -Path "C:\extractor\test_xt.py" -Value $testScript -Encoding UTF8

# Convert paths to JSON
$jsonPaths = ($paths | ConvertTo-Json -Compress)
if (-not $jsonPaths) { $jsonPaths = "[]" }

Write-Host "`nRunning test..."
& C:\Python311\python.exe C:\extractor\test_xt.py $jsonPaths
