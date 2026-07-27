-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    -- hash argon2id (bukan bcrypt) — lihat STARTER §2.1 #13
    pass_hash  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
