package utils

import (
	"strings"
	"testing"
)

// TestHashAPIKey_Deterministic: the same plaintext always yields the same hash.
func TestHashAPIKey_Deterministic(t *testing.T) {
	h1 := HashAPIKey("sk_secret123")
	h2 := HashAPIKey("sk_secret123")
	if h1 != h2 {
		t.Errorf("相同明文应产生相同 hash")
	}
	if h1 == "" {
		t.Errorf("hash 不应为空")
	}
}

// TestHashAPIKey_Distinct: different plaintexts yield different hashes.
func TestHashAPIKKey_Distinct(t *testing.T) {
	if HashAPIKey("sk_a") == HashAPIKey("sk_b") {
		t.Errorf("不同明文不应产生相同 hash")
	}
}

// TestHashAPIKey_NotPlaintext: the hash must not leak the plaintext.
func TestHashAPIKey_NotPlaintext(t *testing.T) {
	h := HashAPIKey("sk_secret123")
	if strings.Contains(h, "sk_secret123") {
		t.Errorf("hash 不应包含明文")
	}
}

// TestVerifyAPIKeyHash_Correct: verifying with the original plaintext passes.
func TestVerifyAPIKeyHash_Correct(t *testing.T) {
	h := HashAPIKey("sk_secret123")
	if !VerifyAPIKeyHash("sk_secret123", h) {
		t.Errorf("正确明文应校验通过")
	}
}

// TestVerifyAPIKeyHash_Wrong: verifying with the wrong plaintext fails.
func TestVerifyAPIKeyHash_Wrong(t *testing.T) {
	h := HashAPIKey("sk_secret123")
	if VerifyAPIKeyHash("sk_wrong", h) {
		t.Errorf("错误明文应校验失败")
	}
}

// TestVerifyAPIKeyHash_EmptyHash: an empty stored hash never verifies
// (fail-closed — a missing hash must not match anything).
func TestVerifyAPIKeyHash_EmptyHash(t *testing.T) {
	if VerifyAPIKeyHash("sk_anything", "") {
		t.Errorf("空 hash 应 fail-closed 拒绝")
	}
}

// TestGenerateAPIKeySecret_Format: the generated secret has the sk_ prefix and
// reasonable entropy; the prefix is a short display fragment of the secret.
func TestGenerateAPIKeySecret_Format(t *testing.T) {
	secret, prefix := GenerateAPIKeySecret()
	if !strings.HasPrefix(secret, "sk_") {
		t.Errorf("secret 应以 'sk_' 开头，实际 %q", secret)
	}
	if len(secret) < 24 {
		t.Errorf("secret 熵不足 (len=%d)", len(secret))
	}
	if prefix == "" || !strings.HasPrefix(secret, prefix) {
		t.Errorf("prefix %q 应是 secret 的前缀片段", prefix)
	}
}

// TestGenerateAPIKeySecret_Unique: two generations produce distinct secrets.
func TestGenerateAPIKeySecret_Unique(t *testing.T) {
	s1, _ := GenerateAPIKeySecret()
	s2, _ := GenerateAPIKeySecret()
	if s1 == s2 {
		t.Errorf("两次生成的 secret 不应相同")
	}
}

// TestGenerateAPIKeyID_Format: the generated key ID has the ak_ prefix and
// enough entropy to be globally unique across a tenant's keys.
func TestGenerateAPIKeyID_Format(t *testing.T) {
	id := GenerateAPIKeyID()
	if !strings.HasPrefix(id, "ak_") {
		t.Errorf("id 应以 'ak_' 开头，实际 %q", id)
	}
	if len(id) < 20 {
		t.Errorf("id 熵不足 (len=%d)", len(id))
	}
}

// TestGenerateAPIKeyID_Unique: two generations produce distinct IDs.
func TestGenerateAPIKeyID_Unique(t *testing.T) {
	if GenerateAPIKeyID() == GenerateAPIKeyID() {
		t.Errorf("两次生成的 id 不应相同")
	}
}

// TestGenerateArtifactID_Format: the generated artifact ID has the art_ prefix
// and enough entropy to be globally unique.
func TestGenerateArtifactID_Format(t *testing.T) {
	id := GenerateArtifactID()
	if !strings.HasPrefix(id, "art_") {
		t.Errorf("artifact id 应以 'art_' 开头，实际 %q", id)
	}
	if len(id) < 20 {
		t.Errorf("artifact id 熵不足 (len=%d)", len(id))
	}
}

// TestGenerateArtifactID_Unique: two generations produce distinct IDs.
func TestGenerateArtifactID_Unique(t *testing.T) {
	if GenerateArtifactID() == GenerateArtifactID() {
		t.Errorf("两次生成的 artifact id 不应相同")
	}
}
