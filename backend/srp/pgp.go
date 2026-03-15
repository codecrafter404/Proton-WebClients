package srp

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

// generatePGPKeyAndSignModulus generates a PGP key pair and signs the modulus.
func (s *Server) generatePGPKeyAndSignModulus() error {
	// Generate a new PGP key for signing the modulus
	pgp := crypto.PGP()

	keyGenHandle := pgp.KeyGeneration().
		AddUserId("proton@srp.modulus", "").
		New()

	key, err := keyGenHandle.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate PGP key: %w", err)
	}

	// Export the public key (armored)
	pubKey, err := key.GetArmoredPublicKey()
	if err != nil {
		return fmt.Errorf("failed to export public key: %w", err)
	}
	s.PublicKeyArmored = pubKey

	// Get the modulus as base64 string (this is what gets signed)
	modulusBase64 := base64.StdEncoding.EncodeToString(s.modBytes)

	// Create a cleartext signed message
	signingKeyRing, err := crypto.NewKeyRing(key)
	if err != nil {
		return fmt.Errorf("failed to create key ring: %w", err)
	}

	signHandle, err := pgp.Sign().SigningKeys(signingKeyRing).New()
	if err != nil {
		return fmt.Errorf("failed to create sign handle: %w", err)
	}

	signedMessage, err := signHandle.SignCleartext([]byte(modulusBase64))
	if err != nil {
		return fmt.Errorf("failed to sign modulus: %w", err)
	}

	s.SignedModulus = string(signedMessage)

	return nil
}
