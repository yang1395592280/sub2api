ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS dynamic_billing_profit_markup DECIMAL(12,6);

COMMENT ON COLUMN groups.dynamic_billing_profit_markup IS
    'OpenAI 动态扣费分组固定利润；NULL 继承全局配置，0 表示不加利润';

-- 动态扣费字段会进入 API Key 认证快照。继续沿用持久化
-- outbox 触发器，避免直接 SQL 修改后节点仍读取旧计费配置。
CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.platform IS NOT DISTINCT FROM NEW.platform
       AND OLD.subscription_type IS NOT DISTINCT FROM NEW.subscription_type
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.peak_rate_enabled IS NOT DISTINCT FROM NEW.peak_rate_enabled
       AND OLD.peak_start IS NOT DISTINCT FROM NEW.peak_start
       AND OLD.peak_end IS NOT DISTINCT FROM NEW.peak_end
       AND OLD.peak_rate_multiplier IS NOT DISTINCT FROM NEW.peak_rate_multiplier
       AND OLD.profit_control_enabled IS NOT DISTINCT FROM NEW.profit_control_enabled
       AND OLD.profit_min_margin IS NOT DISTINCT FROM NEW.profit_min_margin
       AND OLD.profit_safety_buffer IS NOT DISTINCT FROM NEW.profit_safety_buffer
       AND OLD.dynamic_billing_enabled IS NOT DISTINCT FROM NEW.dynamic_billing_enabled
       AND OLD.dynamic_billing_profit_markup IS NOT DISTINCT FROM NEW.dynamic_billing_profit_markup
       AND OLD.upstream_price_grouping_enabled IS NOT DISTINCT FROM NEW.upstream_price_grouping_enabled
       AND OLD.upstream_price_grouping_min IS NOT DISTINCT FROM NEW.upstream_price_grouping_min
       AND OLD.upstream_price_grouping_max IS NOT DISTINCT FROM NEW.upstream_price_grouping_max
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.group_id = target_group_id
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
