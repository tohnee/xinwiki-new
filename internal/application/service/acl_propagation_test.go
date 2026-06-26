package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestCalculateDerivedACL_EmptySources(t *testing.T) {
	_, err := CalculateDerivedACL(nil)
	if err == nil {
		t.Error("expected error for nil sources, got nil")
	}

	_, err = CalculateDerivedACL([]types.ACLSource{})
	if err == nil {
		t.Error("expected error for empty sources, got nil")
	}
}

func TestCalculateDerivedACL_SingleSource(t *testing.T) {
	cases := []struct {
		name   string
		source types.ACLSource
	}{
		{
			name: "L1 single source",
			source: types.ACLSource{
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  []string{"u1", "u2"},
				AllowedGroupIDs: []string{"g1"},
			},
		},
		{
			name: "L3 single source",
			source: types.ACLSource{
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  []string{"u3"},
				AllowedGroupIDs: []string{"g2", "g3"},
			},
		},
		{
			name: "empty ACL single source",
			source: types.ACLSource{
				SecurityLevel:   types.SecurityLevelL2,
				AllowedUserIDs:  nil,
				AllowedGroupIDs: nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CalculateDerivedACL([]types.ACLSource{tc.source})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.SecurityLevel != tc.source.SecurityLevel {
				t.Errorf("security_level = %q, want %q", result.SecurityLevel, tc.source.SecurityLevel)
			}
			if len(result.AllowedUserIDs) != len(tc.source.AllowedUserIDs) {
				t.Errorf("allowed_user_ids len = %d, want %d", len(result.AllowedUserIDs), len(tc.source.AllowedUserIDs))
			}
			if len(result.AllowedGroupIDs) != len(tc.source.AllowedGroupIDs) {
				t.Errorf("allowed_group_ids len = %d, want %d", len(result.AllowedGroupIDs), len(tc.source.AllowedGroupIDs))
			}
		})
	}
}

func TestCalculateDerivedACL_MultiSourceSecurityLevel(t *testing.T) {
	cases := []struct {
		name     string
		sources  []string
		expected string
	}{
		{
			name:     "L1 + L2 = L2",
			sources:  []string{types.SecurityLevelL1, types.SecurityLevelL2},
			expected: types.SecurityLevelL2,
		},
		{
			name:     "L2 + L1 = L2",
			sources:  []string{types.SecurityLevelL2, types.SecurityLevelL1},
			expected: types.SecurityLevelL2,
		},
		{
			name:     "L1 + L3 + L2 = L3",
			sources:  []string{types.SecurityLevelL1, types.SecurityLevelL3, types.SecurityLevelL2},
			expected: types.SecurityLevelL3,
		},
		{
			name:     "all L4 = L4",
			sources:  []string{types.SecurityLevelL4, types.SecurityLevelL4},
			expected: types.SecurityLevelL4,
		},
		{
			name:     "L1 + L4 = L4",
			sources:  []string{types.SecurityLevelL1, types.SecurityLevelL4},
			expected: types.SecurityLevelL4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := make([]types.ACLSource, len(tc.sources))
			for i, sl := range tc.sources {
				sources[i] = types.ACLSource{SecurityLevel: sl}
			}
			result, err := CalculateDerivedACL(sources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.SecurityLevel != tc.expected {
				t.Errorf("security_level = %q, want %q", result.SecurityLevel, tc.expected)
			}
		})
	}
}

func TestCalculateDerivedACL_MultiSourceUserIntersection(t *testing.T) {
	cases := []struct {
		name     string
		sources  [][]string
		expected []string
	}{
		{
			name: "two sources with common users",
			sources: [][]string{
				{"u1", "u2", "u3"},
				{"u2", "u3", "u4"},
			},
			expected: []string{"u2", "u3"},
		},
		{
			name: "three sources with one common user",
			sources: [][]string{
				{"u1", "u2", "u3"},
				{"u2", "u3", "u4"},
				{"u3", "u4", "u5"},
			},
			expected: []string{"u3"},
		},
		{
			name: "no common users",
			sources: [][]string{
				{"u1", "u2"},
				{"u3", "u4"},
			},
			expected: []string{},
		},
		{
			name: "one source has empty users",
			sources: [][]string{
				{"u1", "u2"},
				{},
			},
			expected: []string{},
		},
		{
			name: "all sources have empty users",
			sources: [][]string{
				{},
				{},
			},
			expected: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := make([]types.ACLSource, len(tc.sources))
			for i, users := range tc.sources {
				sources[i] = types.ACLSource{
					SecurityLevel:  types.SecurityLevelL1,
					AllowedUserIDs: users,
				}
			}
			result, err := CalculateDerivedACL(sources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.AllowedUserIDs) != len(tc.expected) {
				t.Errorf("allowed_user_ids len = %d, want %d; got %v", len(result.AllowedUserIDs), len(tc.expected), result.AllowedUserIDs)
			}
			expectedSet := make(map[string]bool)
			for _, u := range tc.expected {
				expectedSet[u] = true
			}
			for _, u := range result.AllowedUserIDs {
				if !expectedSet[u] {
					t.Errorf("unexpected user %q in result", u)
				}
			}
		})
	}
}

