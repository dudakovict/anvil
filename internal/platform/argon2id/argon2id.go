// Package argon2id provides support for hashing passwords with Argon2id.
package argon2id

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("hash is not in the argon2id format")
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
)

type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultParams = &Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// CreateHash returns the hash in the PHC string format, which embeds the
// params and a random salt: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>.
func CreateHash(password string, p *Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// ComparePasswordAndHash rehashes password with the params embedded in hash
// and compares in constant time.
func ComparePasswordAndHash(password, hash string) (bool, error) {
	p, salt, key, err := decodeHash(hash)
	if err != nil {
		return false, err
	}

	otherKey := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return subtle.ConstantTimeCompare(key, otherKey) == 1, nil
}

func decodeHash(hash string) (*Params, []byte, []byte, error) {
	vals := strings.Split(hash, "$")
	if len(vals) != 6 || vals[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(vals[2], "v=%d", &version); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	p := &Params{}
	if _, err := fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	p.SaltLength = uint32(len(salt)) //nolint:gosec // salt length fits in uint32

	key, err := base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	p.KeyLength = uint32(len(key)) //nolint:gosec // key length fits in uint32

	// Fail closed on truncated hashes: base64-decoding "" succeeds, and an empty
	// key would make ComparePasswordAndHash derive a zero-length key and match ANY
	// password (ConstantTimeCompare of two empty slices is 1).
	if len(salt) == 0 || len(key) == 0 {
		return nil, nil, nil, ErrInvalidHash
	}

	return p, salt, key, nil
}
