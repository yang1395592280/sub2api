INSERT INTO settings (key, value)
VALUES ('game_center_enabled', 'true')
ON CONFLICT (key) DO NOTHING;

UPDATE settings
SET value = 'true'
WHERE key = 'game_center_enabled' AND value = 'false';
