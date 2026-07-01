package uum

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Tencent/XinWiki/internal/logger"
)

// --- Provider Management ---

func (s *service) CreateProvider(ctx context.Context, provider *Provider) error {
	if provider.TenantID == "" || provider.Name == "" || provider.Type == "" {
		return errors.New("tenant_id, name, and type are required")
	}
	if provider.ID == "" {
		provider.ID = newID()
	}
	now := time.Now()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	if provider.Status == "" {
		provider.Status = StatusPending
	}
	if provider.SyncInterval == 0 {
		provider.SyncInterval = 3600
	}
	if provider.AttributeMap == nil {
		provider.AttributeMap = getDefaultAttributeMap(provider.Type)
	}
	return s.repo.CreateProvider(ctx, provider)
}

func (s *service) UpdateProvider(ctx context.Context, provider *Provider) error {
	if provider.ID == "" {
		return errors.New("provider id is required")
	}
	provider.UpdatedAt = time.Now()
	// Invalidate cached validator so config changes (issuer, client_id) take effect.
	s.invalidateOIDCValidator(provider.ID)
	return s.repo.UpdateProvider(ctx, provider)
}

func (s *service) DeleteProvider(ctx context.Context, tenantID, providerID string) error {
	// Stop any running scheduler for this provider
	s.StopSyncSchedulerForProvider(providerID)
	// Invalidate cached OIDC validator so next creation starts fresh.
	s.invalidateOIDCValidator(providerID)
	return s.repo.DeleteProvider(ctx, tenantID, providerID)
}

func (s *service) GetProvider(ctx context.Context, tenantID, providerID string) (*Provider, error) {
	return s.repo.GetProvider(ctx, tenantID, providerID)
}

func (s *service) ListProviders(ctx context.Context, tenantID string) ([]*Provider, error) {
	return s.repo.ListProviders(ctx, tenantID)
}

