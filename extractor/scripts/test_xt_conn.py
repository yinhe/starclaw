"""Test xtquant connection to miniQMT - pure ASCII"""
import sys, os, glob

print("Python:", sys.version)

# Step 1: import xtquant
try:
    from xtquant import xtdata
    from xtquant.xttrader import XtQuantTrader
    from xtquant.xttype import StockAccount
    print("[OK] xtquant imported")
except Exception as e:
    print(f"[FAIL] xtquant import: {e}")
    sys.exit(1)

# Step 2: find QMT userdata_mini paths
qmt_roots = glob.glob("C:\\*QMT*")
print(f"\nQMT dirs found: {qmt_roots}")

paths = []
for root in qmt_roots:
    udm = os.path.join(root, "userdata_mini")
    if os.path.isdir(udm):
        paths.append(udm)
        print(f"  userdata_mini: {udm}")
        contents = os.listdir(udm)
        print(f"    contents: {contents[:8]}")

# Step 3: try xtdata.connect() first (market data, no trader needed)
print("\n--- xtdata.connect() test ---")
try:
    xtdata.connect()
    print("[OK] xtdata connected")
except Exception as e:
    print(f"[WARN] xtdata.connect: {e}")

# Step 4: try XtQuantTrader connection with each path
print("\n--- XtQuantTrader connection test ---")
for p in paths:
    print(f"\nTrying path: {p}")
    try:
        trader = XtQuantTrader(p, 654321)
        trader.start()
        import time
        time.sleep(2)
        result = trader.connect()
        print(f"  connect() returned: {result}")
        if result == 0:
            print("  [OK] CONNECTED to miniQMT!")
            # Try to subscribe account
            try:
                acc = StockAccount("test1006")
                sub = trader.subscribe_account(acc)
                print(f"  subscribe_account result: {sub}")
            except Exception as e2:
                print(f"  subscribe_account error: {e2}")
            trader.stop()
            break
        else:
            print(f"  [FAIL] connect returned {result}")
            trader.stop()
    except Exception as e:
        print(f"  [ERROR] {e}")

# Step 5: try fetching some market data
print("\n--- Market data test ---")
try:
    codes = xtdata.get_stock_list_in_sector("沪深A股".encode().decode())
    print(f"[OK] A-share stock count: {len(codes)}")
    if codes:
        print(f"  First 5: {codes[:5]}")
except Exception as e:
    print(f"[WARN] get_stock_list: {e}")

try:
    data = xtdata.get_market_data_ex([], ["000001.SZ"], period="1d", count=5)
    if "000001.SZ" in data:
        print(f"[OK] Got kline data for 000001.SZ: {len(data['000001.SZ'])} bars")
        print(data["000001.SZ"].tail(2))
    else:
        print("[WARN] No data returned for 000001.SZ")
except Exception as e:
    print(f"[WARN] get_market_data_ex: {e}")
