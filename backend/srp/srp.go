// Package srp implements the server side of the SRP-6a protocol
// compatible with the Proton frontend's @proton/srp package.
//
// Protocol details:
//   - 2048-bit modulus N (PGP cleartext-signed)
//   - Generator g = 2
//   - Multiplier k = expandHash(LE(g, 256) || LE(N, 256))
//   - All big integers are little-endian 256-byte arrays
//   - expandHash(x) = SHA512(x||0) || SHA512(x||1) || SHA512(x||2) || SHA512(x||3)
//   - Client proof  = expandHash(LE(A) || LE(B) || LE(S))
//   - Server proof  = expandHash(LE(A) || clientProof || LE(S))
package srp

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"math/big"
	"sync"

	"golang.org/x/crypto/blowfish"
)

const (
	// SRPLen is the byte length of the 2048-bit modulus (256 bytes).
	SRPLen = 2048 / 8
)

// ── helpers ──────────────────────────────────────────────────────────

// expandHash: 4×SHA-512 with suffix byte 0‥3 → 256 bytes.
func expandHash(input []byte) []byte {
	out := make([]byte, 0, 256)
	for i := byte(0); i < 4; i++ {
		h := sha512.New()
		h.Write(input)
		h.Write([]byte{i})
		out = append(out, h.Sum(nil)...)
	}
	return out
}

// bigIntToLE converts a *big.Int to a little-endian byte slice of exactly n bytes.
func bigIntToLE(v *big.Int, n int) []byte {
	be := v.Bytes() // big-endian
	le := make([]byte, n)
	for i, b := range be {
		if idx := len(be) - 1 - i; idx < n {
			le[idx] = b
		}
	}
	return le
}

// leToBigInt converts a little-endian byte slice to a *big.Int.
func leToBigInt(le []byte) *big.Int {
	be := make([]byte, len(le))
	for i, b := range le {
		be[len(le)-1-i] = b
	}
	return new(big.Int).SetBytes(be)
}

func cat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ── bcrypt with explicit salt ────────────────────────────────────────

// bcryptBase64Encode encodes src using bcrypt's custom base64 alphabet.
func bcryptBase64Encode(src []byte) string {
	const alpha = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	sz := (len(src)*4 + 2) / 3
	dst := make([]byte, 0, sz)
	for i := 0; i < len(src); i += 3 {
		rem := len(src) - i
		b0 := src[i]
		var b1, b2 byte
		if rem > 1 {
			b1 = src[i+1]
		}
		if rem > 2 {
			b2 = src[i+2]
		}
		dst = append(dst, alpha[b0>>2])
		dst = append(dst, alpha[((b0&0x03)<<4)|(b1>>4)])
		if rem > 1 {
			dst = append(dst, alpha[((b1&0x0f)<<2)|(b2>>6)])
		}
		if rem > 2 {
			dst = append(dst, alpha[b2&0x3f])
		}
	}
	return string(dst)
}

// bcryptBase64Decode decodes a bcrypt-base64 encoded string.
func bcryptBase64Decode(enc string, maxLen int) []byte {
	const alpha = "./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	lut := [256]byte{}
	for i := range alpha {
		lut[alpha[i]] = byte(i)
	}
	out := make([]byte, 0, maxLen)
	for i := 0; i < len(enc); i += 4 {
		rem := len(enc) - i
		var b [4]byte
		n := 0
		for j := 0; j < 4 && j < rem; j++ {
			b[j] = lut[enc[i+j]]
			n++
		}
		out = append(out, (b[0]<<2)|(b[1]>>4))
		if n > 2 {
			out = append(out, (b[1]<<4)|(b[2]>>2))
		}
		if n > 3 {
			out = append(out, (b[2]<<6)|b[3])
		}
	}
	if len(out) > maxLen {
		out = out[:maxLen]
	}
	return out
}

