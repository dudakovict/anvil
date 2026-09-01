-- +goose Up
-- Sequential PK by design: random UUIDs (v4) as primary keys fragment btree
-- indexes and bloat WAL. Use bigint identity; if globally-unique IDs are
-- required, use UUIDv7 (time-ordered, index-friendly).
CREATE TABLE users (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
