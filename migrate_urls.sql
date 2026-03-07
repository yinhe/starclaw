-- Migration: Update /api/v1/ → /v1/ in all stored URLs
-- Run this on the server after deploying the new code:
--   docker exec -i starclaw-mysql mysql -uroot -p${DB_ROOT_PASSWORD} starclaw < migrate_urls.sql

-- Video records
UPDATE video_records SET video_url = REPLACE(video_url, '/api/v1/', '/v1/') WHERE video_url LIKE '%/api/v1/%';
UPDATE video_records SET img_url = REPLACE(img_url, '/api/v1/', '/v1/') WHERE img_url LIKE '%/api/v1/%';
UPDATE video_records SET narrated_url = REPLACE(narrated_url, '/api/v1/', '/v1/') WHERE narrated_url LIKE '%/api/v1/%';

-- Music records
UPDATE music_records SET local_url = REPLACE(local_url, '/api/v1/', '/v1/') WHERE local_url LIKE '%/api/v1/%';

-- Image records
UPDATE image_records SET local_url = REPLACE(local_url, '/api/v1/', '/v1/') WHERE local_url LIKE '%/api/v1/%';

SELECT 'Migration complete: /api/v1/ → /v1/' AS status;
