package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors so the handler can map cleanly to HTTP status codes
// (400 for validation, 404 for not-found) without leaking GORM internals.
var (
	ErrNoteEmptyTitle    = errors.New("note title is required")
	ErrNoteTitleTooLong  = errors.New("note title exceeds 255 characters")
	ErrNoteContentTooBig = errors.New("note content exceeds 65535 characters")
	ErrNoteNotFound      = errors.New("note not found")
)

const (
	maxNoteTitleLen   = 255
	maxNoteContentLen = 65535
)

type userNoteService struct {
	repo interfaces.UserNoteRepository
}

// NewUserNoteService wraps the repository with input validation and UUID
// generation. The handler stays thin; all business rules live here.
func NewUserNoteService(repo interfaces.UserNoteRepository) interfaces.UserNoteService {
	return &userNoteService{repo: repo}
}

func (s *userNoteService) List(
	ctx context.Context, userID string, tenantID uint64,
) ([]*types.UserNote, error) {
	return s.repo.List(ctx, userID, tenantID)
}

func (s *userNoteService) ListBySession(
	ctx context.Context, userID string, tenantID uint64, sessionID string,
) ([]*types.UserNote, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrNoteNotFound
	}
	return s.repo.ListBySession(ctx, userID, tenantID, sessionID)
}

func (s *userNoteService) Get(
	ctx context.Context, userID string, tenantID uint64, id string,
) (*types.UserNote, error) {
	note, err := s.repo.Get(ctx, userID, tenantID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	return note, nil
}

func (s *userNoteService) Create(
	ctx context.Context, userID string, tenantID uint64, in types.UserNote,
) (*types.UserNote, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, ErrNoteEmptyTitle
	}
	if utf8.RuneCountInString(in.Title) > maxNoteTitleLen {
		return nil, ErrNoteTitleTooLong
	}
	if utf8.RuneCountInString(in.Content) > maxNoteContentLen {
		return nil, ErrNoteContentTooBig
	}
	in.ID = uuid.NewString()
	in.UserID = userID
	in.TenantID = tenantID
	return s.repo.Create(ctx, &in)
}

func (s *userNoteService) Update(
	ctx context.Context, userID string, tenantID uint64, id string, title, content string,
) (*types.UserNote, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrNoteEmptyTitle
	}
	if utf8.RuneCountInString(title) > maxNoteTitleLen {
		return nil, ErrNoteTitleTooLong
	}
	if utf8.RuneCountInString(content) > maxNoteContentLen {
		return nil, ErrNoteContentTooBig
	}
	note, err := s.repo.Update(ctx, userID, tenantID, id, title, content)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	return note, nil
}

func (s *userNoteService) Delete(
	ctx context.Context, userID string, tenantID uint64, id string,
) error {
	deleted, err := s.repo.Delete(ctx, userID, tenantID, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNoteNotFound
	}
	return nil
}
