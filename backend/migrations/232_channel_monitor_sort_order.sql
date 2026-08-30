-- Migration: 232_channel_monitor_sort_order
-- 为渠道监控增加可持久化的显示排序，供管理端和用户端渠道状态共用。

-- 仅在字段首次创建时初始化存量数据；重复执行迁移不得覆盖管理员已保存的顺序。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'channel_monitors'
          AND column_name = 'sort_order'
    ) THEN
        ALTER TABLE channel_monitors
            ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

        WITH ranked AS (
            SELECT id, (ROW_NUMBER() OVER (ORDER BY id) - 1) * 10 AS next_sort_order
            FROM channel_monitors
        )
        UPDATE channel_monitors AS monitors
        SET sort_order = ranked.next_sort_order
        FROM ranked
        WHERE monitors.id = ranked.id;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_channel_monitors_sort_order
    ON channel_monitors (sort_order, id);
