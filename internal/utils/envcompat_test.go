package utils

import (
	"slices"
	"testing"
)

// TestResolveEnv_PrefersXinwiki: when the XINWIKI_-prefixed var is set, it
// wins over the legacy WEKNORA_-prefixed var.
func TestResolveEnv_PrefersXinwiki(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "preferred")
	t.Setenv("WEKNORA_FOO", "legacy")

	if got := ResolveEnv("FOO"); got != "preferred" {
		t.Fatalf("ResolveEnv 期望 'preferred'，实际 %q", got)
	}
}

// TestResolveEnv_FallsBackToWeknora: with only the legacy var set, it is
// returned so existing deployments keep working unchanged.
func TestResolveEnv_FallsBackToWeknora(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "")
	t.Setenv("WEKNORA_FOO", "legacy")

	if got := ResolveEnv("FOO"); got != "legacy" {
		t.Fatalf("ResolveEnv 期望 'legacy'，实际 %q", got)
	}
}

// TestResolveEnv_BothUnset: neither var set returns "".
func TestResolveEnv_BothUnset(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "")
	t.Setenv("WEKNORA_FOO", "")

	if got := ResolveEnv("FOO"); got != "" {
		t.Fatalf("ResolveEnv 期望 ''，实际 %q", got)
	}
}

// TestResolveEnv_PreservesRawValue: ResolveEnv does not trim, matching
// os.Getenv semantics — call sites that need trimming TrimSpace themselves.
func TestResolveEnv_PreservesRawValue(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "  spaced  ")

	if got := ResolveEnv("FOO"); got != "  spaced  " {
		t.Fatalf("ResolveEnv 应保留原值不 trim，实际 %q", got)
	}
}

// TestLegacyEnvSuffixes_RegistryPopulated: the registry is the single source
// of truth for the migration and must cover the known WEKNORA_* vars.
func TestLegacyEnvSuffixes_RegistryPopulated(t *testing.T) {
	if len(LegacyEnvSuffixes) == 0 {
		t.Fatalf("LegacyEnvSuffixes 不应为空")
	}
	mustHave := []string{
		"TENANT_ENABLE_RBAC",
		"LANGUAGE",
		"SANDBOX_MODE",
		"REDIS_NAMESPACE",
		"AUDIT_RETENTION_DAYS",
	}
	for _, s := range mustHave {
		if !slices.Contains(LegacyEnvSuffixes, s) {
			t.Errorf("LegacyEnvSuffixes 应包含 %q", s)
		}
	}
}

// TestActiveLegacyEnvSuffixes_ReportsLegacyInUse: a legacy var that is set
// while its preferred alias is unset is reported; setting the preferred alias
// removes it from the active-legacy list.
func TestActiveLegacyEnvSuffixes_ReportsLegacyInUse(t *testing.T) {
	// Legacy set, preferred unset -> reported as active legacy.
	t.Setenv("XINWIKI_SANDBOX_MODE", "")
	t.Setenv("WEKNORA_SANDBOX_MODE", "docker")
	if !slices.Contains(ActiveLegacyEnvSuffixes(), "SANDBOX_MODE") {
		t.Errorf("WEKNORA_SANDBOX_MODE 在用时应被 ActiveLegacyEnvSuffixes 报告")
	}

	// Preferred set -> no longer legacy-active.
	t.Setenv("XINWIKI_SANDBOX_MODE", "docker")
	if slices.Contains(ActiveLegacyEnvSuffixes(), "SANDBOX_MODE") {
		t.Errorf("XINWIKI_SANDBOX_MODE 设置后不应再报告为 legacy")
	}
}

// TestResolveEnvName_WeknoraPrefixPrefersXinwiki: a full WEKNORA_-prefixed
// name resolves to the XINWIKI_ alias when set.
func TestResolveEnvName_WeknoraPrefixPrefersXinwiki(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "preferred")
	t.Setenv("WEKNORA_FOO", "legacy")

	if got := ResolveEnvName("WEKNORA_FOO"); got != "preferred" {
		t.Fatalf("ResolveEnvName 期望 'preferred'，实际 %q", got)
	}
}

// TestResolveEnvName_WeknoraPrefixFallsBack: with only the legacy name set,
// the legacy value is returned.
func TestResolveEnvName_WeknoraPrefixFallsBack(t *testing.T) {
	t.Setenv("XINWIKI_FOO", "")
	t.Setenv("WEKNORA_FOO", "legacy")

	if got := ResolveEnvName("WEKNORA_FOO"); got != "legacy" {
		t.Fatalf("ResolveEnvName 期望 'legacy'，实际 %q", got)
	}
}

