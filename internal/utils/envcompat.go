// Package utils envcompat: backward-compatible env-var aliasing for the
// WeKnora -> XinWiki brand migration. Operators may set either the
// XINWIKI_-prefixed (preferred) or the legacy WEKNORA_-prefixed (deprecated)
// form of a configuration variable; ResolveEnv hides the difference so call
// sites stay brand-agnostic.
package utils

import (
	"os"
	"strings"
)

const (
	xinwikiEnvPrefix = "XINWIKI_"
	legacyEnvPrefix  = "WEKNORA_"
)

// ResolveEnv resolves a configuration env var by its suffix (the part after
// the brand prefix), e.g. ResolveEnv("TENANT_ENABLE_RBAC"). The XINWIKI_-
// prefixed name is preferred; when it is unset or empty the legacy WEKNORA_-
// prefixed name is used so existing deployments keep working without changes.
//
// The returned value is NOT trimmed, matching os.Getenv semantics — callers
// that need whitespace trimmed must strings.TrimSpace it themselves.
func ResolveEnv(suffix string) string {
	if v := os.Getenv(xinwikiEnvPrefix + suffix); v != "" {
		return v
	}
	return os.Getenv(legacyEnvPrefix + suffix)
}

// ResolveEnvName resolves a full env-var name. If the name carries the
// legacy WEKNORA_ prefix, the XINWIKI_-prefixed alias is preferred (falling
// back to the legacy name). Names without the WEKNORA_ prefix (e.g.
// SSRF_WHITELIST, DISABLE_REGISTRATION) are read verbatim and are NOT
// rewritten — only brand-prefixed vars participate in aliasing. Use this for
// call sites that hold a full env-var name (e.g. a SystemSetting spec's
// EnvName) and cannot speak in suffixes.
func ResolveEnvName(name string) string {
	if rest, ok := strings.CutPrefix(name, legacyEnvPrefix); ok {
		return ResolveEnv(rest)
	}
	return os.Getenv(name)
}

// ResolveEnvLookup is the (value, ok) form of ResolveEnv, for callers that
// must distinguish "unset" from "set to empty" (e.g. trustedProxies(), where
// an explicit empty value means "disable" rather than "use default"). ok is
// true when EITHER the preferred XINWIKI_* alias OR the legacy WEKNORA_* var
// is set; an explicitly-empty preferred alias counts as set and wins.
func ResolveEnvLookup(suffix string) (string, bool) {
	if v, ok := os.LookupEnv(xinwikiEnvPrefix + suffix); ok {
		return v, true
	}
	return os.LookupEnv(legacyEnvPrefix + suffix)
}

// ResolveEnvNameLookup is the (value, ok) form of ResolveEnvName. A full
// WEKNORA_-prefixed name resolves with XINWIKI_* preferred; unbranded names
// pass through to os.LookupEnv verbatim.
func ResolveEnvNameLookup(name string) (string, bool) {
	if rest, ok := strings.CutPrefix(name, legacyEnvPrefix); ok {
		return ResolveEnvLookup(rest)
	}
	return os.LookupEnv(name)
}

// LegacyEnvSuffixes is the registry of env-var suffixes that support
// XINWIKI_/WEKNORA_ aliasing. It is the single source of truth for the
// migration surface and is walked once at startup to emit deprecation
// warnings. Keep it in sync with ResolveEnv call sites.
var LegacyEnvSuffixes = []string{
	"TENANT_ENABLE_RBAC",
	"TENANT_ENABLE_CROSS_TENANT_ACCESS",
	"TENANT_MAX_OWNED_PER_USER",
	"TENANT_DEFAULT_STORAGE_QUOTA_GB",
	"AUDIT_RETENTION_DAYS",
	"LANGUAGE",
	"REDIS_NAMESPACE",
	"REDIS_OP_TIMEOUT_MS",
	"ASYNQ_CONCURRENCY",
	"HOUSEKEEPING_ENABLED",
	"INVITATION_TTL",
	"TRUSTED_PROXIES",
	"WEB_DIR",
	"SKILLS_DIR",
	"SANDBOX_MODE",
	"SANDBOX_TIMEOUT",
	"SANDBOX_DOCKER_IMAGE",
	"AGENT_LLM_TIMEOUT",
	"AGENT_TOOL_APPROVAL_TIMEOUT",
	"AGENT_TOOL_APPROVAL_FAIL_OPEN",
	"DOCUMENT_PROCESS_TIMEOUT",
	"DOCREADER_CALL_TIMEOUT",
	"LLM_CHAT_TIMEOUT_SECONDS",
	"LLM_STREAM_TIMEOUT_SECONDS",
	"LLM_STREAM_RAW_DUMP",
	"LLM_STREAM_RAW_DUMP_DIR",
	"TEST_TIMEOUT_SECONDS",
	"BOOTSTRAP_SYSTEM_ADMIN_EMAIL",
}

// ActiveLegacyEnvSuffixes returns the suffixes whose legacy WEKNORA_-prefixed
// var is currently providing a value because the preferred XINWIKI_-prefixed
// alias is unset. Startup code logs this list once as a deprecation notice so
// operators know which vars to rename.
func ActiveLegacyEnvSuffixes() []string {
	var active []string
	for _, s := range LegacyEnvSuffixes {
		if os.Getenv(xinwikiEnvPrefix+s) == "" && os.Getenv(legacyEnvPrefix+s) != "" {
			active = append(active, s)
		}
	}
	return active
}
