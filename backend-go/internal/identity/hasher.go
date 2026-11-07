package identity

import (
	"errors"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/password"
)

// argon2id is the production Hasher: ADR-019's algorithm at the parameters recorded in
// internal/password. It is a thin adapter whose only real job is to translate "the password was
// wrong" from the hashing package's vocabulary into this package's, because the two outcomes
// ErrMismatch and ErrMalformed mean different things to a login flow — one spends the failure
// counter, the other is a broken row that must not be reported to the account's owner as a bad
// guess on their part.
type argon2id struct{}

// NewHasher returns the Argon2id implementation. There is deliberately no option for anything
// else: the legacy service's bcrypt hashes are being replaced, not carried forward, and an
// injectable "algorithm" is how a weaker one comes back.
func NewHasher() Hasher { return argon2id{} }

func (argon2id) Hash(plaintext string) (string, error) { return password.Hash(plaintext) }

func (argon2id) Verify(stored, plaintext string) error {
	switch err := password.Verify(stored, plaintext); {
	case errors.Is(err, password.ErrMismatch):
		return ErrCredentialMismatch
	case err != nil:
		return err
	}
	return nil
}