func TestCalculateDerivedACL_MultiSourceGroupIntersection(t *testing.T) {
	cases := []struct {
		name     string
		sources  [][]string
		expected []string
	}{
		{
			name: "two sources with common groups",
			sources: [][]string{
				{"g1", "g2", "g3"},
				{"g2", "g3", "g4"},
			},
			expected: []string{"g2", "g3"},
		},
		{
			name: "three sources with one common group",
			sources: [][]string{
				{"g1", "g2"},
				{"g2", "g3"},
				{"g2", "g4"},
			},
			expected: []string{"g2"},
		},
		{
			name: "no common groups",
			sources: [][]string{
				{"g1", "g2"},
				{"g3", "g4"},
			},
			expected: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := make([]types.ACLSource, len(tc.sources))
			for i, groups := range tc.sources {
				sources[i] = types.ACLSource{
					SecurityLevel:   types.SecurityLevelL1,
					AllowedGroupIDs: groups,
				}
			}
			result, err := CalculateDerivedACL(sources)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.AllowedGroupIDs) != len(tc.expected) {
				t.Errorf("allowed_group_ids len = %d, want %d; got %v", len(result.AllowedGroupIDs), len(tc.expected), result.AllowedGroupIDs)
			}
			expectedSet := make(map[string]bool)
			for _, g := range tc.expected {
				expectedSet[g] = true
			}
			for _, g := range result.AllowedGroupIDs {
				if !expectedSet[g] {
					t.Errorf("unexpected group %q in result", g)
				}
			}
		})
	}
}

func TestCalculateDerivedACL_MissingACLDoesNotExpandPermissions(t *testing.T) {
	sources := []types.ACLSource{
		{
			SecurityLevel:   types.SecurityLevelL2,
			AllowedUserIDs:  []string{"u1", "u2"},
			AllowedGroupIDs: []string{"g1"},
		},
		{
			SecurityLevel: types.SecurityLevelL3,
		},
	}

	result, err := CalculateDerivedACL(sources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SecurityLevel != types.SecurityLevelL3 {
		t.Errorf("security_level = %q, want L3", result.SecurityLevel)
	}

	if len(result.AllowedUserIDs) != 0 {
		t.Errorf("allowed_user_ids should be empty when one source missing, got %v", result.AllowedUserIDs)
	}
	if len(result.AllowedGroupIDs) != 0 {
		t.Errorf("allowed_group_ids should be empty when one source missing, got %v", result.AllowedGroupIDs)
	}
}

func TestCalculateDerivedACL_CombinedScenario(t *testing.T) {
	sources := []types.ACLSource{
		{
			SecurityLevel:   types.SecurityLevelL2,
			AllowedUserIDs:  []string{"u1", "u2", "u3"},
			AllowedGroupIDs: []string{"g1", "g2"},
		},
		{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  []string{"u2", "u3", "u4"},
			AllowedGroupIDs: []string{"g2", "g3"},
		},
		{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{"u2", "u5"},
			AllowedGroupIDs: []string{"g2"},
		},
	}

	result, err := CalculateDerivedACL(sources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SecurityLevel != types.SecurityLevelL3 {
		t.Errorf("security_level = %q, want L3", result.SecurityLevel)
	}

	if len(result.AllowedUserIDs) != 1 || result.AllowedUserIDs[0] != "u2" {
		t.Errorf("allowed_user_ids = %v, want [u2]", result.AllowedUserIDs)
	}

	if len(result.AllowedGroupIDs) != 1 || result.AllowedGroupIDs[0] != "g2" {
		t.Errorf("allowed_group_ids = %v, want [g2]", result.AllowedGroupIDs)
	}
}

func TestChunkToACLSource(t *testing.T) {
	t.Run("nil chunk returns safe defaults", func(t *testing.T) {
		src := ChunkToACLSource(nil)
		if src.SecurityLevel != types.SecurityLevelL1 {
			t.Errorf("security_level = %q, want L1", src.SecurityLevel)
		}
		if src.AllowedUserIDs == nil {
			t.Error("allowed_user_ids should not be nil")
		}
		if src.AllowedGroupIDs == nil {
			t.Error("allowed_group_ids should not be nil")
		}
	})

	t.Run("chunk with values is preserved", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{"u1", "u2"},
			AllowedGroupIDs: types.StringArray{"g1"},
		}
		src := ChunkToACLSource(chunk)
		if src.SecurityLevel != types.SecurityLevelL3 {
			t.Errorf("security_level = %q, want L3", src.SecurityLevel)
		}
		if len(src.AllowedUserIDs) != 2 {
			t.Errorf("allowed_user_ids len = %d, want 2", len(src.AllowedUserIDs))
		}
		if len(src.AllowedGroupIDs) != 1 {
			t.Errorf("allowed_group_ids len = %d, want 1", len(src.AllowedGroupIDs))
		}
	})

	t.Run("empty security level defaults to L1", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel: "",
		}
		src := ChunkToACLSource(chunk)
		if src.SecurityLevel != types.SecurityLevelL1 {
			t.Errorf("security_level = %q, want L1", src.SecurityLevel)
		}
	})
}

