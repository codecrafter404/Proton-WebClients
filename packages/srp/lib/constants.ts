export const SRP_LEN = 2048 / 8; // in bytes

export const AUTH_FALLBACK_VERSION = 2;
export const AUTH_VERSION = 4;

export const MAX_VALUE_ITERATIONS = 1000;

export const SRP_MODULUS_KEY = `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
-----END PGP PUBLIC KEY BLOCK-----`;

// Version 2 of bcrypt with 2**10 rounds.
// https://en.wikipedia.org/wiki/Bcrypt#Description
export const BCRYPT_PREFIX = '$2y$10$';
