package helper

import (
	"strings"
	"testing"
	"time"
)

func TestGoogleOAuthStateValidation(t *testing.T) {
	state, nonce, err := NewGoogleOAuthState("secret", time.Minute)
	if err != nil {
		t.Fatalf("NewGoogleOAuthState() error = %v", err)
	}

	if err = ValidateGoogleOAuthState(state, nonce, "secret"); err != nil {
		t.Fatalf("ValidateGoogleOAuthState() error = %v", err)
	}
}

func TestGoogleOAuthStateValidationRejectsTamperedState(t *testing.T) {
	state, nonce, err := NewGoogleOAuthState("secret", time.Minute)
	if err != nil {
		t.Fatalf("NewGoogleOAuthState() error = %v", err)
	}

	tampered := strings.Replace(state, ".", "x.", 1)
	if err = ValidateGoogleOAuthState(tampered, nonce, "secret"); err == nil {
		t.Fatal("ValidateGoogleOAuthState() error = nil, want tampered state error")
	}
}

func TestGoogleOAuthStateValidationRejectsWrongNonce(t *testing.T) {
	state, _, err := NewGoogleOAuthState("secret", time.Minute)
	if err != nil {
		t.Fatalf("NewGoogleOAuthState() error = %v", err)
	}

	if err = ValidateGoogleOAuthState(state, "wrong-nonce", "secret"); err == nil {
		t.Fatal("ValidateGoogleOAuthState() error = nil, want nonce mismatch")
	}
}
