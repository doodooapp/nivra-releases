package manifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const SchemaVersion = 1

var (
	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	shaPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Payload struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Product          string `json:"product"`
	Channel          string `json:"channel"`
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	Version          string `json:"version"`
	PublishedAt      string `json:"publishedAt"`
	MinimumSupported string `json:"minimumSupportedVersion,omitempty"`
	Mandatory        bool   `json:"mandatory"`
	RolloutPercent   int    `json:"rolloutPercent"`
	ReleaseNotes     string `json:"releaseNotes"`
	ReleasePage      string `json:"releasePage"`
	Asset            Asset  `json:"asset"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId"`
	Value     string `json:"value"`
}

type Envelope struct {
	Payload   Payload   `json:"payload"`
	Signature Signature `json:"signature"`
}

type KeyRecord struct {
	ID        string `json:"id"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
	CreatedAt string `json:"createdAt"`
}

type KeySet struct {
	SchemaVersion int         `json:"schemaVersion"`
	ActiveKeyID   string      `json:"activeKeyId"`
	Keys          []KeyRecord `json:"keys"`
}

func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func ValidVersion(v string) bool {
	return versionPattern.MatchString(NormalizeVersion(v))
}

func TagForVersion(v string) string {
	return "v" + NormalizeVersion(v)
}

func KeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return "nivra-" + hex.EncodeToString(sum[:8])
}

func NewPayload(channel, version, notes, releasePage string, asset Asset, minimum string, mandatory bool, rollout int, publishedAt time.Time) Payload {
	return Payload{
		SchemaVersion:    SchemaVersion,
		Product:          "Nivra",
		Channel:          strings.ToLower(strings.TrimSpace(channel)),
		Platform:         "windows",
		Architecture:     "x64",
		Version:          NormalizeVersion(version),
		PublishedAt:      publishedAt.UTC().Format(time.RFC3339),
		MinimumSupported: NormalizeVersion(minimum),
		Mandatory:        mandatory,
		RolloutPercent:   rollout,
		ReleaseNotes:     strings.TrimSpace(notes),
		ReleasePage:      strings.TrimSpace(releasePage),
		Asset:            asset,
	}
}

func CanonicalPayload(payload Payload) ([]byte, error) {
	if err := ValidatePayload(payload); err != nil {
		return nil, err
	}
	return json.Marshal(payload)
}

func Sign(payload Payload, privateKey ed25519.PrivateKey) (Envelope, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("invalid Ed25519 private key")
	}
	canonical, err := CanonicalPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return Envelope{}, errors.New("could not derive Ed25519 public key")
	}
	sig := ed25519.Sign(privateKey, canonical)
	return Envelope{
		Payload: payload,
		Signature: Signature{
			Algorithm: "ed25519",
			KeyID:     KeyID(publicKey),
			Value:     base64.StdEncoding.EncodeToString(sig),
		},
	}, nil
}

func Verify(envelope Envelope, publicKey ed25519.PublicKey) error {
	if err := ValidateEnvelope(envelope); err != nil {
		return err
	}
	if envelope.Signature.KeyID != KeyID(publicKey) {
		return fmt.Errorf("manifest key id %q does not match trusted key %q", envelope.Signature.KeyID, KeyID(publicKey))
	}
	sig, err := base64.StdEncoding.DecodeString(envelope.Signature.Value)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	canonical, err := CanonicalPayload(envelope.Payload)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, sig) {
		return errors.New("manifest signature verification failed")
	}
	return nil
}

func ValidateEnvelope(envelope Envelope) error {
	if err := ValidatePayload(envelope.Payload); err != nil {
		return err
	}
	if envelope.Signature.Algorithm != "ed25519" {
		return fmt.Errorf("unsupported signature algorithm %q", envelope.Signature.Algorithm)
	}
	if strings.TrimSpace(envelope.Signature.KeyID) == "" {
		return errors.New("signature keyId is required")
	}
	if strings.TrimSpace(envelope.Signature.Value) == "" {
		return errors.New("signature value is required")
	}
	return nil
}

func ValidatePayload(payload Payload) error {
	if payload.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", payload.SchemaVersion)
	}
	if payload.Product != "Nivra" {
		return fmt.Errorf("unexpected product %q", payload.Product)
	}
	switch payload.Channel {
	case "alpha", "beta", "stable":
	default:
		return fmt.Errorf("invalid channel %q", payload.Channel)
	}
	if payload.Platform != "windows" || payload.Architecture != "x64" {
		return fmt.Errorf("unsupported target %s/%s", payload.Platform, payload.Architecture)
	}
	if !ValidVersion(payload.Version) {
		return fmt.Errorf("invalid version %q", payload.Version)
	}
	if payload.MinimumSupported != "" && !ValidVersion(payload.MinimumSupported) {
		return fmt.Errorf("invalid minimum supported version %q", payload.MinimumSupported)
	}
	if _, err := time.Parse(time.RFC3339, payload.PublishedAt); err != nil {
		return fmt.Errorf("invalid publishedAt: %w", err)
	}
	if payload.RolloutPercent < 0 || payload.RolloutPercent > 100 {
		return fmt.Errorf("rolloutPercent must be between 0 and 100")
	}
	if strings.TrimSpace(payload.ReleasePage) == "" {
		return errors.New("releasePage is required")
	}
	if strings.TrimSpace(payload.Asset.Name) == "" {
		return errors.New("asset.name is required")
	}
	if strings.TrimSpace(payload.Asset.URL) == "" {
		return errors.New("asset.url is required")
	}
	if !shaPattern.MatchString(strings.ToLower(payload.Asset.SHA256)) {
		return errors.New("asset.sha256 must contain 64 lowercase hexadecimal characters")
	}
	if payload.Asset.Size <= 0 {
		return errors.New("asset.size must be greater than zero")
	}
	return nil
}

func MarshalIndent(envelope Envelope) ([]byte, error) {
	if err := ValidateEnvelope(envelope); err != nil {
		return nil, err
	}
	return json.MarshalIndent(envelope, "", "  ")
}

func Parse(data []byte) (Envelope, error) {
	var envelope Envelope
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return Envelope{}, errors.New("unexpected trailing JSON data")
	} else if !errors.Is(err, io.EOF) {
		return Envelope{}, err
	}
	if err := ValidateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}
