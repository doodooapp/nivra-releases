package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"time"
)

const backupIterations = 600_000

type backupFile struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
	PublicKey  string `json:"publicKey"`
	CreatedAt  string `json:"createdAt"`
}

func ExportBackup(path string, privateKey ed25519.PrivateKey, passphrase []byte) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid Ed25519 private key")
	}
	if len(passphrase) < 12 {
		return errors.New("backup passphrase must contain at least 12 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key := pbkdf2Key(passphrase, salt, backupIterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	pub, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return errors.New("could not derive public key")
	}
	additionalData := []byte("Nivra release signing key backup v1")
	ciphertext := gcm.Seal(nil, nonce, privateKey, additionalData)
	backup := backupFile{
		Version:    1,
		Algorithm:  "ed25519+a256gcm",
		KDF:        "pbkdf2-hmac-sha256",
		Iterations: backupIterations,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o600)
}

func ImportBackup(path string, passphrase []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var backup backupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, nil, err
	}
	if backup.Version != 1 || backup.Algorithm != "ed25519+a256gcm" || backup.KDF != "pbkdf2-hmac-sha256" {
		return nil, nil, errors.New("unsupported release key backup format")
	}
	if backup.Iterations < 100_000 || backup.Iterations > 5_000_000 {
		return nil, nil, errors.New("invalid backup KDF iteration count")
	}
	salt, err := base64.StdEncoding.DecodeString(backup.Salt)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(backup.Nonce)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(backup.Ciphertext)
	if err != nil {
		return nil, nil, err
	}
	expectedPublic, err := base64.StdEncoding.DecodeString(backup.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	key := pbkdf2Key(passphrase, salt, backup.Iterations, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, nil, errors.New("invalid backup nonce")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte("Nivra release signing key backup v1"))
	if err != nil {
		return nil, nil, errors.New("wrong passphrase or damaged backup")
	}
	if len(plain) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("invalid private key in backup")
	}
	priv := ed25519.PrivateKey(append([]byte(nil), plain...))
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, errors.New("could not derive public key")
	}
	if !hmac.Equal(pub, expectedPublic) {
		return nil, nil, fmt.Errorf("backup public key does not match decrypted private key")
	}
	return pub, priv, nil
}

func pbkdf2Key(password, salt []byte, iterations, keyLen int, newHash func() hash.Hash) []byte {
	hLen := newHash().Size()
	numBlocks := (keyLen + hLen - 1) / hLen
	result := make([]byte, 0, numBlocks*hLen)
	for block := 1; block <= numBlocks; block++ {
		mac := hmac.New(newHash, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(newHash, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		result = append(result, t...)
	}
	return result[:keyLen]
}
