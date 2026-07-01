package uum

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestRSAKey creates an RSA key pair for signing test JWTs.
func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

// jwk represents a JSON Web Key (public portion only).
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type oidcConfigResponse struct {
	Issuer                string `json:"issuer"`
	JwksURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
}

// setupTestOIDCProvider spins up a test HTTP server that serves OIDC discovery
// and JWKS endpoints. It returns the server URL and the RSA private key used
// to sign tokens (so tests can mint valid/invalid JWTs).
func setupTestOIDCProvider(t *testing.T) (issuer string, signKey *rsa.PrivateKey, cleanup func()) {
	t.Helper()
	signKey = generateTestRSAKey(t)

	// Build the JWK from public key
	pubKey := &signKey.PublicKey
	nBytes := pubKey.N.Bytes()
	eBytes := big.NewInt(int64(pubKey.E)).Bytes()
	testJWK := jwk{
		Kty: "RSA",
		Kid: "test-key-1",
		Use: "sig",
		Alg: "RS256",
		N:   base64.RawURLEncoding.EncodeToString(nBytes),
		E:   base64.RawURLEncoding.EncodeToString(eBytes),
	}

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		cfg := oidcConfigResponse{
			Issuer:                srv.URL,
			JwksURI:               srv.URL + "/.well-known/jwks.json",
			AuthorizationEndpoint: srv.URL + "/authorize",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksResponse{Keys: []jwk{testJWK}})
	})

	srv = httptest.NewServer(mux)
	return srv.URL, signKey, srv.Close
}

// mintTestJWT signs a JWT with the given key/claims and returns the compact serialized form.
func mintTestJWT(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign test JWT: %v", err)
	}
	return signed
}

func TestOIDCValidator_ValidToken(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		ID:       "oidc-test",
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config: map[string]interface{}{
			"issuer":    issuer,
			"client_id": "test-client",
		},
	}

	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":            issuer,
		"sub":            "user-123",
		"aud":            "test-client",
		"exp":            now.Add(1 * time.Hour).Unix(),
		"iat":            now.Unix(),
		"email":          "alice@example.com",
		"email_verified": true,
		"name":           "Alice Example",
		"preferred_username": "alice",
		"groups":         []string{"engineering", "wiki-editors"},
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	got, err := validator.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if got.Subject != "user-123" {
		t.Errorf("expected sub 'user-123', got %q", got.Subject)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected email 'alice@example.com', got %q", got.Email)
	}
	if got.Name != "Alice Example" {
		t.Errorf("expected name 'Alice Example', got %q", got.Name)
	}
	if !got.EmailVerified {
		t.Errorf("expected email_verified = true")
	}
	if len(got.Groups) != 2 || got.Groups[0] != "engineering" {
		t.Errorf("unexpected groups: %v", got.Groups)
	}
	if got.Issuer != issuer {
		t.Errorf("expected issuer %q, got %q", issuer, got.Issuer)
	}
}

