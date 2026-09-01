-- +goose Up
ALTER TABLE users
    ADD COLUMN role text NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user'));

-- +goose Down
ALTER TABLE users
    DROP COLUMN role;
