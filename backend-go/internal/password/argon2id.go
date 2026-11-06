// Package password is the only place a plaintext credential is turned into a stored one.
//
// ADR-019 requires Argon2id with documented parameters, and its verification list asks for a
// test that the parameters are "recorded and re-verifiable against a stored hash". Both halves
// of that are structural here: the parameters live inside the encoded hash, so a row never
// depends on a config value that changed after it was written, and Verify re-derives using the
// parameters it read rather than the current ones.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// The parameters a new hash is created with. Changing any of them is a decision that belongs in
// a new ADR, not in a constant edit: existing hashes stay verifiable, but they get weaker than
// the ones written after the change, so the migration is rehash-on-next-successful-login.
const (
	Algorithm  = "argon2id"
	phcVersion = 19 // Argon2 v1.3, the only version in the PHC registry.

	Time      = 3
	MemoryKiB = 64 * 1024 // 64 MiB
	Threads   = 4
	KeyLen    = 32

	saltLen = 16
)

// Bounds a stored hash is allowed to ask for. Verify spends memory and CPU named by the string
// it is handed, and a hash that says m=4194304 (4 GiB) is one INSERT away from taking the
// process down. Legitimate values are far inside these limits.
const (
	maxTime      = 64
	maxMemoryKiB = 1 << 20 // 1 GiB
	maxThreads   = 64
	maxKeyLen    = 128
)

var (
	// ErrMalformed means the stored value is not a PHC-encoded Argon2 hash at all.
	ErrMalformed = errors.New("stored password hash is malformed")
	// ErrUnsupported means it is well-formed but uses an algorithm or parameters this build
	// will not execute. It is distinct from ErrMismatch so a login failure never tells an
	// attacker more than "no".
	ErrUnsupported = errors.New("stored password hash uses unsupported parameters")
	// ErrMismatch means the password is wrong.
	ErrMismatch = errors.New("password does not match")
)

// Params is the decoded content of a stored hash.
type Params struct {
	Algorithm string
	Version   int
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
	KeyLen    uint32
	Salt      []byte
	Sum       []byte
}

// Encode renders Params back into the PHC string stored in identity.users.password_hash:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<salt>$<sum>
//
// base64 without padding, per the PHC format.
func (p Params) Encode() string {
	return "$" + strings.Join([]string{
		Algorithm,
		"v=" + strconv.Itoa(p.Version),
		fmt.Sprintf("m=%d,t=%d,p=%d", p.MemoryKiB, p.Time, p.Threads),
		base64.RawStdEncoding.EncodeToString(p.Salt),
		base64.RawStdEncoding.EncodeToString(p.Sum),
	}, "$")
}

// Hash derives an Argon2id sum from plaintext with a fresh 16-byte salt and returns the PHC
// string. The plaintext is only ever read here; nothing in this package returns it.
func Hash(plaintext string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating password salt: %w", err)
	}
	sum := argon2.IDKey([]byte(plaintext), salt, Time, MemoryKiB, Threads, KeyLen)
	return Params{
		Algorithm: Algorithm,
		Version:   phcVersion,
		Time:      Time,
		MemoryKiB: MemoryKiB,
		Threads:   Threads,
		KeyLen:    KeyLen,
		Salt:      salt,
		Sum:       sum,
	}.Encode(), nil
}

// Verify checks plaintext against a stored PHC hash, using the parameters recorded in that hash.
// It returns ErrMismatch for a wrong password and a different error for a stored value that is
// itself unusable, so a corrupted hash cannot be mistaken for a failed login attempt.
func Verify(stored, plaintext string) error {
	p, err := Parse(stored)
	if err != nil {
		return err
	}
	sum := argon2.IDKey([]byte(plaintext), p.Salt, p.Time, p.MemoryKiB, p.Threads, p.KeyLen)
	if subtle.ConstantTimeCompare(sum, p.Sum) != 1 {
		return ErrMismatch
	}
	return nil
}

// Parse decodes a stored hash without deriving from it, which is how a test (or a future
// rehash-on-login path) reads the parameters a row was written with.
func Parse(stored string) (Params, error) {
	fields := strings.Split(stored, "$")
	// "$argon2id$v=19$m=..,t=..,p=..$salt$sum" — the leading "$" makes fields[0] empty.
	if len(fields) != 6 || fields[0] != "" {
		return Params{}, ErrMalformed
	}
	if fields[1] != Algorithm {
		return Params{}, fmt.Errorf("%w: %q", ErrUnsupported, fields[1])
	}

	raw := fields[2]
	version, err := strconv.Atoi(strings.TrimPrefix(raw, "v="))
	if err != nil || !strings.HasPrefix(raw, "v=") {
		return Params{}, ErrMalformed
	}
	if version != phcVersion {
		return Params{}, fmt.Errorf("%w: version %d", ErrUnsupported, version)
	}

	var p Params
	p.Version = version
	for _, kv := range strings.Split(fields[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return Params{}, ErrMalformed
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil || n == 0 {
			return Params{}, ErrMalformed
		}
		switch k {
		case "m":
			p.MemoryKiB = uint32(n)
		case "t":
			p.Time = uint32(n)
		case "p":
			if n > maxThreads {
				return Params{}, fmt.Errorf("%w: p=%d", ErrUnsupported, n)
			}
			p.Threads = uint8(n)
		default:
			return Params{}, ErrMalformed
		}
	}
	if p.MemoryKiB == 0 || p.Time == 0 ||
		p.Time > maxTime || p.MemoryKiB > maxMemoryKiB || p.Threads == 0 {
		return Params{}, fmt.Errorf("%w: m=%d,t=%d,p=%d", ErrUnsupported, p.MemoryKiB, p.Time, p.Threads)
	}

	salt, err := base64.RawStdEncoding.DecodeString(fields[4])
	if err != nil || len(salt) < saltLen {
		return Params{}, ErrMalformed
	}
	sum, err := base64.RawStdEncoding.DecodeString(fields[5])
	if err != nil || len(sum) == 0 || len(sum) > maxKeyLen {
		return Params{}, ErrMalformed
	}
	p.Salt, p.Sum = salt, sum
	p.KeyLen = uint32(len(sum))
	return p, nil
}
