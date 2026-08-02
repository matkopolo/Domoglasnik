-- Source refreshes replace a complete ETN year. Repeat-sale pairs are derived
-- data and must disappear with either referenced sale fact before they are
-- rebuilt later in the same system refresh.
ALTER TABLE analytics.repeat_sale_pairs
    DROP CONSTRAINT IF EXISTS repeat_sale_pairs_transaction_id_fkey,
    DROP CONSTRAINT IF EXISTS repeat_sale_pairs_previous_transaction_id_fkey;

ALTER TABLE analytics.repeat_sale_pairs
    ADD CONSTRAINT repeat_sale_pairs_transaction_id_fkey
        FOREIGN KEY (transaction_id)
        REFERENCES analytics.sale_facts(transaction_id) ON DELETE CASCADE,
    ADD CONSTRAINT repeat_sale_pairs_previous_transaction_id_fkey
        FOREIGN KEY (previous_transaction_id)
        REFERENCES analytics.sale_facts(transaction_id) ON DELETE CASCADE;
