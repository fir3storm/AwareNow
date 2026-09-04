package auth

import "testing"

func TestGenerateJWTReturnsClaimsForAuthenticatedUser(t *testing.T) {
	token, err := GenerateJWT(42, "alex", "admin")
	if err != nil {
		t.Fatalf("GenerateJWT returned an error: %v", err)
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT returned an error: %v", err)
	}

	if claims.UserID != 42 || claims.Username != "alex" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}
