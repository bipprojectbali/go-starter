package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_stater/internal/db"
)

func TestTodoCreate_Success(t *testing.T) {
	env, uid := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"beli kopi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.doAuthed(uid, req, env.h.TodoCreate)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "beli kopi") {
		t.Errorf("response tidak memuat judul todo:\n%s", rec.Body.String())
	}

	// Verifikasi tersimpan di DB milik user yang benar.
	curAt, curID := firstPageCursor()
	todos, err := env.h.DB.ListTodos(req.Context(), db.ListTodosParams{
		UserID:          uid,
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
	env, uid := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.doAuthed(uid, req, env.h.TodoCreate)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "wajib diisi") {
		t.Errorf("response tidak memuat pesan validasi:\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `class="todo-item"`) {
		t.Errorf("todo item seharusnya TIDAK dibuat saat validasi gagal")
	}
}

func TestTodoList_RendersFullPage(t *testing.T) {
	env, uid := setupTest(t)
	// Buat satu todo lewat handler.
	create := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader(`{"title":"tugas A"}`))
	create.Header.Set("Content-Type", "application/json")
	env.doAuthed(uid, create, env.h.TodoCreate)

	req := httptest.NewRequest(http.MethodGet, "/todos", nil)
	rec := env.doAuthed(uid, req, env.h.TodoList)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<!doctype html>", "datastar.js", "tugas A", `id="todo-list"`} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman tidak memuat %q", want)
		}
	}
}

// TestTodoDelete_OwnershipEnforced memastikan user tidak bisa hapus todo milik
// orang lain (authz ownership via user_id di query).
func TestTodoDelete_OwnershipEnforced(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	// User kedua + todo miliknya.
	other, err := env.h.DB.CreateUser(ctx, db.CreateUserParams{Email: "other@local", PassHash: "x"})
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	otherTodo, err := env.h.DB.CreateTodo(ctx, db.CreateTodoParams{UserID: other.ID, Title: "rahasia"})
	if err != nil {
		t.Fatalf("create other todo: %v", err)
	}

	// uid mencoba hapus todo milik other → tidak boleh terhapus.
	req := httptest.NewRequest(http.MethodDelete, "/todos/1", nil)
	req.SetPathValue("id", "1")
	env.doAuthed(uid, withChiParam(req, "id", itoa(otherTodo.ID)), env.h.TodoDelete)

	// Todo other harus masih ada.
	got, err := env.h.DB.GetTodo(ctx, db.GetTodoParams{ID: otherTodo.ID, UserID: other.ID})
	if err != nil {
		t.Fatalf("todo milik other seharusnya masih ada, tapi hilang: %v", err)
	}
	if got.ID != otherTodo.ID {
		t.Errorf("todo other berubah tak terduga")
	}
}
