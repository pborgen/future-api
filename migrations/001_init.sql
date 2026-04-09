CREATE TABLE IF NOT EXISTS appointments (
    id          BIGSERIAL PRIMARY KEY,
    trainer_id  BIGINT      NOT NULL,
    user_id     BIGINT      NOT NULL,
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A trainer cannot have two appointments starting at the same instant.
-- Combined with our fixed 30-minute slots aligned to :00/:30, this is
-- sufficient to enforce non-overlap at the database layer.
CREATE UNIQUE INDEX IF NOT EXISTS appointments_trainer_starts_at_key
    ON appointments (trainer_id, starts_at);

CREATE INDEX IF NOT EXISTS appointments_trainer_idx
    ON appointments (trainer_id, starts_at);
