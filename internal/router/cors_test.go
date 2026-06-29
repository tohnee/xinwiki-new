package router

import (
	"testing"

	"github.com/Tencent/XinWiki/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildCORSOriginFunc_NoConfig_AllowsLocalhost(t *testing.T) {
	fn := buildCORSOriginFunc(nil)

	assert.True(t, fn("http://localhost:3000"), "localhost:3000 should be allowed in dev mode")
	assert.True(t, fn("http://localhost:8080"), "localhost:8080 should be allowed in dev mode")
	assert.True(t, fn("http://127.0.0.1:5173"), "127.0.0.1 should be allowed in dev mode")
	assert.False(t, fn("https://evil.com"), "evil.com should NOT be allowed in dev mode")
	assert.False(t, fn("https://localhost.evil.com"), "localhost.evil.com should NOT be allowed")
}

func TestBuildCORSOriginFunc_ExplicitOrigins(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			CORSAllowedOrigins: []string{"https://app.example.com", "https://admin.example.com/"},
		},
	}
	fn := buildCORSOriginFunc(cfg)

	assert.True(t, fn("https://app.example.com"), "explicit origin should be allowed")
	assert.True(t, fn("https://admin.example.com"), "trailing slash should be trimmed")
	assert.False(t, fn("https://evil.com"), "non-listed origin should be rejected")
	assert.False(t, fn("http://localhost:3000"), "localhost not allowed when explicit origins set")
}

func TestBuildCORSOriginFunc_WildcardRejected(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			CORSAllowedOrigins: []string{"*"},
		},
	}
	fn := buildCORSOriginFunc(cfg)

	assert.True(t, fn("http://localhost:3000"), "localhost still allowed with wildcard (dev)")
	assert.False(t, fn("https://evil.com"), "wildcard with credentials must not allow arbitrary origins")
}

func TestBuildCORSOriginFunc_FrontendBaseURL(t *testing.T) {
	cfg := &config.Config{
		FrontendBaseURL: "https://wiki.example.com",
	}
	fn := buildCORSOriginFunc(cfg)

	assert.True(t, fn("https://wiki.example.com"), "FrontendBaseURL should be auto-allowed")
	assert.False(t, fn("https://evil.com"), "other origins should be rejected")
}

func TestBuildCORSOriginFunc_MixedOrigins(t *testing.T) {
	cfg := &config.Config{
		Server: &config.ServerConfig{
			CORSAllowedOrigins: []string{"https://a.example.com", "https://b.example.com"},
		},
	}
	fn := buildCORSOriginFunc(cfg)

	assert.True(t, fn("https://a.example.com"))
	assert.True(t, fn("https://b.example.com"))
	assert.False(t, fn("https://c.example.com"))
}
