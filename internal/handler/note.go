package handler

import (
	stderrors "errors"
	"net/http"

	"github.com/Tencent/XinWiki/internal/application/service"
	apperrors "github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// UserNoteHandler exposes the per-user notes API (NotebookLM-style "Notes"
// surface in the Workspace).
//
// Authorization model mirrors UserResourceFavoriteHandler: a user can only
// manipulate their own notes in the tenant they're currently scoped into.
// (userID, tenantID) are always taken from the auth context, never from
// the request body or query params.
type UserNoteHandler struct {
	service interfaces.UserNoteService
}

func NewUserNoteHandler(svc interfaces.UserNoteService) *UserNoteHandler {
	return &UserNoteHandler{service: svc}
}

// noteContext resolves the (userID, tenantID) pair the handler will scope
// all queries to. Centralised so the five endpoints stay short and
// consistent in their error shape.
func noteContext(c *gin.Context) (string, uint64, bool) {
	uidVal, ok := c.Get(types.UserIDContextKey.String())
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("user ID not found"))
		return "", 0, false
	}
	userID, _ := uidVal.(string)
	if userID == "" {
		c.Error(apperrors.NewUnauthorizedError("user ID not found"))
		return "", 0, false
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	if tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("tenant ID not found"))
		return "", 0, false
	}
	return userID, tenantID, true
}

// ListNotes godoc
// @Summary      List my notes
// @Description  Lists this user's notes in the current tenant, newest first
// @Tags         User
// @Param        session_id  query     string  false  "Filter by chat session"
// @Success      200   {object}  map[string]interface{}
// @Router       /user/notes [get]
func (h *UserNoteHandler) ListNotes(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := noteContext(c)
	if !ok {
		return
	}
	sessionID := c.Query("session_id")

	var (
		list []*types.UserNote
		err  error
	)
	if sessionID != "" {
		list, err = h.service.ListBySession(ctx, userID, tenantID, sessionID)
	} else {
		list, err = h.service.List(ctx, userID, tenantID)
	}
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// GetNote godoc
// @Summary      Get a note
// @Tags         User
// @Param        id    path      string  true  "Note id"
// @Success      200   {object}  map[string]interface{}
// @Router       /user/notes/{id} [get]
func (h *UserNoteHandler) GetNote(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := noteContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	note, err := h.service.Get(ctx, userID, tenantID, id)
	if err != nil {
		if stderrors.Is(err, service.ErrNoteNotFound) {
			c.Error(apperrors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": note})
}

// CreateNoteRequest is the body for POST /user/notes. The source_* fields
// are populated when the user saves a cited excerpt from chat; they're
// empty for hand-written notes.
type CreateNoteRequest struct {
	Title         string `json:"title"`
	Content       string `json:"content"`
	SessionID     string `json:"session_id,omitempty"`
	SourceExcerpt string `json:"source_excerpt,omitempty"`
	SourceRefID   string `json:"source_ref_id,omitempty"`
	SourceTitle   string `json:"source_title,omitempty"`
	SourceURL     string `json:"source_url,omitempty"`
}

// CreateNote godoc
// @Summary      Create a note
// @Tags         User
// @Param        body  body      CreateNoteRequest  true  "Note payload"
// @Success      200   {object}  map[string]interface{}
// @Router       /user/notes [post]
func (h *UserNoteHandler) CreateNote(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := noteContext(c)
	if !ok {
		return
	}
	var req CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid request body").WithDetails(err.Error()))
		return
	}
	in := types.UserNote{
		Title:         req.Title,
		Content:       req.Content,
		SessionID:     req.SessionID,
		SourceExcerpt: req.SourceExcerpt,
		SourceRefID:   req.SourceRefID,
		SourceTitle:   req.SourceTitle,
		SourceURL:     req.SourceURL,
	}
	note, err := h.service.Create(ctx, userID, tenantID, in)
	if err != nil {
		if isNoteValidationError(err) {
			c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": note})
}

// UpdateNoteRequest is the body for PUT /user/notes/:id. Only title and
// content are mutable; source_* fields are fixed at creation time.
type UpdateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// UpdateNote godoc
// @Summary      Update a note
// @Tags         User
// @Param        id    path      string             true  "Note id"
// @Param        body  body      UpdateNoteRequest  true  "Note payload"
// @Success      200   {object}  map[string]interface{}
// @Router       /user/notes/{id} [put]
func (h *UserNoteHandler) UpdateNote(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := noteContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	var req UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid request body").WithDetails(err.Error()))
		return
	}
	note, err := h.service.Update(ctx, userID, tenantID, id, req.Title, req.Content)
	if err != nil {
		if stderrors.Is(err, service.ErrNoteNotFound) {
			c.Error(apperrors.NewNotFoundError(err.Error()))
			return
		}
		if isNoteValidationError(err) {
			c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": note})
}

// DeleteNote godoc
// @Summary      Delete a note
// @Tags         User
// @Param        id    path      string  true  "Note id"
// @Success      200   {object}  map[string]interface{}
// @Router       /user/notes/{id} [delete]
func (h *UserNoteHandler) DeleteNote(c *gin.Context) {
	ctx := c.Request.Context()
	userID, tenantID, ok := noteContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if err := h.service.Delete(ctx, userID, tenantID, id); err != nil {
		if stderrors.Is(err, service.ErrNoteNotFound) {
			c.Error(apperrors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// isNoteValidationError returns true for the sentinel validation errors
// defined in the service layer, so the handler can map them to 400 without
// listing each one inline.
func isNoteValidationError(err error) bool {
	return stderrors.Is(err, service.ErrNoteEmptyTitle) ||
		stderrors.Is(err, service.ErrNoteTitleTooLong) ||
		stderrors.Is(err, service.ErrNoteContentTooBig)
}