func TestWikiPageToACLSource(t *testing.T) {
	t.Run("nil page returns safe defaults", func(t *testing.T) {
		src := WikiPageToACLSource(nil)
		if src.SecurityLevel != types.SecurityLevelL1 {
			t.Errorf("security_level = %q, want L1", src.SecurityLevel)
		}
		if src.AllowedUserIDs == nil {
			t.Error("allowed_user_ids should not be nil")
		}
		if src.AllowedGroupIDs == nil {
			t.Error("allowed_group_ids should not be nil")
		}
	})

	t.Run("page with values is preserved", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL2,
			AllowedUserIDs:  types.StringArray{"u3"},
			AllowedGroupIDs: types.StringArray{"g2", "g3"},
		}
		src := WikiPageToACLSource(page)
		if src.SecurityLevel != types.SecurityLevelL2 {
			t.Errorf("security_level = %q, want L2", src.SecurityLevel)
		}
		if len(src.AllowedUserIDs) != 1 {
			t.Errorf("allowed_user_ids len = %d, want 1", len(src.AllowedUserIDs))
		}
		if len(src.AllowedGroupIDs) != 2 {
			t.Errorf("allowed_group_ids len = %d, want 2", len(src.AllowedGroupIDs))
		}
	})
}

func TestApplyDerivedACLToWikiPage(t *testing.T) {
	t.Run("nil page is safe", func(t *testing.T) {
		ApplyDerivedACLToWikiPage(nil, types.DerivedACLResult{
			SecurityLevel: types.SecurityLevelL3,
		})
	})

	t.Run("applies values correctly", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel: types.SecurityLevelL1,
		}
		result := types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL4,
			AllowedUserIDs:  []string{"u1"},
			AllowedGroupIDs: []string{"g1", "g2"},
		}
		ApplyDerivedACLToWikiPage(page, result)
		if page.SecurityLevel != types.SecurityLevelL4 {
			t.Errorf("security_level = %q, want L4", page.SecurityLevel)
		}
		if len(page.AllowedUserIDs) != 1 {
			t.Errorf("allowed_user_ids len = %d, want 1", len(page.AllowedUserIDs))
		}
		if len(page.AllowedGroupIDs) != 2 {
			t.Errorf("allowed_group_ids len = %d, want 2", len(page.AllowedGroupIDs))
		}
	})
}

func TestIsSecurityLevelHigherOrEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{types.SecurityLevelL1, types.SecurityLevelL1, true},
		{types.SecurityLevelL2, types.SecurityLevelL1, true},
		{types.SecurityLevelL3, types.SecurityLevelL2, true},
		{types.SecurityLevelL4, types.SecurityLevelL3, true},
		{types.SecurityLevelL4, types.SecurityLevelL1, true},
		{types.SecurityLevelL1, types.SecurityLevelL2, false},
		{types.SecurityLevelL2, types.SecurityLevelL3, false},
		{types.SecurityLevelL3, types.SecurityLevelL4, false},
	}
	for _, tc := range cases {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			if got := IsSecurityLevelHigherOrEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("IsSecurityLevelHigherOrEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestHasEmptyACL(t *testing.T) {
	t.Run("empty ACL returns true", func(t *testing.T) {
		result := types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{},
		}
		if !HasEmptyACL(result) {
			t.Error("expected HasEmptyACL to return true")
		}
	})

	t.Run("has users returns false", func(t *testing.T) {
		result := types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{"u1"},
			AllowedGroupIDs: []string{},
		}
		if HasEmptyACL(result) {
			t.Error("expected HasEmptyACL to return false")
		}
	})

	t.Run("has groups returns false", func(t *testing.T) {
		result := types.DerivedACLResult{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  []string{},
			AllowedGroupIDs: []string{"g1"},
		}
		if HasEmptyACL(result) {
			t.Error("expected HasEmptyACL to return false")
		}
	})
}

func TestParseSourceRefUUID(t *testing.T) {
	cases := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "uuid only",
			ref:      "550e8400-e29b-41d4-a716-446655440000",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "uuid with title",
			ref:      "550e8400-e29b-41d4-a716-446655440000|Document Title",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "empty string",
			ref:      "",
			expected: "",
		},
		{
			name:     "title only without uuid",
			ref:      "|Just Title",
			expected: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseSourceRefUUID(tc.ref)
			if result != tc.expected {
				t.Errorf("ParseSourceRefUUID(%q) = %q, want %q", tc.ref, result, tc.expected)
			}
		})
	}
}

func TestExtractKnowledgeIDsFromSourceRefs(t *testing.T) {
	t.Run("empty refs returns empty list", func(t *testing.T) {
		result := ExtractKnowledgeIDsFromSourceRefs(nil)
		if len(result) != 0 {
			t.Errorf("expected empty list, got %v", result)
		}
		result = ExtractKnowledgeIDsFromSourceRefs([]string{})
		if len(result) != 0 {
			t.Errorf("expected empty list, got %v", result)
		}
	})

	t.Run("mixed format refs extracts uuids", func(t *testing.T) {
		refs := []string{
			"uuid-1|Title One",
			"uuid-2",
			"uuid-3|Title Three",
		}
		result := ExtractKnowledgeIDsFromSourceRefs(refs)
		if len(result) != 3 {
			t.Fatalf("expected 3 ids, got %d", len(result))
		}
		expected := map[string]bool{"uuid-1": true, "uuid-2": true, "uuid-3": true}
		for _, id := range result {
			if !expected[id] {
				t.Errorf("unexpected id %q in result", id)
			}
		}
	})

	t.Run("skips empty or invalid refs", func(t *testing.T) {
		refs := []string{
			"",
			"|just title",
			"valid-uuid|Valid Title",
		}
		result := ExtractKnowledgeIDsFromSourceRefs(refs)
		if len(result) != 1 {
			t.Fatalf("expected 1 id, got %d", len(result))
		}
		if result[0] != "valid-uuid" {
			t.Errorf("expected valid-uuid, got %q", result[0])
		}
	})
}

func TestChunksToACLSources(t *testing.T) {
	t.Run("nil chunks returns empty sources", func(t *testing.T) {
		sources := ChunksToACLSources(nil)
		if len(sources) != 0 {
			t.Errorf("expected 0 sources, got %d", len(sources))
		}
	})

	t.Run("empty chunks returns empty sources", func(t *testing.T) {
		sources := ChunksToACLSources([]*types.Chunk{})
		if len(sources) != 0 {
			t.Errorf("expected 0 sources, got %d", len(sources))
		}
	})

	t.Run("multiple chunks converted correctly", func(t *testing.T) {
		chunks := []*types.Chunk{
			{
				SecurityLevel:   types.SecurityLevelL2,
				AllowedUserIDs:  types.StringArray{"u1", "u2"},
				AllowedGroupIDs: types.StringArray{"g1"},
			},
			{
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{"u2", "u3"},
				AllowedGroupIDs: types.StringArray{"g1", "g2"},
			},
		}
		sources := ChunksToACLSources(chunks)
		if len(sources) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(sources))
		}
		if sources[0].SecurityLevel != types.SecurityLevelL2 {
			t.Errorf("source[0] security_level = %q, want L2", sources[0].SecurityLevel)
		}
		if sources[1].SecurityLevel != types.SecurityLevelL3 {
			t.Errorf("source[1] security_level = %q, want L3", sources[1].SecurityLevel)
		}
	})
}

