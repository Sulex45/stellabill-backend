CREATE TABLE IF NOT EXISTS subscriptions (
    id           TEXT        PRIMARY KEY,
    plan_id      TEXT        NOT NULL,
    customer_id  TEXT        NOT NULL,
    status       TEXT        NOT NULL,
    amount       TEXT        NOT NULL,
    currency     TEXT        NOT NULL,
    interval     TEXT        NOT NULL,
    next_billing TEXT        NOT NULL DEFAULT '',
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_customer_id ON subscriptions (customer_id);
