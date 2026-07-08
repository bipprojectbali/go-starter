-- name: ListTodos :many
-- Keyset pagination. Halaman pertama: kirim cursor (created_at, id) = nilai maksimum
-- ('infinity'::timestamptz, maxint) agar seluruh baris memenuhi syarat.
-- Halaman berikutnya: kirim (created_at, id) baris TERAKHIR yang tampil.
-- Cast eksplisit ::bigint pada cursor_id: tanpa ini sqlc salah infer tipe cursor_id
-- (mengikuti created_at) dari row-value comparison. Cast memaksa int64.
SELECT * FROM todos
WHERE user_id = $1
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: CreateTodo :one
INSERT INTO todos (user_id, title)
VALUES ($1, $2)
RETURNING *;

-- name: GetTodo :one
SELECT * FROM todos WHERE id = $1 AND user_id = $2;

-- name: DeleteTodo :exec
-- Authz ownership: filter user_id, bukan cuma id.
DELETE FROM todos WHERE id = $1 AND user_id = $2;