func TestOIDCValidator_ExpiredToken(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"aud": "test-client",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	_, err = validator.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestOIDCValidator_WrongIssuer(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"iss": "https://evil.example.com",
		"sub": "user-123",
		"aud": "test-client",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	_, err = validator.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

func TestOIDCValidator_WrongAudience(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"aud": "other-client",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	_, err = validator.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for wrong audience, got nil")
	}
}

func TestOIDCValidator_BadSignature(t *testing.T) {
	issuer, _, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	// Generate a separate attacker key to sign a forged token.
	attackerKey := generateTestRSAKey(t)

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"aud": "test-client",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	// Sign with attacker key, but use the legitimate kid so the validator
	// picks the IdP's public key and signature verification must fail.
	raw := mintTestJWT(t, attackerKey, "test-key-1", claims)

	_, err = validator.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestOIDCValidator_MissingConfig(t *testing.T) {
	cases := []struct {
		name   string
		config map[string]interface{}
	}{
		{"missing issuer", map[string]interface{}{"client_id": "x"}},
		{"missing client_id", map[string]interface{}{"issuer": "https://example.com"}},
		{"empty config", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Provider{TenantID: "t", Type: ProviderOIDC, Config: tc.config}
			_, err := newOIDCValidator(p, nil)
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestOIDCValidator_NotYetValid(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"aud": "test-client",
		"nbf": time.Now().Add(1 * time.Hour).Unix(),
		"exp": time.Now().Add(2 * time.Hour).Unix(),
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	_, err = validator.Validate(context.Background(), raw)
	if err == nil {
		t.Fatal("expected error for token not yet valid (nbf), got nil")
	}
}

func TestOIDCValidator_DiscoveryUnreachable(t *testing.T) {
	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config: map[string]interface{}{
			"issuer":    "http://127.0.0.1:1", // nothing listening
			"client_id": "test-client",
		},
	}
	_, err := newOIDCValidator(provider, nil)
	if err == nil {
		t.Fatal("expected discovery error, got nil")
	}
}

func TestOIDCValidator_JWKSCacheReuse(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	now := time.Now()
	baseClaims := jwt.MapClaims{
		"iss": issuer,
		"aud": "test-client",
		"exp": now.Add(1 * time.Hour).Unix(),
		"iat": now.Unix(),
	}

	// Validate multiple tokens - JWKS should be cached after first fetch.
	for i := 0; i < 3; i++ {
		sub := fmt.Sprintf("user-%d", i)
		c := jwt.MapClaims{}
		for k, v := range baseClaims {
			c[k] = v
		}
		c["sub"] = sub
		c["email"] = sub + "@example.com"
		raw := mintTestJWT(t, signKey, "test-key-1", c)
		got, err := validator.Validate(context.Background(), raw)
		if err != nil {
			t.Fatalf("token %d: unexpected error: %v", i, err)
		}
		if got.Subject != sub {
			t.Errorf("token %d: expected sub %q, got %q", i, sub, got.Subject)
		}
	}
}

func TestOIDCValidator_AudienceSlice(t *testing.T) {
	issuer, signKey, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "test-client"},
	}
	validator, err := newOIDCValidator(provider, nil)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Some providers send aud as a JSON array (multiple audiences).
	// golang-jwt handles this, but we should verify XinWiki's client_id
	// is in the list.
	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-999",
		"aud": []string{"other-aud", "test-client"},
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"email": "multi@example.com",
	}
	raw := mintTestJWT(t, signKey, "test-key-1", claims)

	got, err := validator.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected token with aud array to validate, got error: %v", err)
	}
	if got.Subject != "user-999" {
		t.Errorf("unexpected subject: %q", got.Subject)
	}
}

// TestOIDCValidator_BuildSSOURL verifies the authorization URL builder
// includes standard OIDC parameters.
func TestOIDCValidator_BuildSSOURL(t *testing.T) {
	issuer, _, cleanup := setupTestOIDCProvider(t)
	defer cleanup()

	provider := &Provider{
		ID:       "oidc-url-test",
		TenantID: "tenant-1",
		Type:     ProviderOIDC,
		Config: map[string]interface{}{
			"issuer":                 issuer,
			"client_id":              "my-client",
			"authorization_endpoint": issuer + "/authorize",
		},
	}

	url, err := buildOIDCAuthorizationURL(context.Background(), provider, "https://wiki.example.com/callback", "state-xyz", "openid email profile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "response_type=code") {
		t.Errorf("expected response_type=code in URL, got %s", url)
	}
	if !strings.Contains(url, "client_id=my-client") {
		t.Errorf("expected client_id in URL, got %s", url)
	}
	if !strings.Contains(url, "redirect_uri=") {
		t.Errorf("expected redirect_uri in URL, got %s", url)
	}
	if !strings.Contains(url, "state=state-xyz") {
		t.Errorf("expected state in URL, got %s", url)
	}
	if !strings.Contains(url, "scope=openid") {
		t.Errorf("expected scope in URL, got %s", url)
	}
}
