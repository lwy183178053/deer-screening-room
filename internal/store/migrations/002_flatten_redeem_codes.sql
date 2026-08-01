ALTER TABLE redeem_codes
    ADD COLUMN credits BIGINT,
    ADD COLUMN created_by BIGINT REFERENCES users(id),
    ADD COLUMN created_at TIMESTAMPTZ;

UPDATE redeem_codes AS code
SET credits = batch.credits,
    created_by = batch.created_by,
    created_at = batch.created_at
FROM redeem_batches AS batch
WHERE batch.id = code.batch_id;

ALTER TABLE redeem_codes
    ALTER COLUMN credits SET NOT NULL,
    ALTER COLUMN created_by SET NOT NULL,
    ALTER COLUMN created_at SET NOT NULL,
    ADD CONSTRAINT redeem_codes_credits_positive CHECK (credits > 0),
    DROP COLUMN batch_id;

DROP TABLE redeem_batches;

CREATE INDEX redeem_codes_created_idx ON redeem_codes(created_at DESC, id DESC);
CREATE INDEX redeem_codes_redeemed_idx ON redeem_codes(redeemed_by, redeemed_at DESC);
