package auth

import "testing"

func TestTokenVersionRevocation(t *testing.T) {
	SetJWTSecret("unit-test-secret-key-123456")
	SetTokenVersion(1)
	t.Cleanup(func() { SetTokenVersion(1) })

	tokV1, err := GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if _, err := ValidateToken(tokV1); err != nil {
		t.Fatalf("expected v1 token to be valid, got %v", err)
	}

	SetTokenVersion(2)
	if _, err := ValidateToken(tokV1); err == nil {
		t.Fatalf("expected v1 token to be rejected after bumping token version to 2")
	}

	tokV2, err := GenerateToken("admin")
	if err != nil {
		t.Fatalf("GenerateToken v2 failed: %v", err)
	}
	if _, err := ValidateToken(tokV2); err != nil {
		t.Fatalf("expected v2 token to be valid, got %v", err)
	}
}
