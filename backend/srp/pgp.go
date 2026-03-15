package srp

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

// Static PGP key pair for signing the SRP modulus.
// This key is embedded so the frontend constant (SRP_MODULUS_KEY) can be set once.
const staticPrivateKey = `-----BEGIN PGP PRIVATE KEY BLOCK-----

xVgEabb6jRYJKwYBBAHaRw8BAQdAT5zQGqIA7Z8XMGMeQl98MevKbM9+LCoSb6WX
ec/78s4AAPwLgKIeDN3NLqMimrzd/C+p8QMsk178mgK0ntzYuMxoxBJWzRJwcm90
b25Ac3JwLm1vZHVsdXPCvwQTFggAcQWCabb6jQMLCQcJEK/O6o2vhek4NRQAAAAA
ABwAEHNhbHRAbm90YXRpb25zLm9wZW5wZ3Bqcy5vcme1pFilHS0s5ACwvsHDVw5f
AhUIAxYAAgIZAQKbAwIeARYhBPht9pG3jtKCbSfriK/O6o2vhek4AAD8QAEA9zi6
AQyHQHgP883SA0X57YWvERspC6MX6Gyl1W5r9LUBANWizLIJvj1HMJQGiLAru2zu
Y36OsJaswCDrg0O6adAKx10Eabb6jRIKKwYBBAGXVQEFAQEHQNCbhqt7F+w+yXCh
woqDqDkXyrqUNQwStZLl1ingofk+AwEKCQAA/2RjCWJGSnlJY883HxvboNXo965F
LUMLl2X8iNRjsP24EN/CrgQYFggAYAWCabb6jQkQr87qja+F6Tg1FAAAAAAAHAAQ
c2FsdEBub3RhdGlvbnMub3BlbnBncGpzLm9yZ4mv/eQl6b/mxGloOMiNZE0CmwwW
IQT4bfaRt47Sgm0n64ivzuqNr4XpOAAAtMABANH+SSCXScAPDQaDNIo2RKqYOs2L
2JkSbAi+whJXWePMAQCkA1w36S82YzcwGNc/zGN94WnYXH+U/3vgsYLhosa1Cg==
=xSZc
-----END PGP PRIVATE KEY BLOCK-----`

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

// signModulusWithStaticKey signs the modulus using the embedded static PGP key.
func (s *Server) signModulusWithStaticKey() error {
	pgp := crypto.PGP()

	key, err := crypto.NewKeyFromArmored(staticPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to load static PGP key: %w", err)
	}

	s.PublicKeyArmored = StaticPublicKey

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
