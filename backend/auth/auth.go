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

	"github.com/codecrafter404/Proton-WebClients/backend/srp"
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

	SRP *srp.Server
}

// NewManager creates a new auth manager with SRP support.
func NewManager() (*Manager, error) {
	srpServer, err := srp.NewServer()
	if err != nil {
		return nil, fmt.Errorf("failed to create SRP server: %w", err)
	}

	// Register default demo user
	if err := srpServer.AddUserWithPassword("proton", "proton"); err != nil {
		return nil, fmt.Errorf("failed to add default user: %w", err)
	}

	m := &Manager{
		sessions: make(map[string]*Session),
		tokens:   make(map[string]string),
		SRP:      srpServer,
	}
	return m, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateSession creates a new session for a user.
func (m *Manager) CreateSession(username string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	return sess
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

// Validate checks an access token and returns the session.
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

// HandleAuthInfo handles POST /core/v4/auth/info
// Returns SRP parameters for the given username.
func (m *Manager) HandleAuthInfo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"Username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "invalid request body",
		})
		return
	}

	serverEph, salt, version, srpSession, err := m.SRP.GetAuthInfo(req.Username)
	if err != nil {
		// Return fake parameters to not reveal if user exists
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"Code":            1000,
			"Modulus":         m.SRP.SignedModulus,
			"ServerEphemeral": "",
			"Version":         0,
			"Salt":            "",
			"SRPSession":      "",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":            1000,
		"Modulus":         m.SRP.SignedModulus,
		"ServerEphemeral": serverEph,
		"Version":         version,
		"Salt":            salt,
		"SRPSession":      srpSession,
	})
}

// HandleAuthModulus handles GET /core/v4/auth/modulus
func (m *Manager) HandleAuthModulus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":      1000,
		"Modulus":   m.SRP.SignedModulus,
		"ModulusID": "modulus-id-1",
	})
}

// HandleLogin handles POST /core/v4/auth (SRP authentication)
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"Username"`
		ClientEphemeral string `json:"ClientEphemeral"`
		ClientProof     string `json:"ClientProof"`
		SRPSession      string `json:"SRPSession"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "invalid request body",
		})
		return
	}

	serverProof, username, err := m.SRP.VerifyAuth(req.SRPSession, req.ClientEphemeral, req.ClientProof)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"Code":  8002,
			"Error": "Incorrect login credentials. Please try again.",
		})
		return
	}

	sess := m.CreateSession(username)

	// Set cookies for the new session
	http.SetCookie(w, &http.Cookie{
		Name:     "AUTH-" + sess.UID,
		Value:    sess.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "REFRESH-" + sess.UID,
		Value:    sess.RefreshToken,
		Path:     "/api/auth/refresh",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":         1000,
		"UID":          sess.UID,
		"AccessToken":  sess.AccessToken,
		"RefreshToken": sess.RefreshToken,
		"ExpiresIn":    86400,
		"TokenType":    "Bearer",
		"Scope":        "full self organization payments keys parent user addresses balance client settings member",
		"UserID":       sess.UserID,
		"ServerProof":  serverProof,
		"PasswordMode": 1,
		"2FA": map[string]interface{}{
			"Enabled": 0,
			"TOTP":    0,
			"FIDO2":   map[string]interface{}{"AuthenticationOptions": nil, "RegisteredKeys": nil},
		},
	})
}

// HandleRefresh handles POST /auth/refresh
func (m *Manager) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UID          string `json:"UID"`
		RefreshToken string `json:"RefreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // ignore error - body may be empty

	// Fall back to x-pm-uid header
	if req.UID == "" {
		req.UID = r.Header.Get("x-pm-uid")
	}

	// Fall back to cookie-based refresh token
	if req.RefreshToken == "" && req.UID != "" {
		if c, err := r.Cookie("REFRESH-" + req.UID); err == nil {
			req.RefreshToken = c.Value
		}
	}

	if req.UID == "" || req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "missing UID or RefreshToken",
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

	// Update cookies with new tokens
	http.SetCookie(w, &http.Cookie{
		Name:     "AUTH-" + sess.UID,
		Value:    sess.AccessToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "REFRESH-" + sess.UID,
		Value:    sess.RefreshToken,
		Path:     "/api/auth/refresh",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

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

// HandleUnauthSession handles POST /auth/v4/sessions
// Creates a temporary unauthenticated session used before login.
func (m *Manager) HandleUnauthSession(w http.ResponseWriter, r *http.Request) {
	sess := m.CreateSession("_unauth")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":         1000,
		"UID":          sess.UID,
		"AccessToken":  sess.AccessToken,
		"RefreshToken": sess.RefreshToken,
		"ExpiresIn":    3600,
		"TokenType":    "Bearer",
		"Scope":        "full",
	})
}

// HandleAuthCookies handles POST /core/v4/auth/cookies
// Sets HTTP cookies for session auth (as the Proton frontend expects).
func (m *Manager) HandleAuthCookies(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UID          string `json:"UID"`
		RefreshToken string `json:"RefreshToken"`
		State        string `json:"State"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"Code": 400, "Error": "invalid request body",
		})
		return
	}

	// Look up the session by UID to get the access token
	m.mu.RLock()
	sess, ok := m.sessions[req.UID]
	m.mu.RUnlock()

	if !ok {
		// If we can't find the session, check the Authorization header
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != authHeader {
			m.mu.RLock()
			uid, tokenOk := m.tokens[token]
			if tokenOk {
				sess = m.sessions[uid]
				ok = sess != nil
			}
			m.mu.RUnlock()
		}
	}

	if ok && sess != nil {
		// Set access token cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "AUTH-" + sess.UID,
			Value:    sess.AccessToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		// Set refresh token cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "REFRESH-" + sess.UID,
			Value:    sess.RefreshToken,
			Path:     "/api/auth/refresh",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// Middleware returns an HTTP middleware that validates the Bearer token.
// It skips authentication for public endpoints.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for OPTIONS (CORS preflight)
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Public endpoints that don't require authentication
		publicPaths := map[string]string{
			"/core/v4/auth":            http.MethodPost,
			"/core/v4/auth/info":       http.MethodPost,
			"/core/v4/auth/modulus":    http.MethodGet,
			"/auth/refresh":           http.MethodPost,
			"/auth/v4/sessions":       http.MethodPost,
			"/core/v4/auth/cookies":   http.MethodPost,
		}
		if method, ok := publicPaths[path]; ok && r.Method == method {
			next.ServeHTTP(w, r)
			return
		}

		// Public prefixes that don't require authentication
		publicPrefixes := []string{
			"/feature/",    // Unleash feature flags
			"/challenge/",  // Captcha challenge
		}
		for _, prefix := range publicPrefixes {
			if strings.HasPrefix(path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		token := ""
		if authHeader != "" {
			token = strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"Code": 401, "Error": "invalid authorization format",
				})
				return
			}
		}

		// Fall back to cookie-based auth if no Authorization header
		if token == "" {
			uid := r.Header.Get("x-pm-uid")
			if uid != "" {
				if c, err := r.Cookie("AUTH-" + uid); err == nil {
					token = c.Value
				}
			}
		}

		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"Code": 401, "Error": "missing authorization header",
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
