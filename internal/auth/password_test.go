package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Keep the tests fast; the format is the same at any work factor.
	Iterations = 1000
	m.Run()
}

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "pbkdf2-sha256$1000$") {
		t.Errorf("unexpected encoding %q", h)
	}
	if err := VerifyPassword(h, "correct horse"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	if err := VerifyPassword(h, "wrong horse"); !errors.Is(err, ErrMismatch) {
		t.Errorf("expected ErrMismatch, got %v", err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("two hashes of the same password are identical")
	}
}

func TestVerifyKeepsStoredIterations(t *testing.T) {
	h, _ := HashPassword("pw-123456")
	Iterations = 2000
	defer func() { Iterations = 1000 }()
	if err := VerifyPassword(h, "pw-123456"); err != nil {
		t.Errorf("hash made with older work factor rejected: %v", err)
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{
		"",
		"plain",
		"bcrypt$10$abc$def",
		"pbkdf2-sha256$x$abc$def",
		"pbkdf2-sha256$0$abc$def",
		"pbkdf2-sha256$1000$!!$def",
		"pbkdf2-sha256$1000$YWJj$!!",
	} {
		if err := VerifyPassword(h, "pw"); err == nil || errors.Is(err, ErrMismatch) {
			t.Errorf("%q: expected a format error, got %v", h, err)
		}
	}
}
