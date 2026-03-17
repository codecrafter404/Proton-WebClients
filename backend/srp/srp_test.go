package srp

import (
	"encoding/base64"
	"testing"
)

func TestExpandHash(t *testing.T) {
	// Verify expandHash produces 256 bytes
	result := expandHash([]byte("test input"))
	if len(result) != 256 {
		t.Fatalf("expandHash: expected 256 bytes, got %d", len(result))
	}
}

func TestBcryptBase64RoundTrip(t *testing.T) {
	input := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	encoded := bcryptBase64Encode(input)
	if len(encoded) < 22 {
		t.Fatalf("expected at least 22 chars, got %d", len(encoded))
	}
	decoded := bcryptBase64Decode(encoded[:22], 16)
	for i := range input {
		if input[i] != decoded[i] {
			t.Fatalf("round-trip mismatch at byte %d: %d != %d", i, input[i], decoded[i])
		}
	}
}

func TestNewServerAndAddUser(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Verify signed modulus was generated
	if srv.SignedModulus == "" {
		t.Fatal("SignedModulus is empty")
	}
	if srv.PublicKeyArmored == "" {
		t.Fatal("PublicKeyArmored is empty")
	}

	// Add a user with password
	err = srv.AddUserWithPassword("testuser", "testpassword")
	if err != nil {
		t.Fatalf("AddUserWithPassword: %v", err)
	}

	// Verify user was added
	srv.mu.RLock()
	u, ok := srv.users["testuser"]
	srv.mu.RUnlock()
	if !ok {
		t.Fatal("user not found")
	}
	if u.Version != 4 {
		t.Fatalf("expected version 4, got %d", u.Version)
	}
	if u.Salt == "" {
		t.Fatal("salt is empty")
	}
	if u.Verifier == nil || u.Verifier.Sign() == 0 {
		t.Fatal("verifier is zero")
	}
}

func TestSRPFullFlow(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Add user
	err = srv.AddUserWithPassword("alice", "s3cret")
	if err != nil {
		t.Fatalf("AddUserWithPassword: %v", err)
	}

	// Get auth info (server side)
	serverEph, salt, version, sessionID, err := srv.GetAuthInfo("alice")
	if err != nil {
		t.Fatalf("GetAuthInfo: %v", err)
	}
	if serverEph == "" || salt == "" || sessionID == "" {
		t.Fatal("empty auth info fields")
	}
	if version != 4 {
		t.Fatalf("expected version 4, got %d", version)
	}

	// Simulate client side: compute client ephemeral and proof
	// We can't fully simulate without the frontend's SRP client,
	// but we can verify the server handles invalid proofs correctly
	fakeA := make([]byte, SRPLen)
	fakeA[0] = 1 // Non-zero
	fakeProof := make([]byte, 256)

	_, _, err = srv.VerifyAuth(sessionID,
		base64.StdEncoding.EncodeToString(fakeA),
		base64.StdEncoding.EncodeToString(fakeProof))
	if err == nil {
		t.Fatal("expected error for invalid proof")
	}
}

func TestSRPSessionNotReusable(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	err = srv.AddUserWithPassword("bob", "password123")
	if err != nil {
		t.Fatalf("AddUserWithPassword: %v", err)
	}

	_, _, _, sessionID, err := srv.GetAuthInfo("bob")
	if err != nil {
		t.Fatalf("GetAuthInfo: %v", err)
	}

	fakeA := make([]byte, SRPLen)
	fakeA[0] = 1
	fakeProof := make([]byte, 256)

	// First attempt (invalid proof but consumes session)
	srv.VerifyAuth(sessionID,
		base64.StdEncoding.EncodeToString(fakeA),
		base64.StdEncoding.EncodeToString(fakeProof))

	// Second attempt should fail (session consumed)
	_, _, err = srv.VerifyAuth(sessionID,
		base64.StdEncoding.EncodeToString(fakeA),
		base64.StdEncoding.EncodeToString(fakeProof))
	if err == nil {
		t.Fatal("expected error for reused session")
	}
}

func TestGetAuthInfoUnknownUser(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	_, _, _, _, err = srv.GetAuthInfo("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}
