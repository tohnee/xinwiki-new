package uum

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultJWKSCacheTTL = 1 * time.Hour
	defaultHTTPTimeout  = 10 * time.Second
)

// oidcWellKnown represents the subset of OIDC discovery document we need.
type oidcWellKnown struct {
	Issuer                string `json:"issuer"`
	JwksURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

// oidcJWK represents a single JSON Web Key.
type oidcJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

// oidcJWKS represents a JWKS response.
type oidcJWKS struct {
	Keys []oidcJWK `json:"keys"`
}

// cachedKeySet holds a fetched JWKS and its fetch time for TTL-based invalidation.
type cachedKeySet struct {
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// oidcValidator performs OIDC ID Token validation using OIDC discovery and
// JWKS-based signature verification. It is safe for concurrent use.
type oidcValidator struct {
	issuer    string
	clientID  string
	httpCli   *http.Client
	jwksURI   string

	mu        sync.RWMutex
	cache     *cachedKeySet
	cacheTTL  time.Duration
}

// newOIDCValidator creates a new OIDC validator for the given provider.
// It performs OIDC discovery (fetching .well-known/openid-configuration) to
// locate the JWKS endpoint. If httpCli is nil a default client is used.
func newOIDCValidator(provider *Provider, httpCli *http.Client) (*oidcValidator, error) {
	if provider.Config == nil {
		return nil, errors.New("OIDC provider config is required")
	}
	issuer, _ := provider.Config["issuer"].(string)
	if issuer == "" {
		return nil, errors.New("OIDC issuer is required in provider config")
	}
	clientID, _ := provider.Config["client_id"].(string)
	if clientID == "" {
		return nil, errors.New("OIDC client_id is required in provider config")
	}
	if httpCli == nil {
		httpCli = &http.Client{Timeout: defaultHTTPTimeout}
	}

	v := &oidcValidator{
		issuer:   issuer,
		clientID: clientID,
		httpCli:  httpCli,
		cacheTTL: defaultJWKSCacheTTL,
	}

	// Allow operator to override jwks_uri directly (useful for proxies/testing)
	if override, ok := provider.Config["jwks_uri"].(string); ok && override != "" {
		v.jwksURI = override
	} else {
		// Run discovery.
		wk, err := v.fetchDiscovery(context.Background())
		if err != nil {
			return nil, fmt.Errorf("OIDC discovery failed: %w", err)
		}
		if wk.Issuer != issuer {
			return nil, fmt.Errorf("OIDC discovery issuer mismatch: expected %q, got %q", issuer, wk.Issuer)
		}
		v.jwksURI = wk.JwksURI
	}
	if v.jwksURI == "" {
		return nil, errors.New("OIDC jwks_uri not found in discovery document")
	}
	return v, nil
}

// Validate performs full JWT validation per OIDC Core spec:
//   - signature verification via JWKS
//   - iss, aud, exp, iat, nbf claim validation
//   - returns parsed OIDCToken on success
func (v *oidcValidator) Validate(ctx context.Context, rawToken string) (*OIDCToken, error) {
	// First pass: parse header (without verification) to extract kid.
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}))
	tok, _, err := parser.ParseUnverified(rawToken, &oidcClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	kid, _ := tok.Header["kid"].(string)
	if kid == "" {
		return nil, errors.New("token header missing 'kid'")
	}

	// Resolve signing key (with cache).
	key, err := v.getSigningKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve signing key: %w", err)
	}

	// Second pass: verify signature + standard claims.
	claims := &oidcClaims{}
	_, err = jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (interface{}, error) {
		// Reject unexpected alg values.
		alg := t.Method.Alg()
		switch alg {
		case "RS256", "RS384", "RS512":
			return key, nil
		default:
			return nil, fmt.Errorf("unsupported signing algorithm: %s", alg)
		}
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	// Verify issuer.
	if claims.Issuer != v.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %q, got %q", v.issuer, claims.Issuer)
	}

	// Verify audience (must contain our client_id; handles both string and []string).
	if !claims.containsAudience(v.clientID) {
		return nil, fmt.Errorf("invalid audience: expected %q in aud claim", v.clientID)
	}

	now := time.Now()
	// exp / iat are validated by jwt v5 library, but we add an explicit check
	// for nbf (leeway of 30 seconds for clock skew).
	if claims.NotBefore != nil && claims.NotBefore.After(now.Add(30*time.Second)) {
		return nil, errors.New("token is not yet valid (nbf)")
	}

	// Map claims to OIDCToken.
	out := &OIDCToken{
		Issuer:            claims.Issuer,
		Subject:           claims.Subject,
		Email:             claims.Email,
		Name:              claims.Name,
		PreferredUsername: claims.PreferredUsername,
		Groups:            claims.Groups,
		Department:        claims.Department,
		EmailVerified:     claims.EmailVerified,
	}
	if claims.ExpiresAt != nil {
		out.Expiration = claims.ExpiresAt.Unix()
	}
	if claims.IssuedAt != nil {
		out.IssuedAt = claims.IssuedAt.Unix()
	}
	if len(claims.Audience) == 1 {
		out.Audience = claims.Audience[0]
	}
	return out, nil
}

