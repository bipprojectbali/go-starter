-- +goose Up
-- +goose StatementBegin

-- audit_logs.target_type dulu ditulis "user" untuk SEMUA aksi (h.audit
-- meng-hardcode-nya), padahal tujuh aksi mengirim tenants.id sebagai target_id.
-- Selama jejaknya tak pernah dibaca, kekeliruan itu tak terasa. Begitu panel
-- menampilkannya dengan meng-JOIN nama berdasarkan target_id, ia berubah jadi
-- kebohongan yang meyakinkan: "workspace.create" akan menampilkan NAMA ORANG
-- yang id-nya kebetulan sama dengan id workspace.
--
-- Diperbaiki dari `action`, bukan ditebak dari data: nama aksinya sendiri yang
-- menentukan jenis sasaran, dan itu tak berubah oleh baris mana pun.
UPDATE audit_logs SET target_type = 'workspace'
WHERE target_type = 'user'
  AND action IN (
      'workspace.create', 'workspace.rename', 'workspace.archive',
      'workspace.unarchive', 'workspace.delete', 'workspace.suspend',
      'invite.create', 'invite.accept'
  );

UPDATE audit_logs SET target_type = 'platform'
WHERE target_type = 'user'
  AND action IN ('settings.workspace_quota', 'platform.tenancy.upgrade');

-- Index untuk panel jejak aktivitas. Tanpa ini tiap penyaringan memindai seluruh
-- tabel — dan tabel ini append-only yang tumbuh selamanya, jadi biayanya naik
-- terus sementara halamannya justru makin sering dibuka.
--
-- created_at DESC + id DESC menyamai ORDER BY halaman, sehingga cursor keyset
-- berjalan di atas index (bukan sort ulang). idx_audit_actor yang sudah ada
-- menangani penyaringan per-orang.
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action  ON audit_logs (action, created_at DESC);

COMMENT ON COLUMN audit_logs.target_type IS
    'Jenis sasaran: user | workspace | platform | session. Menentukan tabel mana '
    'yang boleh di-JOIN untuk mencari nama saat jejak ditampilkan — salah '
    'menyebutnya menghasilkan nama yang keliru, bukan sekadar label yang keliru.';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_audit_action;
DROP INDEX IF EXISTS idx_audit_created;
-- target_type TIDAK dikembalikan ke 'user': nilai lamanya keliru, dan memulihkan
-- kekeliruan bukan tugas rollback.
-- +goose StatementEnd
