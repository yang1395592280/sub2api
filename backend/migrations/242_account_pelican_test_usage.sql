-- Usage is optional because some upstream streaming protocols omit token counts.
ALTER TABLE account_pelican_tests
    ADD COLUMN IF NOT EXISTS input_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS output_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS total_tokens BIGINT;
