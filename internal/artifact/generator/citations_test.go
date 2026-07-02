package generator

import (
	"testing"

	"github.com/Tencent/XinWiki/internal/types"

	"github.com/stretchr/testify/assert"
)

func TestBuildCitationsFromWikiPage_NilPage(t *testing.T) {
	got := BuildCitationsFromWikiPage(nil)
	assert.Nil(t, got, "nil page should yield nil citations, not an empty slice")
}

func TestBuildCitationsFromWikiPage_MinimalPage(t *testing.T) {
	page := &types.WikiPage{ID: "wp-1", Title: "RAG 概览"}
	got := BuildCitationsFromWikiPage(page)
	if assert.Len(t, got, 1) {
		c := got[0]
		assert.Equal(t, 1, c.ID)
		assert.Equal(t, "RAG 概览", c.Title)
		assert.Equal(t, "wiki_page", c.Type)
		assert.Equal(t, "wp-1", c.RefID)
		assert.Empty(t, c.URL, "URL must be empty when Slug is empty")
	}
}

func TestBuildCitationsFromWikiPage_WithSlug(t *testing.T) {
	page := &types.WikiPage{
		ID:              "wp-2",
		Title:           "Agent ReAct",
		KnowledgeBaseID: "kb-7",
		Slug:            "concept/react",
	}
	got := BuildCitationsFromWikiPage(page)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "/wiki/kb-7/concept/react", got[0].URL)
	}
}

func TestBuildCitationsFromWikiPage_EmptyIDStillProducesCitation(t *testing.T) {
	// Defensive: an empty-ID page should still surface as a citation so the
	// frontend can render "来源 1" rather than dropping the reference.
	page := &types.WikiPage{Title: "Untitled"}
	got := BuildCitationsFromWikiPage(page)
	assert.Len(t, got, 1)
	assert.Equal(t, "", got[0].RefID)
}