func (s *service) TestProviderConnection(ctx context.Context, tenantID, providerID string) error {
	provider, err := s.repo.GetProvider(ctx, tenantID, providerID)
	if err != nil {
		return err
	}

	switch provider.Type {
	case ProviderSCIM:
		return s.testSCIMConnection(ctx, provider)
	case ProviderSAML:
		return s.testSAMLConnection(ctx, provider)
	case ProviderOIDC:
		return s.testOIDCConnection(ctx, provider)
	case ProviderLDAP:
		return s.testLDAPConnection(ctx, provider)
	default:
		return fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

// --- SCIM Operations ---

func (s *service) HandleSCIMUser(ctx context.Context, tenantID string, _ ProviderType, user *SCIMUser) error {
	event := &SyncEvent{
		ID:         newID(),
		TenantID:   tenantID,
		ProviderID: "", // Should be set by caller
		EventType:  "user.upsert",
		ObjectType: "user",
		ObjectID:   user.ID,
		ExternalID: user.ExternalID,
		Data:       user,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	if err := s.repo.CreateSyncEvent(ctx, event); err != nil {
		return err
	}
	return s.processUserEvent(ctx, tenantID, event)
}

func (s *service) HandleSCIMDepartment(ctx context.Context, tenantID string, dept *SCIMDepartment) error {
	event := &SyncEvent{
		ID:         newID(),
		TenantID:   tenantID,
		EventType:  "dept.upsert",
		ObjectType: "department",
		ObjectID:   dept.ID,
		Data:       dept,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}
	if err := s.repo.CreateSyncEvent(ctx, event); err != nil {
		return err
	}
	return s.processDeptEvent(ctx, tenantID, event)
}

func (s *service) SyncAllUsers(ctx context.Context, tenantID, providerID string) (*SyncResult, error) {
	provider, err := s.repo.GetProvider(ctx, tenantID, providerID)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{
		ProviderID:  providerID,
		StartedAt:   time.Now(),
	}

	logger.Infof(ctx, "[uum] starting full sync for provider %s (%s) in tenant %s", providerID, provider.Type, tenantID)

	// Mark sync start
	now := time.Now()
	provider.LastSyncAt = &now
	provider.LastSyncStatus = "in_progress"
	_ = s.repo.UpdateProvider(ctx, provider)

	// Sync logic would be implemented based on provider type
	// For brevity, this is the framework - actual sync requires client implementations

	result.CompletedAt = time.Now()

	// Update provider with sync result
	now = time.Now()
	provider.LastSyncAt = &now
	if len(result.Errors) == 0 {
		provider.LastSyncStatus = "success"
		provider.LastSyncError = ""
		provider.Status = StatusActive
	} else {
		provider.LastSyncStatus = "partial"
		provider.LastSyncError = fmt.Sprintf("%d errors occurred", len(result.Errors))
	}
	_ = s.repo.UpdateProvider(ctx, provider)

	return result, nil
}

// --- SSO Authentication ---

func (s *service) ValidateSAMLAssertion(ctx context.Context, tenantID string, samlResponse string) (*SAMLAssertion, error) {
	// CRITICAL SECURITY: SAML assertion validation requires a proper SAML 2.0
	// library (e.g. github.com/russellhaering/gosaml2) to verify the XML-DSig
	// signature, Issuer, InResponseTo, NotBefore/NotOnOrAfter, etc.
	// Returning a "not implemented" error prevents authentication bypass via
	// forged assertions. Remove this error only after proper SAML validation
	// is integrated.
	_ = ctx
	_ = tenantID
	_ = samlResponse
	return nil, fmt.Errorf("SAML SSO is not yet implemented; do not accept unvalidated assertions")
}

func (s *service) ValidateOIDCToken(ctx context.Context, tenantID string, rawToken string) (*OIDCToken, error) {
	if rawToken == "" {
		return nil, errors.New("OIDC token is required")
	}

	// First pass: parse the unverified token to learn its issuer (iss) so we
	// can locate the correct OIDC provider for this tenant without requiring
	// the caller to pass a providerID.
	parsed, _, err := jwt.NewParser().ParseUnverified(rawToken, &oidcClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse OIDC token: %w", err)
	}
	claims, ok := parsed.Claims.(*oidcClaims)
	if !ok || claims.Issuer == "" {
		return nil, errors.New("OIDC token missing 'iss' claim")
	}
	tokenIssuer := claims.Issuer

	// List OIDC providers for the tenant and find the one matching the issuer.
	providers, err := s.repo.ListProviders(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list UUM providers: %w", err)
	}

	var matchedProvider *Provider
	for _, p := range providers {
		if p.Type != ProviderOIDC || p.Status != StatusActive {
			continue
		}
		iss, _ := p.Config["issuer"].(string)
		if iss == tokenIssuer {
			matchedProvider = p
			break
		}
	}
	if matchedProvider == nil {
		return nil, fmt.Errorf("no active OIDC provider found for issuer %q in tenant %q", tokenIssuer, tenantID)
	}

	// Get or create a cached OIDC validator for this provider.
	validator, err := s.getOrCreateOIDCValidator(ctx, matchedProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC validator: %w", err)
	}

	validated, err := validator.Validate(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	// Auto-provision user on successful SSO login (JIT provisioning).
	if err := s.provisionUserFromOIDCToken(ctx, tenantID, matchedProvider, validated); err != nil {
		logger.Warnf(ctx, "[uum] failed to auto-provision user %s from OIDC token: %v", validated.Subject, err)
		// Provisioning failure must NOT block login.
	}

	return validated, nil
}

// getOrCreateOIDCValidator returns a cached oidcValidator for the given
// provider, creating one on first use. Safe for concurrent use.
func (s *service) getOrCreateOIDCValidator(ctx context.Context, provider *Provider) (*oidcValidator, error) {
	s.oidcValidatorsMu.RLock()
	if v, ok := s.oidcValidators[provider.ID]; ok {
		s.oidcValidatorsMu.RUnlock()
		return v, nil
	}
	s.oidcValidatorsMu.RUnlock()

	s.oidcValidatorsMu.Lock()
	defer s.oidcValidatorsMu.Unlock()

	// Double-check after acquiring write lock.
	if v, ok := s.oidcValidators[provider.ID]; ok {
		return v, nil
	}
	v, err := newOIDCValidator(provider, s.httpClient)
	if err != nil {
		return nil, err
	}
	s.oidcValidators[provider.ID] = v
	return v, nil
}

// invalidateOIDCValidator removes a cached validator (e.g. after provider update/delete).
func (s *service) invalidateOIDCValidator(providerID string) {
	s.oidcValidatorsMu.Lock()
	defer s.oidcValidatorsMu.Unlock()
	delete(s.oidcValidators, providerID)
}

func (s *service) BuildSSOURL(ctx context.Context, tenantID, providerID string, redirectURI string) (string, error) {
	provider, err := s.repo.GetProvider(ctx, tenantID, providerID)
	if err != nil {
		return "", err
	}

	switch provider.Type {
	case ProviderSAML:
		return s.buildSAMLSSOURL(ctx, provider, redirectURI)
	case ProviderOIDC:
		// Generate a random state for CSRF protection. Callers that need to
		// correlate the callback should retrieve the state via a separate
		// mechanism (e.g. session storage) in the future; for now we use a
		// random nonce.
		state := generateOIDCState()
		return buildOIDCAuthorizationURL(ctx, provider, redirectURI, state, "")
	default:
		return "", fmt.Errorf("SSO not supported for provider type: %s", provider.Type)
	}
}

// generateOIDCState produces a cryptographically random 16-byte hex state
// parameter for OIDC CSRF protection.
func generateOIDCState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fall back to UUID if crypto rand fails (extremely unlikely).
		return newID()
	}
	return fmt.Sprintf("%x", b)
}

// --- User Provisioning ---

func (s *service) ProvisionUser(ctx context.Context, tenantID string, userData map[string]interface{}) error {
	_, err := s.userRepo.UpsertUser(ctx, tenantID, userData)
	if err != nil {
		return err
	}
	// Assign default role
	userID, _ := userData["id"].(string)
	if userID != "" {
		_ = s.userRepo.AssignDefaultRole(ctx, tenantID, userID)
	}
	return nil
}

func (s *service) DeprovisionUser(ctx context.Context, tenantID, externalUserID string) error {
	userID, err := s.userRepo.FindUserByExternalID(ctx, tenantID, externalUserID)
	if err != nil {
		return err
	}
	return s.userRepo.DisableUser(ctx, tenantID, userID)
}

func (s *service) SyncUserDepartments(ctx context.Context, tenantID, userID string, departments []string) error {
	// This would clear existing department memberships and set new ones
	// Full implementation would get current memberships, diff, and apply changes
	for _, deptName := range departments {
		deptID, err := s.userRepo.FindDepartmentByName(ctx, tenantID, deptName)
		if err != nil {
			// Create department if it doesn't exist
			deptID, err = s.userRepo.UpsertDepartment(ctx, tenantID, map[string]interface{}{
				"name": deptName,
			})
			if err != nil {
				logger.Warnf(ctx, "[uum] failed to create department %s: %v", deptName, err)
				continue
			}
		}
		_ = s.userRepo.AddUserToDepartment(ctx, tenantID, userID, deptID)
	}
	return nil
}

// --- Sync Events ---

func (s *service) ListSyncEvents(ctx context.Context, tenantID string, limit, offset int) ([]*SyncEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListSyncEvents(ctx, tenantID, limit, offset)
}

func (s *service) RetrySyncEvent(ctx context.Context, tenantID, eventID string) error {
	events, err := s.repo.ListSyncEvents(ctx, tenantID, 1, 0)
	if err != nil {
		return err
	}
	var event *SyncEvent
	for _, e := range events {
		if e.ID == eventID {
			event = e
			break
		}
	}
	if event == nil {
		return errors.New("sync event not found")
	}
	if event.ObjectType == "user" {
		return s.processUserEvent(ctx, tenantID, event)
	}
	return s.processDeptEvent(ctx, tenantID, event)
}

// --- Sync Scheduler ---

func (s *service) StartSyncScheduler(ctx context.Context) error {
	// This would start background sync for all providers with sync enabled
	// Implementation would use tickers per provider based on SyncInterval
	logger.Info(ctx, "[uum] sync scheduler started")
	return nil
}

func (s *service) StopSyncScheduler(ctx context.Context) error {
	for providerID, cancel := range s.schedulers {
		cancel()
		delete(s.schedulers, providerID)
	}
	logger.Info(ctx, "[uum] sync scheduler stopped")
	return nil
}

func (s *service) StopSyncSchedulerForProvider(providerID string) {
	if cancel, ok := s.schedulers[providerID]; ok {
		cancel()
		delete(s.schedulers, providerID)
	}
}

// --- Internal Helpers ---

func (s *service) processUserEvent(ctx context.Context, tenantID string, event *SyncEvent) error {
	user, ok := event.Data.(*SCIMUser)
	if !ok {
		err := errors.New("invalid user data in event")
		_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "failed", err.Error())
		return err
	}

	// Map SCIM attributes to XinWiki user model
	userData := s.mapSCIMUser(user)

	userID, err := s.userRepo.UpsertUser(ctx, tenantID, userData)
	if err != nil {
		_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "failed", err.Error())
		return err
	}

	// Assign default role if new user
	if existingID, _ := s.userRepo.FindUserByExternalID(ctx, tenantID, user.ExternalID); existingID == "" {
		_ = s.userRepo.AssignDefaultRole(ctx, tenantID, userID)
	}

	// Sync department membership
	if user.Department != "" {
		depts := strings.Split(user.Department, ",")
		_ = s.SyncUserDepartments(ctx, tenantID, userID, depts)
	}

	now := time.Now()
	event.Status = "processed"
	event.ProcessedAt = &now
	_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "processed", "")
	return nil
}

