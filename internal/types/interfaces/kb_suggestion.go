// Package interfaces defines the interface contracts between different system components.
// This file defines the KnowledgeBaseSuggestionService interface, which provides
// KB-level suggested questions for the NotebookLM-style "Notebook Guide" feature.
// Unlike the agent-level suggested-questions endpoint (which scopes to an agent's
// configured KBs), this service scopes to a single knowledge base and is called
// after the handler has already resolved cross-tenant access via
// KnowledgeBaseHandler.validateAndGetKnowledgeBase.
package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

// KnowledgeBaseSuggestionService generates NotebookLM-style "Notebook Guide"
// suggested questions for a single knowledge base. Questions are sourced from
// three buckets - FAQ recommended chunks, document chunks with AI-generated
// questions, and wiki page titles - then round-robin sampled for diversity.
//
// The caller is responsible for access control: the effectiveTenantID passed
// in must already be the resolved source tenant (owner tenant for shared KBs),
// typically obtained from KnowledgeBaseHandler.validateAndGetKnowledgeBase.
type KnowledgeBaseSuggestionService interface {
	// GetSuggestedQuestions returns up to `limit` suggested questions for the
	// given knowledge base. When knowledgeIDs is non-empty, only chunks
	// belonging to those knowledge items are considered. A limit <= 0 defaults
	// to 6.
	GetSuggestedQuestions(
		ctx context.Context,
		kbID string,
		effectiveTenantID uint64,
		knowledgeIDs []string,
		limit int,
	) ([]types.SuggestedQuestion, error)
}
