package acl

import (
	"errors"
	"strings"

	"github.com/Tencent/XinWiki/internal/types"
)

var errEmptySources = errors.New("acl_propagation: at least one source is required")

var securityLevelRank = map[string]int{
	types.SecurityLevelL1: 1,
	types.SecurityLevelL2: 2,
	types.SecurityLevelL3: 3,
	types.SecurityLevelL4: 4,
}

func CalculateDerivedACL(sources []types.ACLSource) (types.DerivedACLResult, error) {
	if len(sources) == 0 {
		return types.DerivedACLResult{}, errEmptySources
	}

	result := types.DerivedACLResult{
		SecurityLevel:   types.SecurityLevelL1,
		AllowedUserIDs:  nil,
		AllowedGroupIDs: nil,
	}

	userIntersectionInitialized := false
	groupIntersectionInitialized := false

	for _, src := range sources {
		sl := strings.TrimSpace(src.SecurityLevel)
		if sl == "" {
			sl = types.SecurityLevelL1
		}

		if securityLevelRank[sl] > securityLevelRank[result.SecurityLevel] {
			result.SecurityLevel = sl
		}

		if !userIntersectionInitialized {
			result.AllowedUserIDs = copyStringSlice(src.AllowedUserIDs)
			userIntersectionInitialized = true
		} else {
			result.AllowedUserIDs = intersectStringSlices(result.AllowedUserIDs, src.AllowedUserIDs)
		}

		if !groupIntersectionInitialized {
			result.AllowedGroupIDs = copyStringSlice(src.AllowedGroupIDs)
			groupIntersectionInitialized = true
		} else {
			result.AllowedGroupIDs = intersectStringSlices(result.AllowedGroupIDs, src.AllowedGroupIDs)
		}
	}

	if result.AllowedUserIDs == nil {
		result.AllowedUserIDs = []string{}
	}
	if result.AllowedGroupIDs == nil {
		result.AllowedGroupIDs = []string{}
	}

	return result, nil
}

func intersectStringSlices(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return []string{}
	}

	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}

	result := make([]string, 0, len(a))
	seen := make(map[string]struct{})
	for _, s := range b {
		if _, ok := set[s]; ok {
			if _, already := seen[s]; !already {
				result = append(result, s)
				seen[s] = struct{}{}
			}
		}
	}

	return result
}

func copyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func ChunkToACLSource(chunk *types.Chunk) types.ACLSource {
	if chunk == nil {
		return types.ACLSource{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{},
		}
	}
	sl := chunk.SecurityLevel
	if strings.TrimSpace(sl) == "" {
		sl = types.SecurityLevelL1
	}
	userIDs := chunk.AllowedUserIDs
	if userIDs == nil {
		userIDs = []string{}
	}
	groupIDs := chunk.AllowedGroupIDs
	if groupIDs == nil {
		groupIDs = []string{}
	}
	return types.ACLSource{
		SecurityLevel:   sl,
		AllowedUserIDs:  userIDs,
		AllowedGroupIDs: groupIDs,
	}
}

func WikiPageToACLSource(page *types.WikiPage) types.ACLSource {
	if page == nil {
		return types.ACLSource{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{},
		}
	}
	sl := page.SecurityLevel
	if strings.TrimSpace(sl) == "" {
		sl = types.SecurityLevelL1
	}
	userIDs := page.AllowedUserIDs
	if userIDs == nil {
		userIDs = []string{}
	}
	groupIDs := page.AllowedGroupIDs
	if groupIDs == nil {
		groupIDs = []string{}
	}
	return types.ACLSource{
		SecurityLevel:   sl,
		AllowedUserIDs:  userIDs,
		AllowedGroupIDs: groupIDs,
	}
}

func ApplyDerivedACLToWikiPage(page *types.WikiPage, result types.DerivedACLResult) {
	if page == nil {
		return
	}
	page.SecurityLevel = result.SecurityLevel
	page.AllowedUserIDs = result.AllowedUserIDs
	page.AllowedGroupIDs = result.AllowedGroupIDs
}

func IsSecurityLevelHigherOrEqual(a, b string) bool {
	return securityLevelRank[a] >= securityLevelRank[b]
}

func HasEmptyACL(result types.DerivedACLResult) bool {
	return len(result.AllowedUserIDs) == 0 && len(result.AllowedGroupIDs) == 0
}

func ParseSourceRefUUID(ref string) string {
	if idx := strings.Index(ref, "|"); idx >= 0 {
		return ref[:idx]
	}
	return ref
}