func TestCalculateDerivedACLFromChunks(t *testing.T) {
	t.Run("empty chunks returns error", func(t *testing.T) {
		_, err := CalculateDerivedACLFromChunks(nil)
		if err == nil {
			t.Error("expected error for nil chunks")
		}
		_, err = CalculateDerivedACLFromChunks([]*types.Chunk{})
		if err == nil {
			t.Error("expected error for empty chunks")
		}
	})

	t.Run("single chunk preserves ACL", func(t *testing.T) {
		chunks := []*types.Chunk{
			{
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{"u1", "u2"},
				AllowedGroupIDs: types.StringArray{"g1"},
			},
		}
		result, err := CalculateDerivedACLFromChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SecurityLevel != types.SecurityLevelL3 {
			t.Errorf("security_level = %q, want L3", result.SecurityLevel)
		}
		if len(result.AllowedUserIDs) != 2 {
			t.Errorf("allowed_user_ids len = %d, want 2", len(result.AllowedUserIDs))
		}
		if len(result.AllowedGroupIDs) != 1 {
			t.Errorf("allowed_group_ids len = %d, want 1", len(result.AllowedGroupIDs))
		}
	})

	t.Run("multiple chunks computes intersection and max level", func(t *testing.T) {
		chunks := []*types.Chunk{
			{
				SecurityLevel:   types.SecurityLevelL2,
				AllowedUserIDs:  types.StringArray{"u1", "u2", "u3"},
				AllowedGroupIDs: types.StringArray{"g1", "g2"},
			},
			{
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{"u2", "u3", "u4"},
				AllowedGroupIDs: types.StringArray{"g2", "g3"},
			},
		}
		result, err := CalculateDerivedACLFromChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SecurityLevel != types.SecurityLevelL3 {
			t.Errorf("security_level = %q, want L3", result.SecurityLevel)
		}
		if len(result.AllowedUserIDs) != 2 {
			t.Errorf("allowed_user_ids len = %d, want 2", len(result.AllowedUserIDs))
		}
		if len(result.AllowedGroupIDs) != 1 || result.AllowedGroupIDs[0] != "g2" {
			t.Errorf("allowed_group_ids = %v, want [g2]", result.AllowedGroupIDs)
		}
	})
}

func TestUserCanAccessChunk(t *testing.T) {
	t.Run("L1 user can access L1 chunk", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected L1 user to access L1 chunk with empty ACL")
		}
	})

	t.Run("L1 user cannot access L3 chunk", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected L1 user to NOT access L3 chunk")
		}
	})

	t.Run("L4 user can access L2 chunk", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL2,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessChunk(chunk, types.SecurityLevelL4, "user1", nil) {
			t.Error("expected L4 user to access L2 chunk")
		}
	})

	t.Run("user in allowed list can access even with lower level", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{"user1", "user2"},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected user in allowed list to access L3 chunk")
		}
	})

	t.Run("user not in allowed list cannot access higher level", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{"user1", "user2"},
			AllowedGroupIDs: types.StringArray{},
		}
		if UserCanAccessChunk(chunk, types.SecurityLevelL1, "user3", nil) {
			t.Error("expected user not in allowed list to NOT access L3 chunk")
		}
	})

	t.Run("user in allowed group can access", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{"group1", "group2"},
		}
		if !UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", []string{"group2"}) {
			t.Error("expected user in allowed group to access L3 chunk")
		}
	})

	t.Run("user not in any allowed group cannot access", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   types.SecurityLevelL3,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{"group1", "group2"},
		}
		if UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", []string{"group3"}) {
			t.Error("expected user not in allowed groups to NOT access L3 chunk")
		}
	})

	t.Run("nil chunk returns false", func(t *testing.T) {
		if UserCanAccessChunk(nil, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected nil chunk to return false")
		}
	})

	t.Run("empty security level defaults to L1", func(t *testing.T) {
		chunk := &types.Chunk{
			SecurityLevel:   "",
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessChunk(chunk, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected empty security level to default to L1 and allow access")
		}
	})
}

