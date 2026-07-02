package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

// fakeUserNoteRepo is an in-memory stand-in for interfaces.UserNoteRepository.
// It embeds the interface so the compiler is satisfied for methods the tests
// never exercise, and overrides the ones the service actually calls.
type fakeUserNoteRepo struct {
	interfaces.UserNoteRepository // nil; unused methods panic if called
	rows                          []*types.UserNote
	createErr                     error
	updateErr                     error
	deleteErr                     error
}

func (r *fakeUserNoteRepo) List(_ context.Context, userID string, tenantID uint64) ([]*types.UserNote, error) {
	var out []*types.UserNote
	for _, n := range r.rows {
		if n.UserID == userID && n.TenantID == tenantID {
			cp := *n
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeUserNoteRepo) ListBySession(_ context.Context, userID string, tenantID uint64, sessionID string) ([]*types.UserNote, error) {
	var out []*types.UserNote
	for _, n := range r.rows {
		if n.UserID == userID && n.TenantID == tenantID && n.SessionID == sessionID {
			cp := *n
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeUserNoteRepo) Get(_ context.Context, userID string, tenantID uint64, id string) (*types.UserNote, error) {
	for _, n := range r.rows {
		if n.ID == id && n.UserID == userID && n.TenantID == tenantID {
			cp := *n
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeUserNoteRepo) Create(_ context.Context, note *types.UserNote) (*types.UserNote, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	cp := *note
	r.rows = append(r.rows, &cp)
	return &cp, nil
}

func (r *fakeUserNoteRepo) Update(_ context.Context, userID string, tenantID uint64, id string, title, content string) (*types.UserNote, error) {
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	for _, n := range r.rows {
		if n.ID == id && n.UserID == userID && n.TenantID == tenantID {
			n.Title = title
			n.Content = content
			cp := *n
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeUserNoteRepo) Delete(_ context.Context, userID string, tenantID uint64, id string) (bool, error) {
	if r.deleteErr != nil {
		return false, r.deleteErr
	}
	for i, n := range r.rows {
		if n.ID == id && n.UserID == userID && n.TenantID == tenantID {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// TestUserNoteService_Create_RejectsEmptyTitle: a blank title is the one
// validation we cannot recover from - the UI shows notes by title, so an
// empty title would render as an invisible row.
func TestUserNoteService_Create_RejectsEmptyTitle(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	for _, title := range []string{"", "   ", "\n\t"} {
		_, err := svc.Create(context.Background(), "u1", 1, types.UserNote{Title: title})
		if !errors.Is(err, ErrNoteEmptyTitle) {
			t.Fatalf("title %q expected ErrNoteEmptyTitle, got %v", title, err)
		}
	}
}

// TestUserNoteService_Create_RejectsTitleTooLong: the column is varchar(255)
// (see migration 000069); a longer title would fail at INSERT time with a
// driver-specific error, so the service pre-validates using rune count to
// support multi-byte CJK titles.
func TestUserNoteService_Create_RejectsTitleTooLong(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	longTitle := strings.Repeat("a", maxNoteTitleLen+1)
	_, err := svc.Create(context.Background(), "u1", 1, types.UserNote{Title: longTitle})
	if !errors.Is(err, ErrNoteTitleTooLong) {
		t.Fatalf("expected ErrNoteTitleTooLong, got %v", err)
	}
}

// TestUserNoteService_Create_RejectsContentTooBig: content is TEXT (65535 in
// MySQL's strict mode); enforcing the cap here gives a stable 400 instead of
// a 500 from the driver.
func TestUserNoteService_Create_RejectsContentTooBig(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	bigContent := strings.Repeat("x", maxNoteContentLen+1)
	_, err := svc.Create(context.Background(), "u1", 1, types.UserNote{Title: "ok", Content: bigContent})
	if !errors.Is(err, ErrNoteContentTooBig) {
		t.Fatalf("expected ErrNoteContentTooBig, got %v", err)
	}
}

// TestUserNoteService_Create_AssignsIDAndScope: on the happy path the service
// mints a UUID and stamps (user, tenant) from the auth context - never from
// the request body - so a caller cannot forge another user's note.
func TestUserNoteService_Create_AssignsIDAndScope(t *testing.T) {
	repo := &fakeUserNoteRepo{}
	svc := NewUserNoteService(repo)

	// The caller tries to spoof user_id/tenant_id; the service must ignore
	// those fields and use the auth-context values instead.
	in := types.UserNote{
		Title:    "my note",
		Content:  "body",
		UserID:   "attacker",
		TenantID: 999,
	}
	note, err := svc.Create(context.Background(), "real-user", 7, in)
	requireNoError(t, err)
	if note.ID == "" {
		t.Fatal("ID must be assigned by the service, not left empty")
	}
	if note.UserID != "real-user" {
		t.Errorf("UserID = %q, want %q (must come from auth context)", note.UserID, "real-user")
	}
	if note.TenantID != 7 {
		t.Errorf("TenantID = %d, want 7", note.TenantID)
	}
	// The persisted row must carry the auth-context scope, not the spoofed one.
	if repo.rows[0].UserID != "real-user" || repo.rows[0].TenantID != 7 {
		t.Errorf("persisted row scope mismatch: %+v", repo.rows[0])
	}
}

// TestUserNoteService_Update_MapsNotFound: gorm.ErrRecordNotFound from the
// repo is translated to ErrNoteNotFound so the handler can return a clean
// 404 without leaking GORM internals to the client.
func TestUserNoteService_Update_MapsNotFound(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	_, err := svc.Update(context.Background(), "u1", 1, "missing", "title", "content")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound, got %v", err)
	}
}

// TestUserNoteService_Update_RejectsEmptyTitle: same validation as Create;
// the handler returns 400 for this, not 500.
func TestUserNoteService_Update_RejectsEmptyTitle(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	_, err := svc.Update(context.Background(), "u1", 1, "any", "   ", "content")
	if !errors.Is(err, ErrNoteEmptyTitle) {
		t.Fatalf("expected ErrNoteEmptyTitle, got %v", err)
	}
}

// TestUserNoteService_Delete_MapsNotFound: when no row is deleted, the
// service returns ErrNoteNotFound so the handler maps to 404 rather than
// the misleading 200 the old favorites path returned.
func TestUserNoteService_Delete_MapsNotFound(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	err := svc.Delete(context.Background(), "u1", 1, "missing")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound, got %v", err)
	}
}

// TestUserNoteService_Delete_HappyPath: when the row exists, Delete returns
// nil and the row is actually removed from the store.
func TestUserNoteService_Delete_HappyPath(t *testing.T) {
	repo := &fakeUserNoteRepo{rows: []*types.UserNote{{ID: "n1", UserID: "u1", TenantID: 1, Title: "t"}}}
	svc := NewUserNoteService(repo)
	requireNoError(t, svc.Delete(context.Background(), "u1", 1, "n1"))
	if len(repo.rows) != 0 {
		t.Fatalf("row should be removed, got %d rows", len(repo.rows))
	}
}

// TestUserNoteService_ListBySession_RejectsEmpty: an empty session_id would
// silently return all notes (since session_id is the only filter), which is
// not what the caller intended - fail fast instead.
func TestUserNoteService_ListBySession_RejectsEmpty(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	_, err := svc.ListBySession(context.Background(), "u1", 1, "")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound for empty session_id, got %v", err)
	}
}

// TestUserNoteService_Get_MapsNotFound: Get on a missing id returns
// ErrNoteNotFound (not the raw gorm error) so the handler stays clean.
func TestUserNoteService_Get_MapsNotFound(t *testing.T) {
	svc := NewUserNoteService(&fakeUserNoteRepo{})
	_, err := svc.Get(context.Background(), "u1", 1, "missing")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound, got %v", err)
	}
}
