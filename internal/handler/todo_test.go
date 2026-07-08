package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"go_stater/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTest menyiapkan Handler terhubung ke TEST_DATABASE_URL + user seed.
// Skip bila env tidak di-set (mis. CI tanpa Postgres).
func setupTest(t *testing.T) (*Handler, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	q := db.New(pool)
	// Bersihkan & seed user deterministik.
	if _, err := pool.Exec(ctx, "TRUNCATE todos, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	u, err := q.CreateUser(ctx, db.CreateUserParams{Email: "test@local", PassHash: "x"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	h := &Handler{DB: q, Pool: pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return h, u.ID
}

// override spikeUserID lewat pembuatan todo langsung memakai user seed.
// spikeUserID const = 1; RESTART IDENTITY membuat user seed juga id=1.
func TestTodoCreate_Success(t *testing.T) {
	h, uid := setupTest(t)
	if uid != spikeUserID {
		t.Fatalf("seed user id=%d, expected %d (spikeUserID)", uid, spikeUserID)
	}

	body := strings.NewReader(`{"title":"beli kopi"}`)
	req := httptest.NewRequest(http.MethodPost, "/todos", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.TodoCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Response SSE harus memuat fragment item baru dengan judulnya.
	if !strings.Contains(rec.Body.String(), "beli kopi") {
		t.Errorf("response tidak memuat judul todo:\n%s", rec.Body.String())
	}

	// Verifikasi tersimpan di DB (pakai cursor halaman pertama yang sama dgn handler).
	curAt, curID := firstPageCursor()
	todos, err := h.DB.ListTodos(req.Context(), db.ListTodosParams{
		UserID:          spikeUserID,
		CursorCreatedAt: curAt,
		CursorID:        curID,
		PageSize:        10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(todos) != 1 || todos[0].Title != "beli kopi" {
		t.Errorf("want 1 todo 'beli kopi', got %+v", todos)
	}
}

func TestTodoCreate_EmptyTitle(t *testing.T) {
	h, _ := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.TodoCreate(rec, req)

	// Validasi gagal tetap 200 (SSE), tapi memuat alert error, bukan item.
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "wajib diisi") {
		t.Errorf("response tidak memuat pesan validasi:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "todo-item") {
		t.Errorf("todo item seharusnya TIDAK dibuat saat validasi gagal")
	}
}

func TestTodoList_RendersFullPage(t *testing.T) {
	h, _ := setupTest(t)
	// Buat satu todo lewat handler agar ada data.
	create := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"tugas A"}`))
	create.Header.Set("Content-Type", "application/json")
	h.TodoCreate(httptest.NewRecorder(), create)

	req := httptest.NewRequest(http.MethodGet, "/todos", nil)
	rec := httptest.NewRecorder()
	h.TodoList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Full page harus punya <!doctype html> + Datastar script + item.
	for _, want := range []string{"<!doctype html>", "datastar.js", "tugas A", `id="todo-list"`} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman tidak memuat %q", want)
		}
	}
}
