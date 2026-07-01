package types

import "testing"

// TestCanAccessArtifact_NilFailsClosed: a nil artifact never authorizes.
func TestCanAccessArtifact_NilFailsClosed(t *testing.T) {
	if CanAccessArtifact(nil, "u1", TenantRoleAdmin, false) {
		t.Errorf("nil artifact must fail-closed")
	}
}

// TestCanAccessArtifact_CreatorAlwaysPasses: the creator sees their own
// artifact regardless of sharing policy.
func TestCanAccessArtifact_CreatorAlwaysPasses(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: ArtifactSharingPrivate}
	if !CanAccessArtifact(a, "u1", TenantRoleViewer, false) {
		t.Errorf("creator must access own private artifact")
	}
}

// TestCanAccessArtifact_PrivateNonCreatorDenied: a private artifact is hidden
// from everyone except the creator (and admins).
func TestCanAccessArtifact_PrivateNonCreatorDenied(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: ArtifactSharingPrivate}
	if CanAccessArtifact(a, "u2", TenantRoleContributor, false) {
		t.Errorf("non-creator must not access a private artifact")
	}
}

// TestCanAccessArtifact_TenantSharedVisibleToMembers: a tenant-shared artifact
// is visible to any tenant member (the tenant boundary is enforced by the
// query, so within-tenant ACL is "all members").
func TestCanAccessArtifact_TenantSharedVisibleToMembers(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: ArtifactSharingTenant}
	if !CanAccessArtifact(a, "u2", TenantRoleViewer, false) {
		t.Errorf("tenant-shared artifact must be visible to any member")
	}
}

// TestCanAccessArtifact_ExplicitAllowedUser: an explicit artifact is visible
// only to the creator and the listed allowed users.
func TestCanAccessArtifact_ExplicitAllowedUser(t *testing.T) {
	a := &Artifact{
		UserID:         "u1",
		SharingPolicy:  ArtifactSharingExplicit,
		AllowedUserIDs: StringArray{"u2", "u3"},
	}
	if !CanAccessArtifact(a, "u2", TenantRoleViewer, false) {
		t.Errorf("allowed user must access explicit artifact")
	}
	if CanAccessArtifact(a, "u4", TenantRoleViewer, false) {
		t.Errorf("non-listed user must not access explicit artifact")
	}
}

// TestCanAccessArtifact_AdminBypasses: a tenant Admin+ sees every artifact in
// the tenant even if it is private to another user.
func TestCanAccessArtifact_AdminBypasses(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: ArtifactSharingPrivate}
	if !CanAccessArtifact(a, "admin", TenantRoleAdmin, false) {
		t.Errorf("admin must bypass private sharing")
	}
	if !CanAccessArtifact(a, "owner", TenantRoleOwner, false) {
		t.Errorf("owner must bypass private sharing")
	}
}

// TestCanAccessArtifact_SystemAdminBypasses: a system admin sees every
// artifact (operational access across tenants).
func TestCanAccessArtifact_SystemAdminBypasses(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: ArtifactSharingPrivate}
	if !CanAccessArtifact(a, "root", TenantRoleViewer, true) {
		t.Errorf("system admin must bypass sharing")
	}
}

// TestCanAccessArtifact_EmptyUserIDDoesNotClaimOwnership: an empty caller user
// id (e.g. a synthetic/anonymous caller) must not match an artifact whose
// creator field is also empty — empty==empty must not authorize.
func TestCanAccessArtifact_EmptyUserIDDoesNotClaimOwnership(t *testing.T) {
	a := &Artifact{UserID: "", SharingPolicy: ArtifactSharingPrivate}
	if CanAccessArtifact(a, "", TenantRoleViewer, false) {
		t.Errorf("empty user id must not claim ownership of an unowned artifact")
	}
}

// TestCanAccessArtifact_UnknownPolicyFailsClosed: a sharing policy that is not
// one of the declared constants denies (defensive against bad data).
func TestCanAccessArtifact_UnknownPolicyFailsClosed(t *testing.T) {
	a := &Artifact{UserID: "u1", SharingPolicy: "world-readable"}
	if CanAccessArtifact(a, "u2", TenantRoleViewer, false) {
		t.Errorf("unknown sharing policy must fail-closed")
	}
}

// TestArtifactType_IsValid: the declared artifact types are recognised.
func TestArtifactType_IsValid(t *testing.T) {
	for _, tt := range []ArtifactType{
		ArtifactTypePPT, ArtifactTypePDF, ArtifactTypeReport,
		ArtifactTypeChart, ArtifactTypeDiagram, ArtifactTypeMarkdown,
	} {
		if !tt.IsValid() {
			t.Errorf("%q should be a valid artifact type", tt)
		}
	}
	if ArtifactType("widget").IsValid() {
		t.Errorf("\"widget\" should not be a valid artifact type")
	}
}

// TestArtifactSharingPolicy_IsValid: the declared sharing policies are
// recognised; unknown strings are rejected so Create can refuse bad input.
func TestArtifactSharingPolicy_IsValid(t *testing.T) {
	for _, p := range []ArtifactSharingPolicy{
		ArtifactSharingPrivate, ArtifactSharingTenant, ArtifactSharingExplicit,
	} {
		if !p.IsValid() {
			t.Errorf("%q should be a valid sharing policy", p)
		}
	}
	if ArtifactSharingPolicy("public").IsValid() {
		t.Errorf("\"public\" should not be a valid sharing policy")
	}
}