// oidcClaims is the set of JWT claims we parse. It embeds jwt.RegisteredClaims
// for standard validation and adds OIDC-specific fields.
type oidcClaims struct {
	jwt.RegisteredClaims
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
	Department        string   `json:"department"`
}

func (c *oidcClaims) containsAudience(target string) bool {
	for _, a := range c.Audience {
		if a == target {
			return true
		}
	}
	return false
}

// getSigningKey returns the RSA public key for the given kid, fetching/updating
// JWKS as needed.
func (v *oidcValidator) getSigningKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Fast path: cache hit + fresh.
	v.mu.RLock()
	if v.cache != nil && time.Since(v.cache.fetched) < v.cacheTTL {
		if key, ok := v.cache.keys[kid]; ok {
			v.mu.RUnlock()
			return key, nil
		}
	}
	v.mu.RUnlock()

	// Slow path: re-fetch JWKS.
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock.
	if v.cache != nil && time.Since(v.cache.fetched) < v.cacheTTL {
		if key, ok := v.cache.keys[kid]; ok {
			return key, nil
		}
	}

	jwks, err := v.fetchJWKS(ctx)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	v.cache = &cachedKeySet{keys: keys, fetched: time.Now()}

	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("no key found for kid %q", kid)
	}
	return key, nil
}

func (v *oidcValidator) fetchDiscovery(ctx context.Context) (*oidcWellKnown, error) {
	// discovery URL is issuer + /.well-known/openid-configuration
	// Allow trailing slash in issuer; trim it for URL assembly.
	issuer := v.issuer
	discoveryURL := issuer
	if len(discoveryURL) > 0 && discoveryURL[len(discoveryURL)-1] == '/' {
		discoveryURL = discoveryURL[:len(discoveryURL)-1]
	}
	discoveryURL += "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("discovery endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	var wk oidcWellKnown
	if err := json.NewDecoder(resp.Body).Decode(&wk); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}
	return &wk, nil
}

func (v *oidcValidator) fetchJWKS(ctx context.Context) (*oidcJWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURI, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("JWKS endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	var jwks oidcJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}
	return &jwks, nil
}

// parseRSAPublicKey decodes the base64url-encoded n and e components of an RSA
// public key from a JWK.
func parseRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWK n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWK e: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid JWK exponent (zero)")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// buildOIDCAuthorizationURL constructs an OIDC Authorization Code Flow URL with
// standard parameters.
func buildOIDCAuthorizationURL(
	_ context.Context,
	provider *Provider,
	redirectURI, state, scope string,
) (string, error) {
	authzEndpoint, _ := provider.Config["authorization_endpoint"].(string)
	if authzEndpoint == "" {
		// Fall back to issuer + /authorize convention if not set.
		issuer, _ := provider.Config["issuer"].(string)
		if issuer == "" {
			return "", errors.New("authorization_endpoint or issuer is required in OIDC provider config")
		}
		if len(issuer) > 0 && issuer[len(issuer)-1] == '/' {
			issuer = issuer[:len(issuer)-1]
		}
		authzEndpoint = issuer + "/authorize"
	}
	clientID, _ := provider.Config["client_id"].(string)
	if scope == "" {
		scope = "openid email profile"
	}
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		authzEndpoint, clientID, url.QueryEscape(redirectURI), url.QueryEscape(scope), url.QueryEscape(state)), nil
}
