package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeAndValidateJWT_ValidToken(t *testing.T) {
	secret := "supersecret"
	userID := uuid.New()
	token, err := MakeJWT(userID, secret, time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	returnedID, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}
	if returnedID != userID {
		t.Errorf("Expected userID %v, got %v", userID, returnedID)
	}
}

func TestValidateJWT_TamperedToken(t *testing.T) {
	secret := "supersecret"
	userID := uuid.New()
	token, err := MakeJWT(userID, secret, time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	// Tamper the token
	tampered := token + "tamper"

	_, err = ValidateJWT(tampered, secret)
	if err == nil {
		t.Fatal("Expected error for tampered token, got nil")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	secret := "supersecret"
	wrongSecret := "wrongsecret"
	userID := uuid.New()
	token, err := MakeJWT(userID, secret, time.Minute)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	_, err = ValidateJWT(token, wrongSecret)
	if err == nil {
		t.Fatal("Expected error for wrong secret, got nil")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	secret := "supersecret"
	userID := uuid.New()
	token, err := MakeJWT(userID, secret, -1*time.Minute) // expired in the past
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}

	_, err = ValidateJWT(token, secret)
	if err == nil {
		t.Fatal("Expected error for expired token, got nil")
	}
}

func TestValidateJWT_EmptyToken(t *testing.T) {
	secret := "supersecret"

	_, err := ValidateJWT("", secret)
	if err == nil {
		t.Fatal("Expected error for empty token, got nil")
	}
}

func TestGetBearerToken_emptyToken(t *testing.T) {
	header := make(http.Header)
	header.Set("Authorization", "")
	_, err := GetBearerToken(header)
	if err == nil {
		t.Fatal(("Expected error for missing Authorization"))
	}
}
