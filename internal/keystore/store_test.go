package keystore

import (
	"path/filepath"
	"testing"
)

func TestLoadOrCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.json")
	pub1, priv1, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected key to be created")
	}
	pub2, priv2, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("expected existing key to be loaded")
	}
	if string(pub1) != string(pub2) || string(priv1) != string(priv2) {
		t.Fatal("loaded key differs from created key")
	}
}
