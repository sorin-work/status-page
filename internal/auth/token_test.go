package auth

import "testing"

func TestNewTokenIsRandom(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewToken()
	if a == b || len(a) != 43 {
		t.Errorf("tokens %q and %q", a, b)
	}
}

func TestHashTokenIsStable(t *testing.T) {
	a, b := "abc", "abc"
	if HashToken(a) != HashToken(b) || HashToken(a) == HashToken("abd") {
		t.Error("HashToken is not a stable function of its input")
	}
	if len(HashToken("abc")) != 64 {
		t.Error("expected a hex sha256")
	}
}
