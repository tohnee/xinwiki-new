package generator

import "github.com/Tencent/XinWiki/internal/types"

// BuildCitationsFromWikiPage constructs a single-element citation list from
// a wiki page. Returns nil when the page is nil so callers can assign the
// result directly without nil checks. The wiki page is the only source
// material the artifact service currently feeds into generators; when richer
// RAG-backed sources are wired in, extend this helper (or add a sibling)
// rather than inlining the mapping at each call site.
func BuildCitationsFromWikiPage(page *types.WikiPage) []Citation {
	if page == nil {
		return nil
	}
	c := Citation{
		ID:    1,
		Title: page.Title,
		Type:  "wiki_page",
		RefID: page.ID,
	}
	if page.Slug != "" {
		// Stable, human-readable path that the frontend can route to
		// without reconstructing it from KB + slug.
		c.URL = "/wiki/" + page.KnowledgeBaseID + "/" + page.Slug
	}
	return []Citation{c}
}
