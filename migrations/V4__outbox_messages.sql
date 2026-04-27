CREATE TABLE outbox_messages
(
    id            BIGSERIAL PRIMARY KEY,
    topic         TEXT        NOT NULL,
    message_key   BYTEA,
    payload       BYTEA       NOT NULL,

    status        TEXT        NOT NULL DEFAULT 'pending',
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at  TIMESTAMPTZ,

    CONSTRAINT chk_outbox_status
        CHECK (status IN ('pending', 'published', 'failed'))
);

CREATE INDEX idx_outbox_messages_pending
    ON outbox_messages (id)
    WHERE status = 'pending';