// bcryptWithSalt computes bcrypt with a specific 16-byte salt and cost 10.
// Returns the full "$2y$10$<salt22><hash31>" string.
func bcryptWithSalt(password, salt []byte) (string, error) {
	if len(salt) != 16 {
		return "", fmt.Errorf("bcrypt salt must be 16 bytes, got %d", len(salt))
	}
	key := append([]byte(nil), password...)
	key = append(key, 0) // bcrypt null-terminates
	c, err := blowfish.NewSaltedCipher(key, salt)
	if err != nil {
		return "", err
	}
	rounds := 1 << 10 // cost = 10
	for i := 0; i < rounds; i++ {
		blowfish.ExpandKey(key, c)
		blowfish.ExpandKey(salt, c)
	}
	ctext := []byte("OrpheanBeholderScryDoubt") // 24 bytes
	for i := 0; i < 64; i++ {
		for j := 0; j < 24; j += 8 {
			c.Encrypt(ctext[j:j+8], ctext[j:j+8])
		}
	}
	saltEnc := bcryptBase64Encode(salt)
	if len(saltEnc) > 22 {
		saltEnc = saltEnc[:22]
	}
	hashEnc := bcryptBase64Encode(ctext[:23])
	return fmt.Sprintf("$2y$10$%s%s", saltEnc, hashEnc), nil
}

// hashPasswordV4 computes the Proton v4 password hash:
//
//	saltBinary = salt || "proton"   (take first 16 bytes)
//	bcryptSalt = bcryptBase64Encode(saltBinary)[:22]
//	h          = bcrypt(password, "$2y$10$" + bcryptSalt)
//	return expandHash(h || modulusLE)
func hashPasswordV4(password string, salt, modulusLE []byte) ([]byte, error) {
	s16 := make([]byte, 16)
	copy(s16, append(append([]byte(nil), salt...), []byte("proton")...))
	raw := bcryptBase64Decode(bcryptBase64Encode(s16)[:22], 16)
	h, err := bcryptWithSalt([]byte(password), raw)
	if err != nil {
		return nil, err
	}
	return expandHash(cat([]byte(h), modulusLE)), nil
}

// ── SRP server ───────────────────────────────────────────────────────

// UserVerifier stores a user's SRP verifier.
type UserVerifier struct {
	Version  int
	Salt     string   // base64
	Verifier *big.Int // v = g^x mod N
}

// srpSession stores per-attempt ephemeral state.
type srpSession struct {
	b        *big.Int
	B        *big.Int
	Username string
}

// Server holds the SRP server state.
type Server struct {
	mu       sync.RWMutex
	N, G, K  *big.Int
	modBytes []byte // LE

	SignedModulus     string // PGP cleartext signed
	PublicKeyArmored string

	users    map[string]*UserVerifier
	sessions map[string]*srpSession
}

// NewServer creates and initialises the SRP server.
func NewServer() (*Server, error) {
	srv := &Server{
		G:        big.NewInt(2),
		users:    make(map[string]*UserVerifier),
		sessions: make(map[string]*srpSession),
	}

	// RFC 3526 2048-bit MODP prime
	hex := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AACAA68FFFFFFFFFFFFFFFF"
	srv.N = new(big.Int)
	srv.N.SetString(hex, 16)
	srv.modBytes = bigIntToLE(srv.N, SRPLen)

	gLE := bigIntToLE(srv.G, SRPLen)
	srv.K = leToBigInt(expandHash(cat(gLE, srv.modBytes)))
	srv.K.Mod(srv.K, srv.N)

	if err := srv.signModulusWithStaticKey(); err != nil {
		return nil, err
	}
	return srv, nil
}

// AddUserWithPassword hashes password and stores the verifier.
func (srv *Server) AddUserWithPassword(username, password string) error {
	salt := make([]byte, 10)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hp, err := hashPasswordV4(password, salt, srv.modBytes)
	if err != nil {
		return err
	}
	x := leToBigInt(hp)
	v := new(big.Int).Exp(srv.G, x, srv.N)

	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.users[username] = &UserVerifier{
		Version:  4,
		Salt:     base64.StdEncoding.EncodeToString(salt),
		Verifier: v,
	}
	return nil
}

