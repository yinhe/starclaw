UPDATE workflows SET webhook_token = NULL WHERE webhook_token = '';
SELECT COUNT(*) as total FROM workflows;
