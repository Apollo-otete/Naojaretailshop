package password_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/password"
)

// The vector below was produced by a second, independent Argon2id implementation (libsodium
// through python-argon2, hash_secret with Type.ID) over a fixed 16-byte salt, not by this
// package. Carrying it as a test is what makes "the stored hash is a normal Argon2id hash" a
// checked claim: a self-consistent but non-standard encoding would keep round-tripping here and
// fail everywhere else. The reverse direction — a hash created here verified there — cannot be
// automated without a second implementation at test time, so it needs re-running by hand if the
// encoding is ever touched.
const (
	refPassword = "naoja-reference-credential"
	refSalt     = "0123456789abcdef"
	refHash     = "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$2x8WSIXNLdCDSjaiBZgpg8B9cq2w1ir2HdFIAe46OtU"
)

func TestReferenceVectorVerifiesAgainstThisImplementation(t *testing.T) {
	if err := password.Verify(refHash, refPassword); err != nil {
		t.Fatalf("Verify of an independently produced Argon2id hash: %v", err)
	}
	if err := password.Verify(refHash, refPassword+"x"); !errors.Is(err, password.ErrMismatch) {
		t.Errorf("wrong password: got %v, want ErrMismatch", err)
	}

	p, err := password.Parse(refHash)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Time != 3 || p.MemoryKiB != 65536 || p.Threads != 4 || p.KeyLen != 32 || p.Version != 19 {
		t.Errorf("recorded parameters = t=%d m=%d p=%d keyLen=%d v=%d, want 3/65536/4/32/19",
			p.Time, p.MemoryKiB, p.Threads, p.KeyLen, p.Version)
	}
	if string(p.Salt) != refSalt {
		t.Errorf("salt = %q, want %q", p.Salt, refSalt)
	}
	if got := base64.RawStdEncoding.EncodeToString(p.Sum); !strings.HasSuffix(refHash, "$"+got) {
		t.Errorf("sum %q is not the last field of the vector", got)
	}
}

func TestHashRecordsTheParametersItWasCreatedWith(t *testing.T) {
	stored, err := password.Hash(refPassword)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if stored == refHash {
		t.Fatal("two hashes of the same password share a salt; hashing is deterministic")
	}

	p, err := password.Parse(stored)
	if err != nil {
		t.Fatalf("Parse of our own output: %v", err)
	}
	if p.Time != password.Time || p.MemoryKiB != password.MemoryKiB ||
		p.Threads != password.Threads || p.KeyLen != password.KeyLen {
		t.Errorf("encoded parameters %d/%d/%d/%d differ from the package constants %d/%d/%d/%d",
			p.Time, p.MemoryKiB, p.Threads, p.KeyLen,
			password.Time, password.MemoryKiB, password.Threads, password.KeyLen)
	}
	if len(p.Salt) != 16 {
		t.Errorf("salt length = %d, want 16", len(p.Salt))
	}
	roundTripped := password.Params{Algorithm: password.Algorithm, Version: 19, Time: p.Time,
		MemoryKiB: p.MemoryKiB, Threads: p.Threads, KeyLen: p.KeyLen, Salt: p.Salt, Sum: p.Sum}
	if roundTripped.Encode() != stored {
		t.Errorf("Encode(Parse(x)) != x\n got %q\nwant %q", roundTripped.Encode(), stored)
	}
	if err := password.Verify(stored, refPassword); err != nil {
		t.Errorf("Verify of a freshly created hash: %v", err)
	}
}

// hashAt encodes a hash derived at deliberately non-production parameters, which is how a test
// can represent "a row written before the parameters changed" without paying for the change.
func hashAt(t *testing.T, plaintext string, time, memory uint32, threads uint8, keyLen uint32) string {
	t.Helper()
	salt := []byte(refSalt)
	sum := argon2.IDKey([]byte(plaintext), salt, time, memory, threads, keyLen)
	return password.Params{Algorithm: password.Algorithm, Version: 19, Time: time,
		MemoryKiB: memory, Threads: threads, KeyLen: keyLen, Salt: salt, Sum: sum}.Encode()
}