// CheckPassword verifies a plain-text password against the stored verifier.
// This is used by the static calendar page which doesn't implement SRP in the browser.
func (srv *Server) CheckPassword(username, password string) bool {
	srv.mu.RLock()
	uv, ok := srv.users[username]
	srv.mu.RUnlock()
	if !ok {
		return false
	}
	saltBytes, err := base64.StdEncoding.DecodeString(uv.Salt)
	if err != nil {
		return false
	}
	hp, err := hashPasswordV4(password, saltBytes, srv.modBytes)
	if err != nil {
		return false
	}
	x := leToBigInt(hp)
	v := new(big.Int).Exp(srv.G, x, srv.N)
	return v.Cmp(uv.Verifier) == 0
}

// AddUserWithVerifier stores a pre-computed verifier.
func (srv *Server) AddUserWithVerifier(username string, version int, saltB64, verifierB64 string) error {
	vBytes, err := base64.StdEncoding.DecodeString(verifierB64)
	if err != nil {
		return err
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.users[username] = &UserVerifier{
		Version:  version,
		Salt:     saltB64,
		Verifier: leToBigInt(vBytes),
	}
	return nil
}

// GetAuthInfo generates ephemeral B and an SRP session ID.
func (srv *Server) GetAuthInfo(username string) (serverEph, salt string, version int, session string, err error) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	u, ok := srv.users[username]
	if !ok {
		return "", "", 0, "", fmt.Errorf("user not found")
	}

	bBytes := make([]byte, SRPLen)
	rand.Read(bBytes)
	b := leToBigInt(bBytes)

	// B = k*v + g^b  (mod N)
	kv := new(big.Int).Mul(srv.K, u.Verifier)
	kv.Mod(kv, srv.N)
	gb := new(big.Int).Exp(srv.G, b, srv.N)
	B := new(big.Int).Add(kv, gb)
	B.Mod(B, srv.N)

	sid := make([]byte, 16)
	rand.Read(sid)
	sidStr := fmt.Sprintf("%x", sid)

	srv.sessions[sidStr] = &srpSession{b: b, B: B, Username: username}

	return base64.StdEncoding.EncodeToString(bigIntToLE(B, SRPLen)),
		u.Salt, u.Version, sidStr, nil
}

// VerifyAuth checks the client proof and returns the server proof.
func (srv *Server) VerifyAuth(sessionID, clientEphB64, clientProofB64 string) (serverProof, username string, err error) {
	srv.mu.Lock()
	sess, ok := srv.sessions[sessionID]
	if !ok {
		srv.mu.Unlock()
		return "", "", fmt.Errorf("bad SRP session")
	}
	uv := srv.users[sess.Username]
	delete(srv.sessions, sessionID)
	srv.mu.Unlock()

	aBytes, err := base64.StdEncoding.DecodeString(clientEphB64)
	if err != nil {
		return "", "", err
	}
	A := leToBigInt(aBytes)
	if new(big.Int).Mod(A, srv.N).Sign() == 0 {
		return "", "", fmt.Errorf("A == 0")
	}
	cpBytes, err := base64.StdEncoding.DecodeString(clientProofB64)
	if err != nil {
		return "", "", err
	}

	aLE := bigIntToLE(A, SRPLen)
	bLE := bigIntToLE(sess.B, SRPLen)

	u := leToBigInt(expandHash(cat(aLE, bLE)))
	if u.Sign() == 0 {
		return "", "", fmt.Errorf("u == 0")
	}

	// S = (A · v^u)^b  mod N
	vu := new(big.Int).Exp(uv.Verifier, u, srv.N)
	avu := new(big.Int).Mul(A, vu)
	avu.Mod(avu, srv.N)
	S := new(big.Int).Exp(avu, sess.b, srv.N)
	sLE := bigIntToLE(S, SRPLen)

	expectCP := expandHash(cat(aLE, bLE, sLE))
	if !constEq(cpBytes, expectCP) {
		return "", "", fmt.Errorf("invalid client proof")
	}

	sp := expandHash(cat(aLE, cpBytes, sLE))
	return base64.StdEncoding.EncodeToString(sp), sess.Username, nil
}

func constEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