func ExtractKnowledgeIDsFromSourceRefs(refs []string) []string {
	if len(refs) == 0 {
		return []string{}
	}
	ids := make([]string, 0, len(refs))
	seen := make(map[string]struct{})
	for _, ref := range refs {
		id := ParseSourceRefUUID(ref)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func ChunksToACLSources(chunks []*types.Chunk) []types.ACLSource {
	if len(chunks) == 0 {
		return []types.ACLSource{}
	}
	sources := make([]types.ACLSource, 0, len(chunks))
	for _, chunk := range chunks {
		sources = append(sources, ChunkToACLSource(chunk))
	}
	return sources
}

func CalculateDerivedACLFromChunks(chunks []*types.Chunk) (types.DerivedACLResult, error) {
	if len(chunks) == 0 {
		return types.DerivedACLResult{}, errEmptySources
	}
	sources := ChunksToACLSources(chunks)
	return CalculateDerivedACL(sources)
}

func UserCanAccessChunk(chunk *types.Chunk, userSecurityLevel string, userID string, userGroupIDs []string) bool {
	if chunk == nil {
		return false
	}

	// L4 (admin) bypasses all ACL restrictions.
	if userSecurityLevel == types.SecurityLevelL4 {
		return true
	}

	chunkSL := strings.TrimSpace(chunk.SecurityLevel)
	if chunkSL == "" {
		chunkSL = types.SecurityLevelL1
	}

	hasUserACL := len(chunk.AllowedUserIDs) > 0
	hasGroupACL := len(chunk.AllowedGroupIDs) > 0

	// When explicit user/group ACLs are present, they act as the authoritative
	// access list: the user must match an allowed user OR belong to an allowed
	// group. Security level is NOT sufficient to bypass explicit user/group
	// restrictions (otherwise an L3 user could read another user's private
	// chunk just by meeting the SL threshold).
	if hasUserACL || hasGroupACL {
		if hasUserACL && userID != "" {
			for _, uid := range chunk.AllowedUserIDs {
				if uid == userID {
					return true
				}
			}
		}
		if hasGroupACL && len(userGroupIDs) > 0 {
			groupSet := make(map[string]struct{}, len(userGroupIDs))
			for _, g := range userGroupIDs {
				groupSet[g] = struct{}{}
			}
			for _, g := range chunk.AllowedGroupIDs {
				if _, ok := groupSet[g]; ok {
					return true
				}
			}
		}
		return false
	}

	// No explicit user/group ACLs — access is determined purely by security level.
	return IsSecurityLevelHigherOrEqual(userSecurityLevel, chunkSL)
}

func UserCanAccessWikiPage(page *types.WikiPage, userSecurityLevel string, userID string, userGroupIDs []string) bool {
	if page == nil {
		return false
	}

	if userSecurityLevel == types.SecurityLevelL4 {
		return true
	}

	pageSL := strings.TrimSpace(page.SecurityLevel)
	if pageSL == "" {
		pageSL = types.SecurityLevelL1
	}

	hasUserACL := len(page.AllowedUserIDs) > 0
	hasGroupACL := len(page.AllowedGroupIDs) > 0

	if hasUserACL || hasGroupACL {
		if hasUserACL && userID != "" {
			for _, uid := range page.AllowedUserIDs {
				if uid == userID {
					return true
				}
			}
		}
		if hasGroupACL && len(userGroupIDs) > 0 {
			groupSet := make(map[string]struct{}, len(userGroupIDs))
			for _, g := range userGroupIDs {
				groupSet[g] = struct{}{}
			}
			for _, g := range page.AllowedGroupIDs {
				if _, ok := groupSet[g]; ok {
					return true
				}
			}
		}
		return false
	}

	return IsSecurityLevelHigherOrEqual(userSecurityLevel, pageSL)
}

func FilterChunksByACL(chunks []*types.Chunk, userSecurityLevel string, userID string, userGroupIDs []string) []*types.Chunk {
	if len(chunks) == 0 {
		return []*types.Chunk{}
	}

	filtered := make([]*types.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if UserCanAccessChunk(chunk, userSecurityLevel, userID, userGroupIDs) {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

func FilterSearchResultChunksByACL(results []*types.SearchResult, chunkMap map[string]*types.Chunk, userSecurityLevel string, userID string, userGroupIDs []string) []*types.SearchResult {
	if len(results) == 0 {
		return []*types.SearchResult{}
	}

	filtered := make([]*types.SearchResult, 0, len(results))
	for _, result := range results {
		chunk, ok := chunkMap[result.ID]
		if !ok {
			continue
		}
		if UserCanAccessChunk(chunk, userSecurityLevel, userID, userGroupIDs) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}