// TestResolveEnvName_UnbrandedPassthrough: names without the WEKNORA_ prefix
// (e.g. SSRF_WHITELIST) are read verbatim and NOT rewritten to XINWIKI_*.
func TestResolveEnvName_UnbrandedPassthrough(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "allow")
	t.Setenv("XINWIKI_SSRF_WHITELIST", "should-not-win")

	if got := ResolveEnvName("SSRF_WHITELIST"); got != "allow" {
		t.Fatalf("无前缀名应直通，期望 'allow'，实际 %q", got)
	}
}

// TestResolveEnvName_EmptyWhenUnset: an unbranded, unset name returns "".
func TestResolveEnvName_EmptyWhenUnset(t *testing.T) {
	if got := ResolveEnvName("SOME_UNSET_VAR_X9Z"); got != "" {
		t.Fatalf("未设置应返回 ''，实际 %q", got)
	}
}

// TestResolveEnvLookup_PrefersXinwiki: when the preferred alias is set, it is
// returned with ok=true, even when the legacy var is also set.
func TestResolveEnvLookup_PrefersXinwiki(t *testing.T) {
	t.Setenv("XINWIKI_XWLOOKUPT", "new")
	t.Setenv("WEKNORA_XWLOOKUPT", "old")

	v, ok := ResolveEnvLookup("XWLOOKUPT")
	if !ok {
		t.Fatalf("期望 ok=true")
	}
	if v != "new" {
		t.Fatalf("期望 'new'，实际 %q", v)
	}
}

// TestResolveEnvLookup_FallsBackToWeknora: preferred unset, legacy set ->
// legacy value with ok=true. (We do NOT t.Setenv the preferred var: leaving
// it unset is the only way to express "unset", since t.Setenv("","") would
// set it to empty. XWLOOKUPT is chosen to avoid colliding with the ambient
// environment.)
func TestResolveEnvLookup_FallsBackToWeknora(t *testing.T) {
	t.Setenv("WEKNORA_XWLOOKUPT", "old")

	v, ok := ResolveEnvLookup("XWLOOKUPT")
	if !ok {
		t.Fatalf("期望 ok=true")
	}
	if v != "old" {
		t.Fatalf("期望 'old'，实际 %q", v)
	}
}

// TestResolveEnvLookup_BothUnset: neither set -> ok=false. This is the
// 3-state distinction ResolveEnv cannot make (empty-vs-unset), needed by
// callers like trustedProxies() that treat "explicitly empty" as "disable".
func TestResolveEnvLookup_BothUnset(t *testing.T) {
	if _, ok := ResolveEnvLookup("XWLOOKUPT"); ok {
		t.Fatalf("两者都未设置时期望 ok=false")
	}
}

// TestResolveEnvLookup_EmptyPreferredIsSet: XINWIKI_* set to empty string is
// still "set" (ok=true, value "") so callers can distinguish "disable" from
// "use default". WEKNORA_* must NOT shadow an explicitly-set empty preferred.
func TestResolveEnvLookup_EmptyPreferredIsSet(t *testing.T) {
	t.Setenv("XINWIKI_XWLOOKUPT", "")
	t.Setenv("WEKNORA_XWLOOKUPT", "should-not-win")

	v, ok := ResolveEnvLookup("XWLOOKUPT")
	if !ok {
		t.Fatalf("XINWIKI_XWLOOKUPT 显式设为空仍应 ok=true")
	}
	if v != "" {
		t.Fatalf("显式空值应返回 ''，实际 %q", v)
	}
}

// TestResolveEnvNameLookup_WeknoraPrefix: a full WEKNORA_-prefixed name
// resolves via the (value, ok) lookup with XINWIKI_* preferred.
func TestResolveEnvNameLookup_WeknoraPrefix(t *testing.T) {
	// Both unset -> ok=false.
	if _, ok := ResolveEnvNameLookup("WEKNORA_XWLOOKUPT"); ok {
		t.Fatalf("两者都未设置时期望 ok=false")
	}

	// Legacy set, preferred unset -> legacy value, ok=true.
	t.Setenv("WEKNORA_XWLOOKUPT", "old")
	v, ok := ResolveEnvNameLookup("WEKNORA_XWLOOKUPT")
	if !ok || v != "old" {
		t.Fatalf("legacy 回退期望 (old,true)，实际 (%q,%v)", v, ok)
	}
}

// TestResolveEnvNameLookup_UnbrandedPassthrough: unbranded names pass through
// to os.LookupEnv verbatim.
func TestResolveEnvNameLookup_UnbrandedPassthrough(t *testing.T) {
	t.Setenv("MY_PLAIN_VAR_XW", "x")
	v, ok := ResolveEnvNameLookup("MY_PLAIN_VAR_XW")
	if !ok || v != "x" {
		t.Fatalf("无前缀名应直通，期望 (x,true)，实际 (%q,%v)", v, ok)
	}
}
