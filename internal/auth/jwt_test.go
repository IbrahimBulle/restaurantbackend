package auth

import (
	"testing"

	"restaurant/backend/internal/domain"
)

func TestIssueAndParseToken(t *testing.T) {
	user := domain.User{ID: 7, Email: "chef@example.com", Role: domain.RoleChef}
	token, err := IssueToken("secret", user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := ParseToken("secret", token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != user.ID || claims.Role != user.Role {
		t.Fatalf("claims mismatch: %#v", claims)
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("Password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPassword(hash, "Password123") {
		t.Fatal("expected password to match hash")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("unexpected password match")
	}
}

func TestSeededAdminPasswordHash(t *testing.T) {
	hash := "$2a$10$i5FMyRLM8BqOtXCC9QajmeNyiKSOYtp07W4jAtX3pPZnt6M7z5tMy"
	if !CheckPassword(hash, "Password123") {
		t.Fatal("seeded admin hash must match README password")
	}
}
