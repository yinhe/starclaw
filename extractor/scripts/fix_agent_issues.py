"""Fix Agent issues: model binding, user_id, marketplace template."""
import sqlite3

DB = r"C:\Users\Yinhe\.spore\installed\claw\v2026.0329.0852\data\claw.db"
AGENT_ID = "7a57c3ec-2c29-433b-a387-b6e0a115f32b"

db = sqlite3.connect(DB)
c = db.cursor()

# 1. Get the real user_id
c.execute("SELECT id, username FROM users LIMIT 1")
user = c.fetchone()
user_id = user[0] if user else ""
print(f"User: id={user_id}, name={user[1] if user else 'none'}")

# 2. Get first enabled model
c.execute("SELECT id, provider, model_name FROM model_configs WHERE is_enabled=1 LIMIT 5")
models = c.fetchall()
model_id = ""
for m in models:
    print(f"  Model: id={m[0]}, provider={m[1]}, name={m[2]}")
    if not model_id and m[1] in ("star-ai", "qwen", "openai", "deepseek"):
        model_id = m[0]

if not model_id and models:
    model_id = models[0][0]
print(f"Selected model_id: {model_id}")

# 3. Fix Agent: bind model + correct user_id
c.execute("UPDATE agents SET model_id=?, user_id=? WHERE id=?", (model_id, user_id, AGENT_ID))
print(f"\nFIX 3: Agent model_id={model_id}, user_id={user_id}")

# 4. Fix all templates + listings: set author_id/creator_id to real user
c.execute("UPDATE agent_templates SET author_id=? WHERE author_id=''", (user_id,))
c.execute("UPDATE agent_listings SET creator_id=? WHERE creator_id=''", (user_id,))
print(f"FIX 4: Templates + listings author_id set to {user_id}")

# 5. Fix MCP server user_id
c.execute("UPDATE mcp_servers SET user_id=? WHERE user_id=''", (user_id,))
print(f"FIX 2: MCP servers user_id set to {user_id}")

# 6. Fix workflow user_id
c.execute("UPDATE workflows SET user_id=? WHERE user_id=''", (user_id,))
print(f"FIX: Workflows user_id set to {user_id}")

# 7. Fix agent_skills (they were created outside user context)
# No user_id field on skills, they link via agent_id which is fine

db.commit()
db.close()
print("\nAll fixes applied!")
