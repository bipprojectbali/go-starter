-- +goose Up
-- +goose StatementBegin
-- Fitur Todos (contoh starter spike) dihapus total. Tabel tak direferensi FK dari
-- tabel lain, jadi drop aman. IF EXISTS = idempotent.
DROP TABLE IF EXISTS todos;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reversible: re-create identik skema 00002 (tabel + index keyset pagination).
CREATE TABLE todos (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    done       BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_todos_user_created ON todos (user_id, created_at DESC, id DESC);
-- +goose StatementEnd
