package uum

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mockRepo is a minimal Repository implementation for ValidateOIDCToken
// integration tests.
type mockRepoForOIDC struct {
	providers []*Provider
}

func (m *mockRepoForOIDC) CreateProvider(_ context.Context, p *Provider) error  { return nil }
func (m *mockRepoForOIDC) UpdateProvider(_ context.Context, p *Provider) error  { return nil }
func (m *mockRepoForOIDC) DeleteProvider(_ context.Context, _, _ string) error  { return nil }
func (m *mockRepoForOIDC) GetProvider(_ context.Context, _, id string) (*Provider, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}
func (m *mockRepoForOIDC) ListProviders(_ context.Context, _ string) ([]*Provider, error) {
	return m.providers, nil
}
func (m *mockRepoForOIDC) CreateSyncEvent(_ context.Context, _ *SyncEvent) error { return nil }
func (m *mockRepoForOIDC) ListSyncEvents(_ context.Context, _ string, _, _ int) ([]*SyncEvent, error) {
	return nil, nil
}
func (m *mockRepoForOIDC) UpdateSyncEventStatus(_ context.Context, _, _, _ string) error { return nil }

// mockUserRepo tracks provisioned users for assertions about JIT behavior.
type mockUserRepoForOIDC struct {
	provisioned []map[string]interface{}
}

func (m *mockUserRepoForOIDC) UpsertUser(_ context.Context, _ string, data map[string]interface{}) (string, error) {
	m.provisioned = append(m.provisioned, data)
	id, _ := data["external_id"].(string)
	if id == "" {
		id = "new-user-" + randomShortHex()
	}
	return id, nil
}
func (m *mockUserRepoForOIDC) DisableUser(_ context.Context, _, _ string) error               { return nil }
func (m *mockUserRepoForOIDC) DeleteUser(_ context.Context, _, _ string) error                { return nil }
func (m *mockUserRepoForOIDC) FindUserByExternalID(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockUserRepoForOIDC) FindUserByEmail(_ context.Context, email string) (string, string, error) {
	// Return no existing user to trigger JIT provisioning.
	return "", "", nil
}
func (m *mockUserRepoForOIDC) UpsertDepartment(_ context.Context, _ string, _ map[string]interface{}) (string, error) {
	return "dept-1", nil
}
func (m *mockUserRepoForOIDC) FindDepartmentByName(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockUserRepoForOIDC) FindDepartmentByExternalID(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockUserRepoForOIDC) AddUserToDepartment(_ context.Context, _, _, _ string) error { return nil }
func (m *mockUserRepoForOIDC) RemoveUserFromDepartment(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockUserRepoForOIDC) AssignDefaultRole(_ context.Context, _, _ string) error { return nil }

