package keystore

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type storedKey struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	CreatedAt  string `json:"createdAt"`
	Ciphertext string `json:"ciphertext"`
}

func DefaultPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = config
	}
	return filepath.Join(base, "Nivra", "Release", "signing-key.json"), nil
}

func LoadOrCreate(path string) (ed25519.PublicKey, ed25519.PrivateKey, bool, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, nil, false, err
		}
	}
	data, err := os.ReadFile(path)
	if err == nil {
		pub, priv, err := decodeStored(data)
		return pub, priv, false, err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, false, err
	}
	if err := Save(path, priv); err != nil {
		return nil, nil, false, err
	}
	return pub, priv, true, nil
}

func Load(path string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return decodeStored(data)
}

func Save(path string, privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid Ed25519 private key")
	}
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	protected, err := protect([]byte(privateKey))
	if err != nil {
		return fmt.Errorf("protect release signing key: %w", err)
	}
	record := storedKey{
		Version:    1,
		Algorithm:  "ed25519",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Ciphertext: base64.StdEncoding.EncodeToString(protected),
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func decodeStored(data []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	var record storedKey
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, nil, err
	}
	if record.Version != 1 || record.Algorithm != "ed25519" {
		return nil, nil, errors.New("unsupported release key file")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return nil, nil, err
	}
	plain, err := unprotect(ciphertext)
	if err != nil {
		return nil, nil, fmt.Errorf("unprotect release signing key: %w", err)
	}
	if len(plain) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid release signing key length")
	}
	priv := ed25519.PrivateKey(append([]byte(nil), plain...))
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, errors.New("could not derive release public key")
	}
	return pub, priv, nil
}
