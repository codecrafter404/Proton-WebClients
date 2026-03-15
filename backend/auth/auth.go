package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Session represents an authenticated user session.
type Session struct {
	UID          string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserID       string
	Username     string
}

// Manager handles authentication, sessions and token validation.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session // keyed by UID
	tokens   map[string]string   // access token -> UID
	users    map[string]string   // username -> password (demo users)
}

// NewManager creates a new auth manager with a default demo user.
func NewManager() *Manager {
	m := &Manager{
		sessions: make(map[string]*Session),
		tokens:   make(map[string]string),
		users:    make(map[string]string),
	}
	// Default demo user
	m.users["proton"] = "proton"
	return m
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Authenticate validates credentials and creates a session.
func (m *Manager) Authenticate(username, password string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expected, ok := m.users[username]
	if !ok || expected != password {
		return nil, fmt.Errorf("invalid credentials")
	}

	uid := randomHex(16)
	accessToken := randomHex(32)
	refreshToken := randomHex(32)

	sess := &Session{
		UID:          uid,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		UserID:       "user-1",
		Username:     username,
	}

	m.sessions[uid] = sess
	m.tokens[accessToken] = uid

	return sess, nil
}

// Refresh creates a new access token from a refresh token.
func (m *Manager) Refresh(uid, refreshToken string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[uid]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	if sess.RefreshToken != refreshToken {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Revoke old access token
	delete(m.tokens, sess.AccessToken)

	// Issue new tokens
	sess.AccessToken = randomHex(32)
	sess.RefreshToken = randomHex(32)
	sess.ExpiresAt = time.Now().Add(24 * time.Hour)

	m.tokens[sess.AccessToken] = uid

	return sess, nil
}

// Validate checks an access token and returns the session UID.
func (m *Manager) Validate(accessToken string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uid, ok := m.tokens[accessToken]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	sess := m.sessions[uid]
	if sess == nil || time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}
	return sess, nil
}

// Logout removes a session.
func (m *Manager) Logout(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[uid]; ok {
		delete(m.tokens, sess.AccessToken)
		delete(m.sessions, uid)
	}
}

// --- HTTP Handlers ---

// LoginRequest is the body for POST /core/v4/auth.
type LoginRequest struct {
	Username string `json:"Username"`
	Password string `json:"Password"`
}

// HandleLogin handles POST /core/v4/auth
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "invalid request body",
		})
		return
	}

	sess, err := m.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"Code": 401, "Error": "invalid credentials",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":         1000,
		"UID":          sess.UID,
		"AccessToken":  sess.AccessToken,
		"RefreshToken": sess.RefreshToken,
		"ExpiresIn":    86400,
		"TokenType":    "Bearer",
		"Scope":        "full",
		"UserID":       sess.UserID,
	})
}

// RefreshRequest is the body for POST /auth/refresh.
type RefreshRequest struct {
	UID          string `json:"UID"`
	RefreshToken string `json:"RefreshToken"`
}

// HandleRefresh handles POST /auth/refresh
func (m *Manager) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "invalid request body",
		})
		return
	}

	sess, err := m.Refresh(req.UID, req.RefreshToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"Code": 401, "Error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":         1000,
		"UID":          sess.UID,
		"AccessToken":  sess.AccessToken,
		"RefreshToken": sess.RefreshToken,
		"ExpiresIn":    86400,
		"TokenType":    "Bearer",
		"Scope":        "full",
	})
}

// HandleLogout handles DELETE /core/v4/auth
func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	uid := r.Header.Get("x-pm-uid")
	m.Logout(uid)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// Middleware returns an HTTP middleware that validates the Bearer token.
// It skips authentication for the auth endpoints themselves.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for login, refresh, and OPTIONS (CORS preflight)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if path == "/core/v4/auth" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if path == "/auth/refresh" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"Code": 401, "Error": "missing authorization header",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"Code": 401, "Error": "invalid authorization format",
			})
			return
		}

		_, err := m.Validate(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"Code": 401, "Error": err.Error(),
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
