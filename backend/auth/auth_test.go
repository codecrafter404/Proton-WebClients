package auth

import (
	"testing"
	"time"
)

func TestAuthenticate_Success(t *testing.T) {
	m := NewManager()

	sess, err := m.Authenticate("proton", "proton")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if sess.UID == "" {
		t.Error("expected non-empty UID")
	}
	if sess.AccessToken == "" {
		t.Error("expected non-empty AccessToken")
	}
	if sess.RefreshToken == "" {
		t.Error("expected non-empty RefreshToken")
	}
	if sess.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", sess.UserID, "user-1")
	}
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	m := NewManager()

	_, err := m.Authenticate("wrong", "wrong")
	if err == nil {
		t.Error("expected error for invalid credentials")
	}

	_, err = m.Authenticate("proton", "wrong")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestValidate_ValidToken(t *testing.T) {
	m := NewManager()

	sess, _ := m.Authenticate("proton", "proton")

	validated, err := m.Validate(sess.AccessToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.UID != sess.UID {
		t.Errorf("UID = %q, want %q", validated.UID, sess.UID)
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	m := NewManager()

	_, err := m.Validate("bogus-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	m := NewManager()

	sess, _ := m.Authenticate("proton", "proton")

	// Manually expire the session
	m.mu.Lock()
	m.sessions[sess.UID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	m.mu.Unlock()

	_, err := m.Validate(sess.AccessToken)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestRefresh_Success(t *testing.T) {
	m := NewManager()

	sess, _ := m.Authenticate("proton", "proton")
	oldAccess := sess.AccessToken
	oldRefresh := sess.RefreshToken

	refreshed, err := m.Refresh(sess.UID, sess.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken == oldAccess {
		t.Error("expected new access token")
	}
	if refreshed.RefreshToken == oldRefresh {
		t.Error("expected new refresh token")
	}

	// Old access token should no longer work
	_, err = m.Validate(oldAccess)
	if err == nil {
		t.Error("expected old access token to be invalid")
	}

	// New access token should work
	_, err = m.Validate(refreshed.AccessToken)
	if err != nil {
		t.Fatalf("new token validation: %v", err)
	}
}

func TestRefresh_InvalidRefreshToken(t *testing.T) {
	m := NewManager()

	sess, _ := m.Authenticate("proton", "proton")

	_, err := m.Refresh(sess.UID, "wrong-refresh-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestRefresh_InvalidUID(t *testing.T) {
	m := NewManager()

	_, err := m.Refresh("nonexistent-uid", "some-token")
	if err == nil {
		t.Error("expected error for nonexistent UID")
	}
}

func TestLogout(t *testing.T) {
	m := NewManager()

	sess, _ := m.Authenticate("proton", "proton")
	m.Logout(sess.UID)

	_, err := m.Validate(sess.AccessToken)
	if err == nil {
		t.Error("expected error after logout")
	}
}

func TestMultipleSessions(t *testing.T) {
	m := NewManager()

	sess1, _ := m.Authenticate("proton", "proton")
	sess2, _ := m.Authenticate("proton", "proton")

	// Both should be valid
	_, err := m.Validate(sess1.AccessToken)
	if err != nil {
		t.Fatalf("sess1 validate: %v", err)
	}
	_, err = m.Validate(sess2.AccessToken)
	if err != nil {
		t.Fatalf("sess2 validate: %v", err)
	}

	// Logout one should not affect the other
	m.Logout(sess1.UID)
	_, err = m.Validate(sess2.AccessToken)
	if err != nil {
		t.Error("sess2 should still be valid after sess1 logout")
	}
}