// ADR-019's verification item is that parameters are recorded and re-verifiable. The only way
// that can be false is a hash written under one set of parameters and checked under another, so
// this test creates one deliberately and asks whether Verify still answers correctly.
func TestVerifyChecksAgainstTheStoredParametersNotTheCurrentOnes(t *testing.T) {
	// Cheap on purpose: a row migrated in from weaker-but-valid settings must still verify.
	stored := hashAt(t, refPassword, 1, 1024, 1, 16)
	if strings.HasPrefix(stored, refHash) {
		t.Fatal("reference parameters were not overridden; the test would prove nothing")
	}
	if err := password.Verify(stored, refPassword); err != nil {
		t.Errorf("Verify under recorded parameters: %v", err)
	}
	if err := password.Verify(stored, "anything else"); !errors.Is(err, password.ErrMismatch) {
		t.Errorf("wrong password under recorded parameters: got %v, want ErrMismatch", err)
	}
	p, err := password.Parse(stored)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Time == password.Time && p.MemoryKiB == password.MemoryKiB {
		t.Error("the recorded parameters were not distinguishable from the current ones")
	}
}

func TestParseRefusesHashesThisBuildMustNotExecute(t *testing.T) {
	good := "MDEyMzQ1Njc4OWFiY2RlZg"
	body := func(params string) string {
		return "$argon2id$v=19$" + params + "$" + good + "$2x8WSIXNLdCDSjaiBZgpg8B9cq2w1ir2HdFIAe46OtU"
	}
	cases := []struct {
		name, stored string
		want         error
	}{
		{"empty", "", password.ErrMalformed},
		{"bare password", "hunter2", password.ErrMalformed},
		{"too few fields", "$argon2id$v=19$m=65536,t=3,p=4$" + good, password.ErrMalformed},
		{"argon2i", strings.Replace(refHash, "argon2id", "argon2i", 1), password.ErrUnsupported},
		{"bcrypt prefix", strings.Replace(refHash, "argon2id", "2y", 1), password.ErrUnsupported},
		{"version 16", strings.Replace(refHash, "v=19", "v=16", 1), password.ErrUnsupported},
		{"4 GiB of memory", body("m=4194304,t=3,p=4"), password.ErrUnsupported},
		{"zero time cost", body("m=65536,t=0,p=4"), password.ErrMalformed},
		{"unknown parameter", body("m=65536,t=3,p=4,x=1"), password.ErrMalformed},
		{"non numeric parameter", body("m=65536,t=three,p=4"), password.ErrMalformed},
		{"padded base64 salt", strings.Replace(refHash, good+"$", good+"==$", 1), password.ErrMalformed},
		{"short salt", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$2x8WSIXNLdCDSjaiBZgpg8B9cq2w1ir2HdFIAe46OtU", password.ErrMalformed},
		{"empty sum", body("m=65536,t=3,p=4") + "$", password.ErrMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := password.Parse(c.stored); !errors.Is(err, c.want) {
				t.Errorf("Parse(%q) = %v, want %v", c.stored, err, c.want)
			}
			// Verify must not derive anything for a hash it will not accept.
			if err := password.Verify(c.stored, refPassword); !errors.Is(err, c.want) {
				t.Errorf("Verify(%q) = %v, want %v", c.stored, err, c.want)
			}
		})
	}
}

// A hash truncated by a bad migration or a column-width change must never authenticate. It is
// reported as malformed when the truncation leaves something undecodable; a truncation that
// happens to decode into a shorter-but-legal sum can only be reported as a mismatch, because
// PHC does not encode the tag length. Either way the answer is no.
func TestTruncatedHashNeverAuthenticates(t *testing.T) {
	// 40 derivations at the production cost would make this the slowest test in the suite, and
	// the behaviour under test is in the encoding, not in the cost.
	crafted := hashAt(t, refPassword, 1, 1024, 1, 32)

	// A sum whose unpadded base64 length leaves 1 remainder does not decode at all.
	if err := password.Verify(crafted[:len(crafted)-2], refPassword); !errors.Is(err, password.ErrMalformed) {
		t.Errorf("Verify(hash with an undecodable sum) = %v, want ErrMalformed", err)
	}
	for n := 1; n < 40; n++ {
		if err := password.Verify(crafted[:len(crafted)-n], refPassword); err == nil {
			t.Errorf("a sum truncated by %d bytes verified the correct password", n)
		}
	}
}
