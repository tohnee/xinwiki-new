package uum

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateSAMLAssertion_RejectsAllAssertionsUntilImplemented(t *testing.T) {
	svc := &service{}

	assertion, err := svc.ValidateSAMLAssertion(context.Background(), "tenant-1", "fake-saml-response")
	assert.Error(t, err, "SAML validation must return an error until real signature verification is implemented")
	assert.Nil(t, assertion, "must not return a SAMLAssertion for unvalidated input")
	// Make sure the error message makes clear that SSO is not implemented
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "not yet implemented") ||
		strings.Contains(strings.ToLower(err.Error()), "not implemented"),
		"error should clearly indicate SAML SSO is not implemented, got: %v", err)
}

func TestValidateOIDCToken_RejectsAllTokensUntilImplemented(t *testing.T) {
	svc := &service{}

	tok, err := svc.ValidateOIDCToken(context.Background(), "tenant-1", "fake.oidc.token")
	assert.Error(t, err, "OIDC validation must return an error until real JWT verification is implemented")
	assert.Nil(t, tok, "must not return an OIDCToken for unvalidated input")
	assert.True(t, strings.Contains(strings.ToLower(err.Error()), "not yet implemented") ||
		strings.Contains(strings.ToLower(err.Error()), "not implemented"),
		"error should clearly indicate OIDC SSO is not implemented, got: %v", err)
}
