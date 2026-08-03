package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
)

func TestBackupRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.json")
	pass := []byte("this is a strong test passphrase")
	if err := ExportBackup(path, priv, pass); err != nil {
		t.Fatal(err)
	}
	pub2, priv2, err := ImportBackup(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if string(pub) != string(pub2) || string(priv) != string(priv2) {
		t.Fatal("backup round trip changed key")
	}
	if _, _, err := ImportBackup(path, []byte("wrong password value")); err == nil {
		t.Fatal("expected wrong passphrase to fail")
	}
}
