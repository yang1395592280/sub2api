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
           (NEW.lucky_coin_outcome = 'double'
                AND NEW.lucky_coin_adjustment = OLD.reward_amount)
           OR (NEW.lucky_coin_outcome = 'zero'
                AND NEW.lucky_coin_adjustment = ROUND(-2 * OLD.reward_amount, 8))
       )
       AND OLD.lucky_coin_played_at IS NULL
       AND NEW.lucky_coin_played_at IS NOT NULL
       AND OLD.balance_after_lucky IS NULL
       AND NEW.balance_after_lucky IS NOT NULL
       AND NEW.user_net_amount = OLD.user_net_amount + NEW.lucky_coin_adjustment
       AND NEW.system_expense = OLD.system_expense + NEW.lucky_coin_adjustment
       AND NEW.system_profit = OLD.system_profit - NEW.lucky_coin_adjustment
       AND to_jsonb(NEW)
           - 'lucky_coin_played' - 'lucky_coin_outcome' - 'lucky_coin_adjustment'
           - 'lucky_coin_played_at' - 'balance_after_lucky'
           - 'user_net_amount' - 'system_expense' - 'system_profit'
        = to_jsonb(OLD)
           - 'lucky_coin_played' - 'lucky_coin_outcome' - 'lucky_coin_adjustment'
           - 'lucky_coin_played_at' - 'balance_after_lucky'
           - 'user_net_amount' - 'system_expense' - 'system_profit' THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.lucky_coin_played = TRUE
       AND OLD.guess_size_played = FALSE
       AND NEW.guess_size_played = TRUE
       AND OLD.guess_size_choice = ''
       AND NEW.guess_size_choice IN ('big', 'small', 'skip')
       AND OLD.guess_size_outcome = ''
       AND NEW.guess_size_outcome IN ('big', 'small', 'skipped')
       AND OLD.guess_size_adjustment = 0
       AND OLD.guess_big_probability_snapshot = 0
       AND OLD.guess_small_probability_snapshot = 0
       AND NEW.guess_big_probability_snapshot >= 0
       AND NEW.guess_small_probability_snapshot >= 0
       AND NEW.guess_big_probability_snapshot + NEW.guess_small_probability_snapshot = 100
       AND OLD.guess_size_played_at IS NULL
       AND NEW.guess_size_played_at IS NOT NULL
       AND OLD.balance_after_guess_size IS NULL
       AND NEW.balance_after_guess_size IS NOT NULL
       AND (
           (NEW.guess_size_choice = 'skip'
                AND NEW.guess_size_outcome = 'skipped'
                AND NEW.guess_size_won = FALSE
                AND NEW.guess_size_adjustment = 0)
           OR (NEW.guess_size_choice IN ('big', 'small')
                AND NEW.guess_size_outcome IN ('big', 'small')
                AND NEW.guess_size_won = (NEW.guess_size_choice = NEW.guess_size_outcome)
                AND (
                    (OLD.lucky_coin_outcome = 'double' AND NEW.guess_size_won = TRUE
                        AND NEW.guess_size_adjustment = ROUND(2 * OLD.reward_amount, 8))
                    OR (OLD.lucky_coin_outcome = 'double' AND NEW.guess_size_won = FALSE
                        AND NEW.guess_size_adjustment = ROUND(-6 * OLD.reward_amount, 8))
                    OR (OLD.lucky_coin_outcome = 'zero' AND NEW.guess_size_won = TRUE
                        AND NEW.guess_size_adjustment = ROUND(-(OLD.reward_amount + OLD.lucky_coin_adjustment), 8))
                    OR (OLD.lucky_coin_outcome = 'zero' AND NEW.guess_size_won = FALSE
                        AND NEW.guess_size_adjustment = 0)
                ))
       )
       AND NEW.user_net_amount = OLD.user_net_amount + NEW.guess_size_adjustment
       AND NEW.system_expense = OLD.system_expense + NEW.guess_size_adjustment
       AND NEW.system_profit = OLD.system_profit - NEW.guess_size_adjustment
       AND to_jsonb(NEW)
           - 'guess_size_played' - 'guess_size_choice' - 'guess_size_outcome'
           - 'guess_size_won' - 'guess_size_adjustment'
           - 'guess_big_probability_snapshot' - 'guess_small_probability_snapshot'
           - 'guess_size_played_at' - 'balance_after_guess_size'
           - 'user_net_amount' - 'system_expense' - 'system_profit'
        = to_jsonb(OLD)
           - 'guess_size_played' - 'guess_size_choice' - 'guess_size_outcome'
           - 'guess_size_won' - 'guess_size_adjustment'
           - 'guess_big_probability_snapshot' - 'guess_small_probability_snapshot'
           - 'guess_size_played_at' - 'balance_after_guess_size'
           - 'user_net_amount' - 'system_expense' - 'system_profit' THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'zenxiang_liyu_records are immutable';
END;
$$;
