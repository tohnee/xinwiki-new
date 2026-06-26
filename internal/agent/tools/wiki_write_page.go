package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appsvc "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/sirupsen/logrus"
)

type wikiWritePageTool struct {
	BaseTool
	wikiPageService  interfaces.WikiPageService
	knowledgeService interfaces.KnowledgeService
	chunkService     interfaces.ChunkService
	kbIDs            []string
}

// NewWikiWritePageTool creates a new wiki_write_page tool
func NewWikiWritePageTool(wikiPageService interfaces.WikiPageService, kbIDs []string, knowledgeService interfaces.KnowledgeService, chunkService interfaces.ChunkService) types.Tool {
	return &wikiWritePageTool{
		BaseTool: NewBaseTool(
			ToolWikiWritePage,
			"Create a new Wiki page or completely overwrite an existing one. Automatically handles outbound links.",
			json.RawMessage(`{
				"type": "object",
				"properties": {
					"slug": {
						"type": "string",
						"description": "The slug of the Wiki page (e.g. 'entity/hunyuan-damoxing')"
					},
					"title": {
						"type": "string",
						"description": "The title of the page"
					},
					"summary": {
						"type": "string",
						"description": "A one-sentence summary for the index listing"
					},
					"content": {
						"type": "string",
						"description": "The FULL, complete Markdown content of the page. Do NOT use placeholders."
					},
					"page_type": {
						"type": "string",
						"description": "The page type, e.g., 'summary', 'entity', 'concept', 'synthesis', 'comparison'"
					},
					"aliases": {
						"type": "array",
						"items": {"type": "string"},
						"description": "A list of aliases for the page (optional)"
					},
					"source_refs": {
						"type": "array",
						"items": {"type": "string"},
						"description": "A list of source knowledge IDs (UUIDs only) that contributed to this page. If provided, these will COMPLETELY REPLACE the existing source_refs of the page."
					}
				},
				"required": ["slug", "title", "summary", "content", "page_type"]
			}`),
		),
		wikiPageService:  wikiPageService,
		knowledgeService: knowledgeService,
		chunkService:     chunkService,
		kbIDs:            kbIDs,
	}
}

