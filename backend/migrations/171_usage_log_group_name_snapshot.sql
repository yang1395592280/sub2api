ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS group_name VARCHAR(255) NULL;

COMMENT ON COLUMN usage_logs.group_name IS
    '分组名称快照，用于分组删除后保持历史使用记录可读';

UPDATE usage_logs ul
SET group_name = g.name
FROM groups g
WHERE ul.group_id = g.id
  AND ul.group_name IS NULL;