func TestUserCanAccessWikiPage(t *testing.T) {
	t.Run("L1 user can access L1 page", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL1,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessWikiPage(page, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected L1 user to access L1 page with empty ACL")
		}
	})

	t.Run("L1 user cannot access L4 page", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL4,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if UserCanAccessWikiPage(page, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected L1 user to NOT access L4 page")
		}
	})

	t.Run("user in allowed list can access", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL4,
			AllowedUserIDs:  types.StringArray{"user1"},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessWikiPage(page, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected user in allowed list to access L4 page")
		}
	})

	t.Run("nil page returns false", func(t *testing.T) {
		if UserCanAccessWikiPage(nil, types.SecurityLevelL1, "user1", nil) {
			t.Error("expected nil page to return false")
		}
	})
}

func TestFilterChunksByACL(t *testing.T) {
	t.Run("filters out inaccessible chunks", func(t *testing.T) {
		chunks := []*types.Chunk{
			{
				ID:              "chunk1",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
			{
				ID:              "chunk2",
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
			{
				ID:              "chunk3",
				SecurityLevel:   types.SecurityLevelL2,
				AllowedUserIDs:  types.StringArray{"user1"},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		filtered := FilterChunksByACL(chunks, types.SecurityLevelL2, "user2", nil)
		if len(filtered) != 2 {
			t.Errorf("expected 2 filtered chunks, got %d", len(filtered))
		}
		if filtered[0].ID != "chunk1" {
			t.Errorf("expected first chunk to be chunk1, got %s", filtered[0].ID)
		}
		if filtered[1].ID != "chunk3" {
			t.Errorf("expected second chunk to be chunk3, got %s", filtered[1].ID)
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		filtered := FilterChunksByACL(nil, types.SecurityLevelL1, "user1", nil)
		if len(filtered) != 0 {
			t.Errorf("expected 0 filtered chunks, got %d", len(filtered))
		}
	})
}

func TestFilterSearchResultChunksByACL(t *testing.T) {
	t.Run("filters out inaccessible chunks from search result map", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"chunk1": {
				ID:              "chunk1",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
			"chunk2": {
				ID:              "chunk2",
				SecurityLevel:   types.SecurityLevelL4,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
			"chunk3": {
				ID:              "chunk3",
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{"user1"},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "chunk1"},
			{ID: "chunk2"},
			{ID: "chunk3"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL2, "user1", nil)
		if len(filtered) != 2 {
			t.Errorf("expected 2 filtered results, got %d", len(filtered))
		}
		if filtered[0].ID != "chunk1" {
			t.Errorf("expected first result chunk1, got %s", filtered[0].ID)
		}
		if filtered[1].ID != "chunk3" {
			t.Errorf("expected second result chunk3, got %s", filtered[1].ID)
		}
	})

	t.Run("result with missing chunk is filtered out", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"chunk1": {
				ID:              "chunk1",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "chunk1"},
			{ID: "chunk2"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "user1", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 filtered result, got %d", len(filtered))
		}
	})
}

func TestPermissionLeakageRegression(t *testing.T) {
	t.Run("L1 user cannot access L4 chunk via search results", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"l1-chunk": {
				ID:              "l1-chunk",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
			"l4-chunk": {
				ID:              "l4-chunk",
				SecurityLevel:   types.SecurityLevelL4,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "l1-chunk", KnowledgeTitle: "Public Doc"},
			{ID: "l4-chunk", KnowledgeTitle: "Top Secret Doc"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "user-low", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 result, got %d - L4 chunk leaked!", len(filtered))
		}
		if filtered[0].ID != "l1-chunk" {
			t.Errorf("expected only l1-chunk, got %s", filtered[0].ID)
		}
	})

	t.Run("user not in allowed list cannot access restricted chunk", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"restricted-chunk": {
				ID:              "restricted-chunk",
				SecurityLevel:   types.SecurityLevelL2,
				AllowedUserIDs:  types.StringArray{"alice", "bob"},
				AllowedGroupIDs: types.StringArray{},
			},
			"public-chunk": {
				ID:              "public-chunk",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "restricted-chunk"},
			{ID: "public-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "charlie", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 result, got %d - restricted chunk leaked!", len(filtered))
		}
		if filtered[0].ID != "public-chunk" {
			t.Errorf("expected only public-chunk, got %s", filtered[0].ID)
		}
	})

	t.Run("user in allowed list can access restricted chunk even with lower security level", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"restricted-chunk": {
				ID:              "restricted-chunk",
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{"alice"},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "restricted-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "alice", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 result for allowed user, got %d", len(filtered))
		}
	})

	t.Run("user in allowed group can access restricted chunk", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"restricted-chunk": {
				ID:              "restricted-chunk",
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{"engineering", "hr"},
			},
		}
		results := []*types.SearchResult{
			{ID: "restricted-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "user1", []string{"engineering"})
		if len(filtered) != 1 {
			t.Errorf("expected 1 result for group member, got %d", len(filtered))
		}
	})

	t.Run("user not in allowed group cannot access restricted chunk", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"restricted-chunk": {
				ID:              "restricted-chunk",
				SecurityLevel:   types.SecurityLevelL3,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{"engineering"},
			},
		}
		results := []*types.SearchResult{
			{ID: "restricted-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "user1", []string{"sales"})
		if len(filtered) != 0 {
			t.Errorf("expected 0 results for non-group member, got %d", len(filtered))
		}
	})

	t.Run("empty security level defaults to L1", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"empty-sl-chunk": {
				ID:              "empty-sl-chunk",
				SecurityLevel:   "",
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "empty-sl-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "user1", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 result for empty-SL chunk with L1 user, got %d", len(filtered))
		}
	})

	t.Run("nil chunk in map is handled safely", func(t *testing.T) {
		chunkMap := map[string]*types.Chunk{
			"nil-chunk": nil,
			"good-chunk": {
				ID:              "good-chunk",
				SecurityLevel:   types.SecurityLevelL1,
				AllowedUserIDs:  types.StringArray{},
				AllowedGroupIDs: types.StringArray{},
			},
		}
		results := []*types.SearchResult{
			{ID: "nil-chunk"},
			{ID: "good-chunk"},
		}
		filtered := FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL4, "admin", nil)
		if len(filtered) != 1 {
			t.Errorf("expected 1 result (nil chunk skipped), got %d", len(filtered))
		}
	})

	t.Run("wiki page ACL: L1 user cannot access L4 wiki page", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL4,
			AllowedUserIDs:  types.StringArray{},
			AllowedGroupIDs: types.StringArray{},
		}
		if UserCanAccessWikiPage(page, types.SecurityLevelL1, "user1", nil) {
			t.Error("L1 user should not access L4 wiki page")
		}
	})

	t.Run("wiki page ACL: allowed user can access higher security level page", func(t *testing.T) {
		page := &types.WikiPage{
			SecurityLevel:   types.SecurityLevelL4,
			AllowedUserIDs:  types.StringArray{"alice"},
			AllowedGroupIDs: types.StringArray{},
		}
		if !UserCanAccessWikiPage(page, types.SecurityLevelL1, "alice", nil) {
			t.Error("allowed user should access L4 wiki page even with L1 clearance")
		}
	})

	t.Run("derived ACL: wiki page inherits highest security level from sources", func(t *testing.T) {
		sources := []types.ACLSource{
			{SecurityLevel: types.SecurityLevelL1, AllowedUserIDs: []string{}, AllowedGroupIDs: []string{}},
			{SecurityLevel: types.SecurityLevelL4, AllowedUserIDs: []string{}, AllowedGroupIDs: []string{}},
			{SecurityLevel: types.SecurityLevelL2, AllowedUserIDs: []string{}, AllowedGroupIDs: []string{}},
		}
		result, err := CalculateDerivedACL(sources)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SecurityLevel != types.SecurityLevelL4 {
			t.Errorf("expected derived security level L4, got %s", result.SecurityLevel)
		}
	})

	t.Run("derived ACL: wiki page inherits intersection of allowed users", func(t *testing.T) {
		sources := []types.ACLSource{
			{SecurityLevel: types.SecurityLevelL1, AllowedUserIDs: []string{"alice", "bob", "charlie"}, AllowedGroupIDs: []string{}},
			{SecurityLevel: types.SecurityLevelL1, AllowedUserIDs: []string{"bob", "dave"}, AllowedGroupIDs: []string{}},
		}
		result, err := CalculateDerivedACL(sources)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.AllowedUserIDs) != 1 || result.AllowedUserIDs[0] != "bob" {
			t.Errorf("expected intersection [bob], got %v", result.AllowedUserIDs)
		}
	})
}
