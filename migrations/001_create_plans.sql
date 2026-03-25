CREATE TABLE IF NOT EXISTS plans (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    amount      TEXT NOT NULL,
    currency    TEXT NOT NULL,
    interval    TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);
