-- Migration: 233_channel_monitor_probe_attempts
-- 为渠道监控增加每次模型探测的并发尝试次数。

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS probe_attempts INTEGER NOT NULL DEFAULT 3;

ALTER TABLE channel_monitors
    DROP CONSTRAINT IF EXISTS channel_monitors_probe_attempts_check;

ALTER TABLE channel_monitors
    ADD CONSTRAINT channel_monitors_probe_attempts_check
    CHECK (probe_attempts BETWEEN 1 AND 5);
