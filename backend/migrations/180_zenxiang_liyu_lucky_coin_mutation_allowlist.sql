CREATE OR REPLACE FUNCTION zenxiang_liyu_records_prevent_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE'
       AND OLD.prize_id IS NOT NULL
       AND NEW.prize_id IS NULL
       AND pg_trigger_depth() > 1
       AND to_jsonb(NEW) - 'prize_id' = to_jsonb(OLD) - 'prize_id' THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.lucky_coin_played = FALSE
       AND NEW.lucky_coin_played = TRUE
       AND OLD.lucky_coin_outcome = ''
       AND NEW.lucky_coin_outcome IN ('double', 'zero')
       AND OLD.lucky_coin_adjustment = 0
       AND (
           (NEW.lucky_coin_outcome = 'double' AND NEW.lucky_coin_adjustment = OLD.reward_amount)
           OR (NEW.lucky_coin_outcome = 'zero' AND NEW.lucky_coin_adjustment = -2 * OLD.reward_amount)
       )
       AND OLD.lucky_coin_played_at IS NULL
       AND NEW.lucky_coin_played_at IS NOT NULL
       AND OLD.balance_after_lucky IS NULL
       AND NEW.balance_after_lucky IS NOT NULL
       AND NEW.user_net_amount = OLD.user_net_amount + NEW.lucky_coin_adjustment
       AND NEW.system_expense = OLD.system_expense + NEW.lucky_coin_adjustment
       AND NEW.system_profit = OLD.system_profit - NEW.lucky_coin_adjustment
       AND to_jsonb(NEW)
           - 'lucky_coin_played'
           - 'lucky_coin_outcome'
           - 'lucky_coin_adjustment'
           - 'lucky_coin_played_at'
           - 'balance_after_lucky'
           - 'user_net_amount'
           - 'system_expense'
           - 'system_profit'
        = to_jsonb(OLD)
           - 'lucky_coin_played'
           - 'lucky_coin_outcome'
           - 'lucky_coin_adjustment'
           - 'lucky_coin_played_at'
           - 'balance_after_lucky'
           - 'user_net_amount'
           - 'system_expense'
           - 'system_profit' THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'zenxiang_liyu_records are immutable';
END;
$$;
