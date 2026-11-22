package security

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	password := "correct horse battery staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == password {
		t.Fatal("password hash must not contain the plaintext password")
	}
	if !VerifyPassword(hash, password) {
		t.Fatal("VerifyPassword() rejected the correct password")
	}
	if VerifyPassword(hash, "incorrect password") {
		t.Fatal("VerifyPassword() accepted an incorrect password")
	}
}

func TestPasswordHashesUseIndependentSalts(t *testing.T) {
	first, err := HashPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("independent password hashes must use independent salts")
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, hash := range []string{"", "argon2id", "$argon2id$v=19$m=999999999,t=3,p=2$bad$bad"} {
		if VerifyPassword(hash, "anything") {
			t.Fatalf("VerifyPassword() accepted malformed hash %q", hash)
		}
	}
}
