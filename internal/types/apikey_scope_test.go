package types

import "testing"

// TestScopesAllow_SuperScopeWildcard: the "*" scope authorizes any required
// scope — the super/owner scope.
func TestScopesAllow_SuperScopeWildcard(t *testing.T) {
	if !ScopesAllow([]string{"*"}, "kb:read") {
		t.Errorf("\"*\" 应允许 kb:read")
	}
	if !ScopesAllow([]string{"*"}, "admin:delete") {
		t.Errorf("\"*\" 应允许任意 scope")
	}
}

// TestScopesAllow_ExactMatch: an exact scope match authorizes.
func TestScopesAllow_ExactMatch(t *testing.T) {
	if !ScopesAllow([]string{"kb:read"}, "kb:read") {
		t.Errorf("kb:read 应允许 kb:read")
	}
}

// TestScopesAllow_PrefixWildcard: "kb:*" authorizes any kb:<action> but not
// other resource families.
func TestScopesAllow_PrefixWildcard(t *testing.T) {
	have := []string{"kb:*"}
	if !ScopesAllow(have, "kb:read") {
		t.Errorf("kb:* 应允许 kb:read")
	}
	if !ScopesAllow(have, "kb:write") {
		t.Errorf("kb:* 应允许 kb:write")
	}
	if ScopesAllow(have, "doc:read") {
		t.Errorf("kb:* 不应允许 doc:read")
	}
}

// TestScopesAllow_PrefixSegmentBound: "kb:*" must NOT authorize "kbx:read" —
// the wildcard is segment-bound, not a string prefix.
func TestScopesAllow_PrefixSegmentBound(t *testing.T) {
	if ScopesAllow([]string{"kb:*"}, "kbx:read") {
		t.Errorf("kb:* 不应允许 kbx:read（段边界）")
	}
}

// TestScopesAllow_NonMatchingScope: a scope that is neither exact nor covered
// by a wildcard must be denied.
func TestScopesAllow_NonMatchingScope(t *testing.T) {
	if ScopesAllow([]string{"kb:read"}, "kb:write") {
		t.Errorf("kb:read 不应允许 kb:write")
	}
	if ScopesAllow([]string{"chat"}, "kb:read") {
		t.Errorf("chat 不应允许 kb:read")
	}
}

// TestScopesAllow_EmptyHaveFailsClosed: no granted scopes -> deny (fail-closed
// for security; never default-allow).
func TestScopesAllow_EmptyHaveFailsClosed(t *testing.T) {
	if ScopesAllow(nil, "kb:read") {
		t.Errorf("空 scope 集合应 fail-closed 拒绝")
	}
	if ScopesAllow([]string{}, "kb:read") {
		t.Errorf("空 scope 集合应 fail-closed 拒绝")
	}
}

// TestScopesAllow_MultipleHave: any granted scope that covers the required
// one authorizes.
func TestScopesAllow_MultipleHave(t *testing.T) {
	if !ScopesAllow([]string{"chat", "kb:read", "doc:*"}, "doc:write") {
		t.Errorf("doc:* 在集合中应允许 doc:write")
	}
	if !ScopesAllow([]string{"chat", "kb:read", "doc:*"}, "chat") {
		t.Errorf("chat 在集合中应允许 chat")
	}
	if ScopesAllow([]string{"chat", "kb:read", "doc:*"}, "admin:users") {
		t.Errorf("无覆盖 admin:users 的 scope，应拒绝")
	}
}

// TestScopesAllowAll_AllRequired: all required scopes must be covered; any
// missing one denies the whole set.
func TestScopesAllowAll_AllRequired(t *testing.T) {
	if !ScopesAllowAll([]string{"kb:read", "kb:write"}, []string{"kb:read", "kb:write"}) {
		t.Errorf("全部覆盖应通过")
	}
	if ScopesAllowAll([]string{"kb:read"}, []string{"kb:read", "kb:write"}) {
		t.Errorf("缺少 kb:write 应整体拒绝")
	}
	if !ScopesAllowAll([]string{"*"}, []string{"kb:read", "admin:delete"}) {
		t.Errorf("\"*\" 应覆盖全部 required")
	}
}

// TestScopesAllowAll_EmptyRequired: no required scopes -> trivially allowed.
func TestScopesAllowAll_EmptyRequired(t *testing.T) {
	if !ScopesAllowAll([]string{"kb:read"}, nil) {
		t.Errorf("无 required scope 应通过")
	}
}

// TestIsValidScope_KnownExact: every declared scope constant is a valid scope.
func TestIsValidScope_KnownExact(t *testing.T) {
	for _, s := range []string{
		ScopeAll, ScopeKBRead, ScopeKBWrite, ScopeDocRead, ScopeDocWrite,
		ScopeChat, ScopeAgentRun, ScopeAdminUsers, ScopeAdminDelete, ScopeAdminAPIKeys,
	} {
		if !IsValidScope(s) {
			t.Errorf("IsValidScope(%q) 应为 true（已知 scope）", s)
		}
	}
}

// TestIsValidScope_ResourceWildcard: "<resource>:*" is valid for known
// resource families (kb / doc / admin) so an operator can grant a blanket
// family without enumerating every action.
func TestIsValidScope_ResourceWildcard(t *testing.T) {
	for _, s := range []string{"kb:*", "doc:*", "admin:*"} {
		if !IsValidScope(s) {
			t.Errorf("IsValidScope(%q) 应为 true（资源通配）", s)
		}
	}
}

// TestIsValidScope_RejectsUnknown: scopes that are neither a known exact
// scope nor a known "<resource>:*" wildcard are rejected so ValidateScopes
// can refuse typos like "kb:wriet" or invented resources like "foo:read".
func TestIsValidScope_RejectsUnknown(t *testing.T) {
	for _, s := range []string{
		"",          // empty
		"foo:read",  // unknown resource
		"kb:",       // empty action, not a wildcard
		":read",     // empty resource
		"kb:read:x", // too many segments
		"*:read",    // only bare "*" is the super scope
		"chat:*",    // chat has no sub-actions; "chat:*" is not a form
		"kbx:read",  // typo / adjacent resource
		"KB:READ",   // case-sensitive
	} {
		if IsValidScope(s) {
			t.Errorf("IsValidScope(%q) 应为 false", s)
		}
	}
}

// TestValidateScopes_RejectsAnyInvalid: one bad scope rejects the whole set;
// a fully-known set passes (including wildcards).
func TestValidateScopes_RejectsAnyInvalid(t *testing.T) {
	if err := ValidateScopes([]string{"kb:read", "foo:bar"}); err == nil {
		t.Errorf("含非法 scope 应返回错误")
	}
	if err := ValidateScopes([]string{"kb:read", "doc:*", "admin:apikeys"}); err != nil {
		t.Errorf("全部合法应返回 nil, got %v", err)
	}
}

// TestValidateScopes_EmptyIsAllowed: an empty scope set is structurally valid
// (the key authenticates but RequireScope fail-closes on every scoped route).
// Validation does not enforce non-empty; enforcement does.
func TestValidateScopes_EmptyIsAllowed(t *testing.T) {
	if err := ValidateScopes(nil); err != nil {
		t.Errorf("nil scopes 应返回 nil, got %v", err)
	}
	if err := ValidateScopes([]string{}); err != nil {
		t.Errorf("空 scopes 应返回 nil, got %v", err)
	}
}
