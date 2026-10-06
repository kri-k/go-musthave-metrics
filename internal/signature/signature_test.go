package signature

import "testing"

func TestKnownHMACSHA256(t *testing.T) {
	const expected = "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	body := []byte("The quick brown fox jumps over the lazy dog")
	if got := Sign(body, "key"); got != expected {
		t.Fatalf("got %s, want %s", got, expected)
	}
	if !Verify(body, "key", expected) {
		t.Fatal("valid signature rejected")
	}
}
