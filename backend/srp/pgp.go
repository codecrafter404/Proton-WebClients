package srp

import "encoding/base64"

const StaticPublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

xjMEabb6jRYJKwYBBAHaRw8BAQdAT5zQGqIA7Z8XMGMeQl98MevKbM9+LCoSb6WX
ec/78s7NEnByb3RvbkBzcnAubW9kdWx1c8K/BBMWCABxBYJptvqNAwsJBwkQr87q
ja+F6Tg1FAAAAAAAHAAQc2FsdEBub3RhdGlvbnMub3BlbnBncGpzLm9yZ7WkWKUd
LSzkALC+wcNXDl8CFQgDFgACAhkBApsDAh4BFiEE+G32kbeO0oJtJ+uIr87qja+F
6TgAAPxAAQD3OLoBDIdAeA/zzdIDRfntha8RGykLoxfobKXVbmv0tQEA1aLMsgm+
PUcwlAaIsCu7bO5jfo6wlqzAIOuDQ7pp0ArOOARptvqNEgorBgEEAZdVAQUBAQdA
0JuGq3sX7D7JcKHCioOoORfKupQ1DBK1kuXWKeCh+T4DAQoJwq4EGBYIAGAFgmm2
+o0JEK/O6o2vhek4NRQAAAAAABwAEHNhbHRAbm90YXRpb25zLm9wZW5wZ3Bqcy5v
cmeJr/3kJem/5sRpaDjIjWRNApsMFiEE+G32kbeO0oJtJ+uIr87qja+F6TgAALTA
AQDR/kkgl0nADw0GgzSKNkSqmDrNi9iZEmwIvsISV1njzAEApANcN+kvNmM3MBjX
P8xjfeFp2Fx/lP974LGC4aLGtQo=
=hS2s
-----END PGP PUBLIC KEY BLOCK-----`

// preSignedModulus is the PGP cleartext-signed modulus, pre-computed using
// openpgp.js to guarantee format compatibility with the Proton frontend's
// pmcrypto/CryptoProxy.verifyCleartextMessage().
//
// gopenpgp v3's SignCleartext() produces signatures with Hash: SHA256 and
// extra notation subpackets that can cause verification failures in the
// frontend's openpgp.js-based stack. By pre-signing with openpgp.js (which
// uses Hash: SHA512 and a minimal v4 signature packet), we ensure the
// signed message is byte-for-byte compatible.
//
// The modulus value is the RFC 3526 2048-bit MODP prime in little-endian
// base64, which never changes, so the signature only needs to be computed
// once.
const preSignedModulus = `-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

//////////9oqqyKWo5yFRAF+pgYJtIV5WqV6nxJlTkYF1iV9ssr3slSTG/wXcW1j6IH7KKDJ5sDhg4YLHee4zvONi5GXpAyfCEYyghsdPEEmLxKTjUMZ22WlnAHKdWeu1KFIFbzYhyWraPcI11lg1/PJP2oPxZpmtNVHDZI2pgFv2OhuHwAwj1b5OxRZihJ5h9LfBEkn66ln4la+2s47u23BvS2XP8La+03pulCTPTGfl5idrWF5EXCUW1tNeFPNxRf8m0KKzAbQzrNsxmV790ENI55CEpRIpsTO6a+CwJ0zGeKCE4CKdEc3ICLYsbENMJoIaLaD8n//////////w==
-----BEGIN PGP SIGNATURE-----

wnUEARYKACcFgmm4bPEJkK/O6o2vhek4FiEE+G32kbeO0oJtJ+uIr87qja+F
6TgAAKA1AQCf3dtb+HgAAU2vcDATeU7c+NXWnhOkvQwmZuYnQWnqvwD/acdq
DL3CUCB9Dv7lzOoBT2YVDqbDPjnh3obUTV3pDQ4=
=QpTX
-----END PGP SIGNATURE-----`

// signModulusWithStaticKey sets the pre-signed modulus and validates that
// the embedded base64 payload matches the server's computed modulus bytes.
func (s *Server) signModulusWithStaticKey() error {
	s.PublicKeyArmored = StaticPublicKey
	s.SignedModulus = preSignedModulus

	// Sanity check: verify the pre-signed modulus matches the computed value.
	expected := base64.StdEncoding.EncodeToString(s.modBytes)
	if extractCleartextBody(preSignedModulus) != expected {
		return errModulusMismatch
	}
	return nil
}

var errModulusMismatch = errorString("pre-signed modulus does not match computed modulus bytes")

type errorString string

func (e errorString) Error() string { return string(e) }

// extractCleartextBody returns the text between the blank line after the
// Hash header and the "-----BEGIN PGP SIGNATURE-----" delimiter.
func extractCleartextBody(signed string) string {
	const sep = "\n\n"
	const sig = "\n-----BEGIN PGP SIGNATURE-----"
	i := indexOf(signed, sep)
	if i < 0 {
		return ""
	}
	body := signed[i+len(sep):]
	j := indexOf(body, sig)
	if j < 0 {
		return ""
	}
	return body[:j]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
