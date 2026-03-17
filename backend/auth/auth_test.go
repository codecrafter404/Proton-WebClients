package auth

import (
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func TestCreateSession_Success(t *testing.T) {
	m := newTestManager(t)

	sess := m.CreateSession("proton")
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

func TestValidate_ValidToken(t *testing.T) {
	m := newTestManager(t)

	sess := m.CreateSession("proton")

	validated, err := m.Validate(sess.AccessToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.UID != sess.UID {
		t.Errorf("UID = %q, want %q", validated.UID, sess.UID)
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Validate("bogus-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestValidate_ExpiredToken(t *testing.T) {
	m := newTestManager(t)

	sess := m.CreateSession("proton")

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
	m := newTestManager(t)

	sess := m.CreateSession("proton")
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
	m := newTestManager(t)

	sess := m.CreateSession("proton")

	_, err := m.Refresh(sess.UID, "wrong-refresh-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestRefresh_InvalidUID(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Refresh("nonexistent-uid", "some-token")
	if err == nil {
		t.Error("expected error for nonexistent UID")
	}
}

func TestLogout(t *testing.T) {
	m := newTestManager(t)

	sess := m.CreateSession("proton")
	m.Logout(sess.UID)

	_, err := m.Validate(sess.AccessToken)
	if err == nil {
		t.Error("expected error after logout")
	}
}

func TestMultipleSessions(t *testing.T) {
	m := newTestManager(t)

	sess1 := m.CreateSession("proton")
	sess2 := m.CreateSession("proton")

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

func TestSRP_DefaultUser(t *testing.T) {
	m := newTestManager(t)

	// Verify the SRP server was initialized with the default user
	if m.SRP == nil {
		t.Fatal("SRP server is nil")
	}

	// Should be able to get auth info for the default "proton" user
	serverEph, salt, version, session, err := m.SRP.GetAuthInfo("proton")
	if err != nil {
		t.Fatalf("GetAuthInfo: %v", err)
	}
	if serverEph == "" || salt == "" || session == "" {
		t.Fatal("empty auth info fields")
	}
	if version != 4 {
		t.Fatalf("expected version 4, got %d", version)
	}
}