func (t *wikiWritePageTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var params struct {
		Slug       string   `json:"slug"`
		Title      string   `json:"title"`
		Summary    string   `json:"summary"`
		Content    string   `json:"content"`
		PageType   string   `json:"page_type"`
		Aliases    []string `json:"aliases"`
		SourceRefs []string `json:"source_refs"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return &types.ToolResult{Success: false, Error: "Failed to parse arguments: " + err.Error()}, nil
	}

	if len(t.kbIDs) == 0 {
		return &types.ToolResult{Success: false, Error: "No knowledge bases available for editing"}, nil
	}
	kbID := t.kbIDs[0]

	if params.Title == "" || params.PageType == "" || params.Content == "" || params.Summary == "" {
		return &types.ToolResult{Success: false, Error: "title, summary, content, and page_type are required for write action"}, nil
	}

	// Try to get the existing page
	existingPage, err := t.wikiPageService.GetPageBySlug(ctx, kbID, params.Slug)
	if err != nil && !errors.Is(err, repository.ErrWikiPageNotFound) {
		return &types.ToolResult{Success: false, Error: "Failed to check existing page: " + err.Error()}, nil
	}

	resolvedRefs := resolveSourceRefs(ctx, t.knowledgeService, params.SourceRefs)

	var derivedACL *types.DerivedACLResult
	if len(resolvedRefs) > 0 && t.chunkService != nil {
		derivedACL = t.computeDerivedACL(ctx, resolvedRefs)
	}

	var action string
	if existingPage != nil {
		// Update
		existingPage.Title = params.Title
		existingPage.Summary = params.Summary
		existingPage.Content = params.Content
		existingPage.PageType = params.PageType
		existingPage.Aliases = params.Aliases

		if len(resolvedRefs) > 0 {
			existingPage.SourceRefs = resolvedRefs
		}

		if derivedACL != nil {
			appsvc.ApplyDerivedACLToWikiPage(existingPage, *derivedACL)
		}

		_, err = t.wikiPageService.UpdatePage(ctx, existingPage)
		if err != nil {
			return &types.ToolResult{Success: false, Error: "Failed to update page: " + err.Error()}, nil
		}
		action = "updated"
	} else {
		// Create
		newPage := &types.WikiPage{
			KnowledgeBaseID: kbID,
			Slug:            params.Slug,
			Title:           params.Title,
			Summary:         params.Summary,
			Content:         params.Content,
			PageType:        params.PageType,
			Aliases:         params.Aliases,
			SourceRefs:      resolvedRefs,
		}

		if derivedACL != nil {
			appsvc.ApplyDerivedACLToWikiPage(newPage, *derivedACL)
		}

		_, err = t.wikiPageService.CreatePage(ctx, newPage)
		if err != nil {
			return &types.ToolResult{Success: false, Error: "Failed to create page: " + err.Error()}, nil
		}
		action = "created"
	}

	// Inject cross-links so other pages know about this new/updated entity
	t.wikiPageService.InjectCrossLinks(ctx, kbID, []string{params.Slug})

	// Rebuild the index page to reflect the new/updated summary
	_ = t.wikiPageService.RebuildIndexPage(ctx, kbID)

	output := fmt.Sprintf("Successfully %s page [[%s]].\n- Title: %s\n- Type: %s\n- Summary: %s\n- Content length: %d chars", action, params.Slug, params.Title, params.PageType, params.Summary, len(params.Content))
	if len(params.Aliases) > 0 {
		output += fmt.Sprintf("\n- Aliases: %s", strings.Join(params.Aliases, ", "))
	}
	if len(resolvedRefs) > 0 {
		output += fmt.Sprintf("\n- Source refs: %d document(s)", len(resolvedRefs))
	}

	data := map[string]interface{}{
		"display_type": "wiki_write_page",
		"action":       action,
		"slug":         params.Slug,
		"title":        params.Title,
		"page_type":    params.PageType,
		"summary":      params.Summary,
	}

	if derivedACL != nil {
		output += fmt.Sprintf("\n- Derived security level: %s", derivedACL.SecurityLevel)
		output += fmt.Sprintf("\n- Derived allowed users: %d", len(derivedACL.AllowedUserIDs))
		output += fmt.Sprintf("\n- Derived allowed groups: %d", len(derivedACL.AllowedGroupIDs))
		data["security_level"] = derivedACL.SecurityLevel
		data["allowed_user_ids"] = derivedACL.AllowedUserIDs
		data["allowed_group_ids"] = derivedACL.AllowedGroupIDs
	}

	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data:    data,
	}, nil
}

func (t *wikiWritePageTool) computeDerivedACL(ctx context.Context, sourceRefs []string) *types.DerivedACLResult {
	knowledgeIDs := appsvc.ExtractKnowledgeIDsFromSourceRefs(sourceRefs)
	if len(knowledgeIDs) == 0 {
		return nil
	}

	allChunks := make([]*types.Chunk, 0)
	for _, kid := range knowledgeIDs {
		chunks, err := t.chunkService.ListChunksByKnowledgeID(ctx, kid)
		if err != nil {
			logrus.WithError(err).WithField("knowledge_id", kid).Warn("failed to list chunks for ACL derivation")
			continue
		}
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		logrus.WithField("knowledge_ids", knowledgeIDs).Warn("no chunks found for ACL derivation, using default L1")
		return &types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{},
		}
	}

	result, err := appsvc.CalculateDerivedACLFromChunks(allChunks)
	if err != nil {
		logrus.WithError(err).Warn("failed to calculate derived ACL, using default L1")
		return &types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{},
		}
	}

	logrus.WithFields(logrus.Fields{
		"security_level":   result.SecurityLevel,
		"allowed_users":    len(result.AllowedUserIDs),
		"allowed_groups":   len(result.AllowedGroupIDs),
		"source_knowledge": len(knowledgeIDs),
		"source_chunks":    len(allChunks),
	}).Info("derived ACL for wiki page crystallization")

	return &result
}
