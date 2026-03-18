#!/bin/bash
# Check and fix Claw users with empty email/phone in star_ai DB
docker exec star-ai-mysql mysql -uroot -pstarclaw123 star_ai -e "SELECT id, name, email, phone, claw_id FROM users WHERE claw_id != '' LIMIT 20;"

echo "--- Fixing empty email/phone for existing Claw users ---"
docker exec star-ai-mysql mysql -uroot -pstarclaw123 star_ai -e "
UPDATE users
SET email = CONCAT(LEFT(SHA2(claw_id, 256), 16), '@claw.local'),
    phone = CONCAT('claw-', LEFT(SHA2(claw_id, 256), 16))
WHERE claw_id != ''
  AND (email = '' OR phone = '');
"
echo "Done. Affected rows above."
