package handler

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"go_stater/internal/db"
	"go_stater/internal/ui"
	"go_stater/internal/ui/pages"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// firstPageCursor mengembalikan cursor keyset untuk halaman pertama:
// (created_at, id) = maksimum, sehingga semua baris lolos syarat < cursor.
func firstPageCursor() (pgtype.Timestamptz, int64) {
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}, math.MaxInt64
}

// TodoList — GET /todos. Full page (§4.4 jalur 1).
func (h *Handler) TodoList(w http.ResponseWriter, r *http.Request) {
	cursorAt, cursorID := firstPageCursor()
	todos, err := h.DB.ListTodos(r.Context(), db.ListTodosParams{
		UserID:          spikeUserID, // SPIKE: ganti session.UserID(ctx) di Fase 4
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize,
	})
	if err != nil {
		h.Log.Error("list todos", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	renderPage(w, r, h.Log, "Todos", pages.TodoList(todos))
}

// TodoCreate — POST /todos. Aksi Datastar: balas fragment via SSE.
func (h *Handler) TodoCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		// Validasi di boundary: patch alert error (id "todo-error") + kosongkan sinyal error lain.
		patch(w, r, h.Log, ui.Alert(ui.VariantDestructive, "todo-error", g.Text("Judul wajib diisi")))
		return
	}

	todo, err := h.DB.CreateTodo(r.Context(), db.CreateTodoParams{
		UserID: spikeUserID,
		Title:  strings.TrimSpace(in.Title),
	})
	if err != nil {
		h.Log.Error("create todo", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Prepend item baru ke #todo-list, bersihkan alert, reset sinyal $title.
	sse := datastar.NewSSE(w, r)
	var sb strings.Builder
	if err := pages.TodoItem(todo).Render(&sb); err != nil {
		h.Log.Error("render todo item", "err", err)
		return
	}
	if err := sse.PatchElements(sb.String(), datastar.WithSelector("#todo-list"), datastar.WithModePrepend()); err != nil {
		h.Log.Error("patch todo", "err", err)
		return
	}
	_ = sse.MarshalAndPatchSignals(map[string]any{"title": ""})
}

// TodoDelete — DELETE /todos/{id}. Authz ownership via user_id di query.
func (h *Handler) TodoDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.DB.DeleteTodo(r.Context(), db.DeleteTodoParams{
		ID:     id,
		UserID: spikeUserID, // ownership: hanya hapus milik user ini
	}); err != nil {
		h.Log.Error("delete todo", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Hapus elemen dari DOM.
	sse := datastar.NewSSE(w, r)
	_ = sse.RemoveElement("#todo-" + strconv.FormatInt(id, 10))
}