func randomShortHex() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// setupFullOIDCEnv starts an OIDC test server and returns (issuer, signKey, cleanup).
func setupFullOIDCEnv(t *testing.T) (string, *rsa.PrivateKey, func()) {
	t.Helper()
	signKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &signKey.PublicKey
	testJWK := oidcJWK{
		Kty: "RSA", Kid: "test-kid", Use: "sig", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(oidcWellKnown{
			Issuer:                srv.URL,
			JwksURI:               srv.URL + "/.well-known/jwks.json",
			AuthorizationEndpoint: srv.URL + "/authorize",
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(oidcJWKS{Keys: []oidcJWK{testJWK}})
	})
	srv = httptest.NewServer(mux)
	return srv.URL, signKey, srv.Close
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims, kid string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestService_ValidateOIDCToken_SuccessAndJIT(t *testing.T) {
	issuer, signKey, cleanup := setupFullOIDCEnv(t)
	defer cleanup()

	provider := &Provider{
		ID:       "oidc-real",
		TenantID: "tenant-x",
		Type:     ProviderOIDC,
		Name:     "RealOIDC",
		Status:   StatusActive,
		Config: map[string]interface{}{
			"issuer":    issuer,
			"client_id": "app1",
		},
	}
	repo := &mockRepoForOIDC{providers: []*Provider{provider}}
	userRepo := &mockUserRepoForOIDC{}

	svc := NewService(repo, userRepo)

	now := time.Now()
	raw := signToken(t, signKey, jwt.MapClaims{
		"iss":            issuer,
		"sub":            "user-001",
		"aud":            "app1",
		"exp":            now.Add(1 * time.Hour).Unix(),
		"iat":            now.Unix(),
		"email":          "bob@example.com",
		"email_verified": true,
		"name":           "Bob",
		"preferred_username": "bob",
		"groups":         []string{"team-a"},
	}, "test-kid")

	got, err := svc.ValidateOIDCToken(context.Background(), "tenant-x", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Subject != "user-001" {
		t.Errorf("expected sub user-001, got %q", got.Subject)
	}
	if got.Email != "bob@example.com" {
		t.Errorf("expected email bob@example.com, got %q", got.Email)
	}
	if len(userRepo.provisioned) != 1 {
		t.Fatalf("expected JIT provisioning to happen, got %d calls", len(userRepo.provisioned))
	}
	if userRepo.provisioned[0]["email"] != "bob@example.com" {
		t.Errorf("provisioned user has wrong email: %v", userRepo.provisioned[0])
	}
}

func TestService_ValidateOIDCToken_EmptyToken(t *testing.T) {
	svc := NewService(&mockRepoForOIDC{}, &mockUserRepoForOIDC{})
	_, err := svc.ValidateOIDCToken(context.Background(), "t", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestService_ValidateOIDCToken_NoMatchingProvider(t *testing.T) {
	issuer, signKey, cleanup := setupFullOIDCEnv(t)
	defer cleanup()

	// Provider has a different issuer than the token.
	provider := &Provider{
		ID:       "oidc-other",
		TenantID: "tenant-x",
		Type:     ProviderOIDC,
		Status:   StatusActive,
		Config:   map[string]interface{}{"issuer": "https://other.example.com", "client_id": "app"},
	}
	repo := &mockRepoForOIDC{providers: []*Provider{provider}}
	svc := NewService(repo, &mockUserRepoForOIDC{})

	now := time.Now()
	raw := signToken(t, signKey, jwt.MapClaims{
		"iss": issuer, "sub": "u", "aud": "app",
		"exp": now.Add(1 * time.Hour).Unix(), "iat": now.Unix(),
	}, "test-kid")

	_, err := svc.ValidateOIDCToken(context.Background(), "tenant-x", raw)
	if err == nil {
		t.Fatal("expected error when no provider matches issuer")
	}
}

func TestService_ValidateOIDCToken_InactiveProviderRejected(t *testing.T) {
	issuer, signKey, cleanup := setupFullOIDCEnv(t)
	defer cleanup()

	provider := &Provider{
		ID:       "oidc-inactive",
		TenantID: "tenant-x",
		Type:     ProviderOIDC,
		Status:   StatusInactive, // NOT active
		Config:   map[string]interface{}{"issuer": issuer, "client_id": "app1"},
	}
	repo := &mockRepoForOIDC{providers: []*Provider{provider}}
	svc := NewService(repo, &mockUserRepoForOIDC{})

	now := time.Now()
	raw := signToken(t, signKey, jwt.MapClaims{
		"iss": issuer, "sub": "u", "aud": "app1",
		"exp": now.Add(1 * time.Hour).Unix(), "iat": now.Unix(),
	}, "test-kid")

	_, err := svc.ValidateOIDCToken(context.Background(), "tenant-x", raw)
	if err == nil {
		t.Fatal("expected error when provider is not active")
	}
}

func TestService_BuildSSOURL_OIDC(t *testing.T) {
	provider := &Provider{
		ID:       "oidc-url",
		TenantID: "t",
		Type:     ProviderOIDC,
		Status:   StatusActive,
		Config: map[string]interface{}{
			"issuer":                 "https://idp.example.com",
			"client_id":              "abc",
			"authorization_endpoint": "https://idp.example.com/oauth2/authorize",
		},
	}
	repo := &mockRepoForOIDC{providers: []*Provider{provider}}
	svc := NewService(repo, &mockUserRepoForOIDC{})

	u, err := svc.BuildSSOURL(context.Background(), "t", "oidc-url", "https://app/cb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u == "" {
		t.Fatal("expected non-empty URL")
	}
	// Must contain state parameter (CSRF).
	if !contains(u, "state=") {
		t.Errorf("expected state param in URL, got %s", u)
	}
	if !contains(u, "client_id=abc") {
		t.Errorf("expected client_id in URL, got %s", u)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
