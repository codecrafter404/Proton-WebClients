package srp

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"testing"
)

// TestSRPEndToEnd simulates a full SRP exchange using the Go server
// for both client and server roles (matching the Proton frontend's logic).
func TestSRPEndToEnd(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	password := "mypassword123"
	username := "e2euser"

	// Register user (server computes verifier)
	err = srv.AddUserWithPassword(username, password)
	if err != nil {
		t.Fatalf("AddUserWithPassword: %v", err)
	}

	// Client requests auth info
	serverEphB64, saltB64, version, sessionID, err := srv.GetAuthInfo(username)
	if err != nil {
		t.Fatalf("GetAuthInfo: %v", err)
	}
	t.Logf("version=%d, session=%s", version, sessionID)

	// --- Client-side computation (simulating the frontend) ---

	// Decode salt
	saltBytes, _ := base64.StdEncoding.DecodeString(saltB64)

	// Hash password (same as server did during registration)
	hp, err := hashPasswordV4(password, saltBytes, srv.modBytes)
	if err != nil {
		t.Fatalf("hashPasswordV4: %v", err)
	}
	x := leToBigInt(hp)

	// Client generates random a, computes A = g^a mod N
	aBytes := make([]byte, SRPLen)
	rand.Read(aBytes)
	a := leToBigInt(aBytes)
	A := new(big.Int).Exp(srv.G, a, srv.N)
	aLE := bigIntToLE(A, SRPLen)

	// Decode server ephemeral B
	bLEBytes, _ := base64.StdEncoding.DecodeString(serverEphB64)
	B := leToBigInt(bLEBytes)

	// Compute scrambling param u = expandHash(LE(A) || LE(B))
	bLE := bigIntToLE(B, SRPLen)
	u := leToBigInt(expandHash(cat(aLE, bLE)))

	// Compute k
	gLE := bigIntToLE(srv.G, SRPLen)
	k := leToBigInt(expandHash(cat(gLE, srv.modBytes)))
	k.Mod(k, srv.N)

	// Compute shared session key:
	// kgx = k * g^x mod N
	gx := new(big.Int).Exp(srv.G, x, srv.N)
	kgx := new(big.Int).Mul(k, gx)
	kgx.Mod(kgx, srv.N)

	// base = B - kgx (mod N)
	base := new(big.Int).Sub(B, kgx)
	base.Mod(base, srv.N)
	if base.Sign() < 0 {
		base.Add(base, srv.N)
	}

	// exponent = a + u * x  (mod N-1)
	nMinus1 := new(big.Int).Sub(srv.N, big.NewInt(1))
	ux := new(big.Int).Mul(u, x)
	ux.Mod(ux, nMinus1)
	exp := new(big.Int).Add(a, ux)
	exp.Mod(exp, nMinus1)

	// S = base^exponent mod N
	S := new(big.Int).Exp(base, exp, srv.N)
	sLE := bigIntToLE(S, SRPLen)

	// Client proof = expandHash(LE(A) || LE(B) || LE(S))
	clientProof := expandHash(cat(aLE, bLE, sLE))

	// Expected server proof = expandHash(LE(A) || clientProof || LE(S))
	expectedServerProof := expandHash(cat(aLE, clientProof, sLE))

	// --- Verify with server ---
	serverProofB64, returnedUsername, err := srv.VerifyAuth(
		sessionID,
		base64.StdEncoding.EncodeToString(aLE),
		base64.StdEncoding.EncodeToString(clientProof),
	)
	if err != nil {
		t.Fatalf("VerifyAuth: %v", err)
	}
	if returnedUsername != username {
		t.Fatalf("wrong username: got %q, want %q", returnedUsername, username)
	}

	// Verify server proof
	actualServerProof, _ := base64.StdEncoding.DecodeString(serverProofB64)
	if !constEq(actualServerProof, expectedServerProof) {
		t.Fatal("server proof mismatch")
	}

	t.Log("Full SRP exchange succeeded!")
}