func (s *service) processDeptEvent(ctx context.Context, tenantID string, event *SyncEvent) error {
	dept, ok := event.Data.(*SCIMDepartment)
	if !ok {
		err := errors.New("invalid department data in event")
		_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "failed", err.Error())
		return err
	}

	deptData := map[string]interface{}{
		"name": dept.DisplayName,
		"external_id": dept.ID,
	}

	_, err := s.userRepo.UpsertDepartment(ctx, tenantID, deptData)
	if err != nil {
		_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "failed", err.Error())
		return err
	}

	now := time.Now()
	event.Status = "processed"
	event.ProcessedAt = &now
	_ = s.repo.UpdateSyncEventStatus(ctx, event.ID, "processed", "")
	return nil
}

func (s *service) mapSCIMUser(user *SCIMUser) map[string]interface{} {
	email := ""
	for _, e := range user.Emails {
		if e.Primary {
			email = e.Value
			break
		}
	}
	if email == "" && len(user.Emails) > 0 {
		email = user.Emails[0].Value
	}

	fullName := user.DisplayName
	if fullName == "" {
		fullName = strings.TrimSpace(user.Name.GivenName + " " + user.Name.FamilyName)
	}

	return map[string]interface{}{
		"external_id": user.ExternalID,
		"username":    user.UserName,
		"email":       email,
		"name":        fullName,
		"first_name":  user.Name.GivenName,
		"last_name":   user.Name.FamilyName,
		"title":       user.Title,
		"department":  user.Department,
		"active":      user.Active,
	}
}

