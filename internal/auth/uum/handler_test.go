package uum

import (
	"context"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestValidateSAMLAssertion_RejectsAllAssertionsUntilImplemented(t *testing.T) {
	svc := &service{}

	assertion, err := svc.ValidateSAMLAssertion(context.Background(), "tenant-1", "fake-saml-response")
	assert.Error(t, err, "SAML validation must return an error until real signature verification is implemented")
	assert.Nil(t, assertion, "must not return a SAMLAssertion for unvalidated input")
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "not yet implemented") ||
		strings.Contains(strings.ToLower(err.Error()), "not implemented"),
		"error should clearly indicate SAML SSO is not implemented, got: %v", err)
}

func TestValidateOIDCToken_EmptyTokenRejected(t *testing.T) {
	svc := &service{}

	tok, err := svc.ValidateOIDCToken(context.Background(), "tenant-1", "")
	assert.Error(t, err, "empty token must be rejected")
	assert.Nil(t, tok, "must not return a token for empty input")
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "required"),
		"error should indicate token is required, got: %v", err)
}

func TestValidateOIDCToken_MalformedTokenRejected(t *testing.T) {
	svc := &service{repo: &mockRepoForOIDC{}}

	tok, err := svc.ValidateOIDCToken(context.Background(), "tenant-1", "not.a.valid.token}")
	assert.Error(t, err, "malformed token must be rejected")
	assert.Nil(t, tok, "must not return a token for malformed input")
}

func TestValidateOIDCToken_NoMatchingProviderRejected(t *testing.T) {
	svc := &service{
		repo:           &mockRepoForOIDC{providers: []*Provider{}},
		oidcValidators: make(map[string]*oidcValidator),
	}

	claims := &oidcClaims{
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "https://unknown.example.com"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	rawTok, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	tok, err := svc.ValidateOIDCToken(context.Background(), "tenant-1", rawTok)
	assert.Error(t, err, "token with unknown issuer must be rejected")
	assert.Nil(t, tok, "must not return a token when no provider matches")
	assert.True(t, strings.Contains(err.Error(), "no active OIDC provider"),
		"error should indicate no matching provider, got: %v", err)
}
