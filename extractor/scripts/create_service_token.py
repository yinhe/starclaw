import sqlite3
import uuid
import secrets

DB_PATH = r"C:\Users\Yinhe\.spore\installed\claw\v2026.0329.0852\data\claw.db"

db = sqlite3.connect(DB_PATH)
c = db.cursor()

# Check if table exists
c.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='service_tokens'")
if not c.fetchone():
    print("Table not found, creating...")
    c.execute("""CREATE TABLE service_tokens (
        id VARCHAR(36) PRIMARY KEY,
        token VARCHAR(64) UNIQUE NOT NULL,
        name VARCHAR(100),
        origin VARCHAR(200),
        permissions VARCHAR(500) DEFAULT 'chat',
        user_id VARCHAR(36),
        revoked BOOLEAN DEFAULT 0,
        last_used_at DATETIME,
        expires_at DATETIME,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        deleted_at DATETIME
    )""")
    print("Table created.")

token = "svc-" + secrets.token_hex(32)
uid = str(uuid.uuid4())
c.execute(
    "INSERT INTO service_tokens (id, token, name, origin, permissions, user_id) VALUES (?, ?, ?, ?, ?, ?)",
    (uid, token, "q8bot-extractor", "q8bot.com", "chat", ""),
)
db.commit()
db.close()

print(f"SERVICE_TOKEN={token}")