func (s *service) provisionUserFromAssertion(ctx context.Context, tenantID string, provider *Provider, assertion *SAMLAssertion) error {
	email, _ := assertion.Attributes["email"]
	if email == "" {
		email, _ = assertion.Attributes["urn:oid:0.9.2342.19200300.100.1.3"] // email OID
	}
	if email == "" {
		return errors.New("email not found in SAML assertion")
	}

	_, userID, err := s.userRepo.FindUserByEmail(ctx, email)
	if err != nil || userID == "" {
		name, _ := assertion.Attributes["displayName"]
		userData := map[string]interface{}{
			"email": email,
			"name":  name,
			"sso_subject": assertion.SubjectID,
		}
		return s.ProvisionUser(ctx, tenantID, userData)
	}
	return nil
}

func (s *service) provisionUserFromOIDCToken(ctx context.Context, tenantID string, provider *Provider, token *OIDCToken) error {
	email := token.Email
	if email == "" {
		return errors.New("email not found in OIDC token")
	}

	_, userID, err := s.userRepo.FindUserByEmail(ctx, email)
	if err != nil || userID == "" {
		userData := map[string]interface{}{
			"email": email,
			"name":  token.Name,
			"sso_subject": token.Subject,
			"username": token.PreferredUsername,
		}
		return s.ProvisionUser(ctx, tenantID, userData)
	}
	return nil
}

func getDefaultAttributeMap(providerType ProviderType) map[string]string {
	switch providerType {
	case ProviderSAML:
		return map[string]string{
			"name":     "urn:oid:2.5.4.3",
			"email":    "urn:oid:0.9.2342.19200300.100.1.3",
			"username": "urn:oid:0.9.2342.19200300.100.1.1",
		}
	case ProviderOIDC:
		return map[string]string{
			"name":     "name",
			"email":    "email",
			"username": "preferred_username",
			"groups":   "groups",
		}
	default:
		return map[string]string{}
	}
}

// --- Provider connection test stubs ---

func (s *service) testSCIMConnection(ctx context.Context, provider *Provider) error {
	// Actual implementation would test SCIM endpoint connectivity
	logger.Infof(ctx, "[uum] SCIM connection test for provider %s", provider.ID)
	provider.Status = StatusActive
	return s.repo.UpdateProvider(ctx, provider)
}

func (s *service) testSAMLConnection(ctx context.Context, provider *Provider) error {
	logger.Infof(ctx, "[uum] SAML connection test for provider %s", provider.ID)
	provider.Status = StatusActive
	return s.repo.UpdateProvider(ctx, provider)
}

func (s *service) testOIDCConnection(ctx context.Context, provider *Provider) error {
	logger.Infof(ctx, "[uum] OIDC connection test for provider %s", provider.ID)
	provider.Status = StatusActive
	return s.repo.UpdateProvider(ctx, provider)
}

func (s *service) testLDAPConnection(ctx context.Context, provider *Provider) error {
	logger.Infof(ctx, "[uum] LDAP connection test for provider %s", provider.ID)
	provider.Status = StatusActive
	return s.repo.UpdateProvider(ctx, provider)
}

func (s *service) buildSAMLSSOURL(ctx context.Context, provider *Provider, redirectURI string) (string, error) {
	// Actual implementation would build SAML AuthnRequest URL
	ssoURL, ok := provider.Config["sso_url"].(string)
	if !ok {
		return "", errors.New("sso_url not configured in SAML provider")
	}
	return ssoURL, nil
}
