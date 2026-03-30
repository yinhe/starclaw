import sqlite3, json, sys

db = r'C:\Users\Yinhe\.spore\installed\claw\v2026.0329.0852\data\claw.db'
agent_id = '44f9781b-a1cf-41b2-90a3-7c322bd2ffc9'

conn = sqlite3.connect(db)

# List tables
tables = [r[0] for r in conn.execute("SELECT name FROM sqlite_master WHERE type='table'")]
print("Tables:", tables[:20])

# Find agent table
for t in tables:
    cols = [r[1] for r in conn.execute(f"PRAGMA table_info({t})")]
    if 'tools' in cols and 'name' in cols:
        print(f"\nFound agent-like table: {t}")
        r = conn.execute(f"SELECT id, name, tools FROM {t} WHERE id=?", (agent_id,)).fetchone()
        if r:
            print(f"  id={r[0]}, name={r[1]}, tools={r[2][:200] if r[2] else 'NULL'}")
            tools = json.loads(r[2]) if r[2] else []
            if 'wechat_cs' not in tools:
                tools.append('wechat_cs')
                conn.execute(f"UPDATE {t} SET tools=? WHERE id=?", (json.dumps(tools), agent_id))
                conn.commit()
                print(f"  UPDATED: added wechat_cs, total {len(tools)} tools")
            else:
                print(f"  wechat_cs already present")

conn.close()
