package manifest

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := NewPayload(
		"alpha",
		"0.1.0-alpha.3",
		"Fixes",
		"https://github.com/example/nivra-releases/releases/tag/v0.1.0-alpha.3",
		Asset{Name: "Nivra-Setup-0.1.0-alpha.3.exe", URL: "https://example.invalid/file.exe", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 42},
		"0.1.0-alpha.2",
		false,
		100,
		time.Unix(0, 0),
	)
	envelope, err := Sign(payload, priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(envelope, pub); err != nil {
		t.Fatalf("verification failed: %v", err)
	}
	envelope.Payload.ReleaseNotes = "tampered"
	if err := Verify(envelope, pub); err == nil {
		t.Fatal("expected tampered manifest to fail")
	}
}

func TestValidVersion(t *testing.T) {
	valid := []string{"0.1.0", "0.1.0-alpha.3", "1.2.3-beta.1+build.5"}
	for _, v := range valid {
		if !ValidVersion(v) {
			t.Fatalf("expected %s to be valid", v)
		}
	}
	invalid := []string{"v", "1", "1.2", "01.2.3", "1.2.3-"}
	for _, v := range invalid {
		if ValidVersion(v) {
			t.Fatalf("expected %s to be invalid", v)
		}
	}
}
