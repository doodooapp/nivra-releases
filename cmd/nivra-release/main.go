package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"nivra-release-server/internal/keystore"
	"nivra-release-server/internal/manifest"
	"nivra-release-server/internal/runner"
)

const toolVersion = "1.0.1"

var safeAssetName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "version", "--version", "-version":
		fmt.Println(toolVersion)
		return
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "publish":
		err = cmdPublish(os.Args[2:])
	case "backup-key":
		err = cmdBackupKey(os.Args[2:])
	case "restore-key":
		err = cmdRestoreKey(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "validate-site":
		err = cmdValidateSite(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`Nivra Release Tool

Commands:
  keygen         Create or load the local Ed25519 signing key and write the public key set.
  publish        Upload an installer, sign an update manifest, and publish the channel.
  backup-key     Export a password-encrypted recovery copy of the signing key.
  restore-key    Restore a signing key from an encrypted recovery copy.
  verify         Verify one signed manifest.
  validate-site  Verify every signed manifest below a site directory.
  status         Print the configured release URLs.
  version        Print the tool version.

Use "NivraRelease.exe <command> -h" for command-specific flags.
`)
}

func backupPassphrase() ([]byte, error) {
	value := os.Getenv("NIVRA_RELEASE_BACKUP_PASSPHRASE")
	if len([]rune(value)) < 12 {
		return nil, errors.New("set NIVRA_RELEASE_BACKUP_PASSPHRASE to a passphrase of at least 12 characters")
	}
	return []byte(value), nil
}

func cmdBackupKey(args []string) error {
	fs := flag.NewFlagSet("backup-key", flag.ContinueOnError)
	out := fs.String("out", "", "encrypted backup output path")
	keyPath := fs.String("key", "", "local protected signing key path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("out is required")
	}
	passphrase, err := backupPassphrase()
	if err != nil {
		return err
	}
	pub, priv, err := keystore.Load(*keyPath)
	if err != nil {
		return err
	}
	if err := keystore.ExportBackup(*out, priv, passphrase); err != nil {
		return err
	}
	fmt.Printf("Encrypted signing-key backup written to %s\nKey ID: %s\n", *out, manifest.KeyID(pub))
	return nil
}

func cmdRestoreKey(args []string) error {
	fs := flag.NewFlagSet("restore-key", flag.ContinueOnError)
	in := fs.String("in", "", "encrypted backup input path")
	keyPath := fs.String("key", "", "local protected signing key path")
	force := fs.Bool("force", false, "replace an existing local signing key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return errors.New("in is required")
	}
	destination := *keyPath
	if destination == "" {
		var err error
		destination, err = keystore.DefaultPath()
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(destination); err == nil && !*force {
		return fmt.Errorf("a local signing key already exists at %s; use --force only when you intend to replace it", destination)
	}
	passphrase, err := backupPassphrase()
	if err != nil {
		return err
	}
	pub, priv, err := keystore.ImportBackup(*in, passphrase)
	if err != nil {
		return err
	}
	if err := keystore.Save(destination, priv); err != nil {
		return err
	}
	fmt.Printf("Signing key restored to %s\nKey ID: %s\n", destination, manifest.KeyID(pub))
	return nil
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	site := fs.String("site", "site", "site directory")
	keyPath := fs.String("key", "", "local protected signing key path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	pub, _, created, err := keystore.LoadOrCreate(*keyPath)
	if err != nil {
		return err
	}
	if err := writeKeySet(*site, pub); err != nil {
		return err
	}
	path := *keyPath
	if path == "" {
		path, _ = keystore.DefaultPath()
	}
	state := "loaded"
	if created {
		state = "created"
	}
	fmt.Printf("Signing key %s.\n", state)
	fmt.Printf("Key ID: %s\n", manifest.KeyID(pub))
	fmt.Printf("Protected key: %s\n", path)
	fmt.Printf("Public key set: %s\n", filepath.Join(*site, "keys", "release-keys.json"))
	return nil
}

type publishOptions struct {
	owner          string
	repo           string
	workdir        string
	installer      string
	version        string
	channel        string
	notes          string
	notesFile      string
	minimum        string
	mandatory      bool
	rollout        int
	keyPath        string
	dryRun         bool
	skipGitHub     bool
	publishedAtRaw string
}

func cmdPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	o := publishOptions{}
	fs.StringVar(&o.owner, "owner", "doodooapp", "GitHub owner")
	fs.StringVar(&o.repo, "repo", "nivra-releases", "GitHub release repository")
	fs.StringVar(&o.workdir, "workdir", ".", "local release repository directory")
	fs.StringVar(&o.installer, "installer", "", "path to Nivra installer")
	fs.StringVar(&o.version, "version", "", "release version, for example 0.1.0-alpha.3")
	fs.StringVar(&o.channel, "channel", "alpha", "alpha, beta, or stable")
	fs.StringVar(&o.notes, "notes", "", "release notes")
	fs.StringVar(&o.notesFile, "notes-file", "", "release notes Markdown file")
	fs.StringVar(&o.minimum, "minimum-version", "", "oldest version allowed to install this update")
	fs.BoolVar(&o.mandatory, "mandatory", false, "mark update as mandatory")
	fs.IntVar(&o.rollout, "rollout", 100, "rollout percentage from 0 to 100")
	fs.StringVar(&o.keyPath, "key", "", "local protected signing key path")
	fs.BoolVar(&o.dryRun, "dry-run", false, "build and verify manifest without publishing")
	fs.BoolVar(&o.skipGitHub, "skip-github", false, "write signed manifest without creating a GitHub release")
	fs.StringVar(&o.publishedAtRaw, "published-at", "", "override RFC3339 publish timestamp (testing only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return publish(o)
}

func publish(o publishOptions) error {
	o.owner = strings.TrimSpace(o.owner)
	o.repo = strings.TrimSpace(o.repo)
	o.channel = strings.ToLower(strings.TrimSpace(o.channel))
	o.version = manifest.NormalizeVersion(o.version)
	o.minimum = manifest.NormalizeVersion(o.minimum)
	if o.owner == "" || o.repo == "" {
		return errors.New("owner and repo are required")
	}
	if !manifest.ValidVersion(o.version) {
		return fmt.Errorf("invalid version %q", o.version)
	}
	if o.minimum != "" && !manifest.ValidVersion(o.minimum) {
		return fmt.Errorf("invalid minimum version %q", o.minimum)
	}
	switch o.channel {
	case "alpha", "beta", "stable":
	default:
		return fmt.Errorf("invalid channel %q", o.channel)
	}
	if o.rollout < 0 || o.rollout > 100 {
		return errors.New("rollout must be between 0 and 100")
	}
	if o.installer == "" {
		return errors.New("installer is required")
	}
	installer, err := filepath.Abs(o.installer)
	if err != nil {
		return err
	}
	info, err := os.Stat(installer)
	if err != nil {
		return fmt.Errorf("installer: %w", err)
	}
	if info.IsDir() || info.Size() <= 0 {
		return errors.New("installer must be a non-empty file")
	}
	assetName := filepath.Base(installer)
	if !safeAssetName.MatchString(assetName) {
		return fmt.Errorf("installer filename %q contains unsupported characters; use letters, numbers, dots, dashes, and underscores", assetName)
	}
	if !strings.EqualFold(filepath.Ext(assetName), ".exe") {
		return errors.New("the Windows installer must have an .exe extension")
	}
	workdir, err := filepath.Abs(o.workdir)
	if err != nil {
		return err
	}
	siteDir := filepath.Join(workdir, "site")
	if stat, err := os.Stat(siteDir); err != nil || !stat.IsDir() {
		return fmt.Errorf("site directory was not found at %s", siteDir)
	}
	notes, err := releaseNotes(o)
	if err != nil {
		return err
	}
	pub, priv, err := keystore.Load(o.keyPath)
	if err != nil {
		return fmt.Errorf("load signing key (run keygen first): %w", err)
	}
	if err := verifySiteKey(siteDir, pub); err != nil {
		return err
	}
	digest, err := sha256File(installer)
	if err != nil {
		return err
	}
	tag := manifest.TagForVersion(o.version)
	repoFull := o.owner + "/" + o.repo
	releasePage := "https://github.com/" + repoFull + "/releases/tag/" + tag
	assetURL := "https://github.com/" + repoFull + "/releases/download/" + url.PathEscape(tag) + "/" + url.PathEscape(assetName)
	publishedAt := time.Now().UTC()
	if o.publishedAtRaw != "" {
		publishedAt, err = time.Parse(time.RFC3339, o.publishedAtRaw)
		if err != nil {
			return fmt.Errorf("published-at: %w", err)
		}
	}
	payload := manifest.NewPayload(o.channel, o.version, notes, releasePage, manifest.Asset{
		Name: assetName, URL: assetURL, SHA256: digest, Size: info.Size(),
	}, o.minimum, o.mandatory, o.rollout, publishedAt)
	envelope, err := manifest.Sign(payload, priv)
	if err != nil {
		return err
	}
	if err := manifest.Verify(envelope, pub); err != nil {
		return fmt.Errorf("self-verification failed: %w", err)
	}
	encoded, err := manifest.MarshalIndent(envelope)
	if err != nil {
		return err
	}
	if o.dryRun {
		out := filepath.Join(workdir, "dist", fmt.Sprintf("manifest-%s-%s.json", o.channel, o.version))
		if err := writeAtomic(out, append(encoded, '\n')); err != nil {
			return err
		}
		fmt.Printf("Dry run complete.\nManifest: %s\nSHA-256: %s\nKey ID: %s\n", out, digest, manifest.KeyID(pub))
		return nil
	}
	if !o.skipGitHub {
		if _, err := runner.LookPath("gh"); err != nil {
			return err
		}
		if _, err := runner.LookPath("git"); err != nil {
			return err
		}
		if _, err := runner.Run(workdir, "gh", "auth", "status"); err != nil {
			return errors.New("GitHub CLI is not authenticated; run gh auth login")
		}
		if dirty, err := gitDirty(workdir); err != nil {
			return err
		} else if dirty {
			return errors.New("release repository has uncommitted changes; commit or discard them before publishing")
		}
		if _, err := runner.Run(workdir, "git", "pull", "--ff-only", "origin", "main"); err != nil {
			return err
		}
		if releaseExists(workdir, repoFull, tag) {
			return fmt.Errorf("GitHub release %s already exists; release versions are immutable", tag)
		}
		tempNotes, err := os.CreateTemp("", "nivra-release-notes-*.md")
		if err != nil {
			return err
		}
		tempNotesPath := tempNotes.Name()
		defer os.Remove(tempNotesPath)
		if _, err := tempNotes.WriteString(notes + "\n"); err != nil {
			tempNotes.Close()
			return err
		}
		if err := tempNotes.Close(); err != nil {
			return err
		}
		createArgs := []string{"release", "create", tag, installer, "--repo", repoFull, "--title", "Nivra " + o.version, "--notes-file", tempNotesPath, "--draft"}
		if o.channel != "stable" {
			createArgs = append(createArgs, "--prerelease", "--latest=false")
		} else {
			createArgs = append(createArgs, "--latest")
		}
		fmt.Printf("Uploading %s to GitHub Release %s...\n", assetName, tag)
		if _, err := runner.Run(workdir, "gh", createArgs...); err != nil {
			return err
		}
		if _, err := runner.Run(workdir, "gh", "release", "edit", tag, "--repo", repoFull, "--draft=false"); err != nil {
			return fmt.Errorf("release was created as a draft but could not be published: %w", err)
		}
	}
	channelPath := filepath.Join(siteDir, "channels", o.channel, "windows-x64.json")
	releasePath := filepath.Join(siteDir, "releases", o.version, "windows-x64.json")
	if err := writeAtomic(channelPath, append(encoded, '\n')); err != nil {
		return err
	}
	if err := writeAtomic(releasePath, append(encoded, '\n')); err != nil {
		return err
	}
	if err := updateStatus(siteDir, envelope.Payload); err != nil {
		return err
	}
	if err := validateSite(siteDir); err != nil {
		return fmt.Errorf("site validation failed before push: %w", err)
	}
	if !o.skipGitHub {
		if _, err := runner.Run(workdir, "git", "add", "site"); err != nil {
			return err
		}
		message := fmt.Sprintf("release: Nivra %s (%s)", o.version, o.channel)
		if _, err := runner.Run(workdir, "git", "commit", "-m", message); err != nil {
			return err
		}
		if _, err := runner.Run(workdir, "git", "push", "origin", "main"); err != nil {
			return fmt.Errorf("release is published but manifest push failed; retry git push: %w", err)
		}
	}
	fmt.Printf("Published Nivra %s to %s.\n", o.version, o.channel)
	fmt.Printf("Manifest: https://%s.github.io/%s/channels/%s/windows-x64.json\n", o.owner, o.repo, o.channel)
	fmt.Printf("Installer SHA-256: %s\n", digest)
	return nil
}

func releaseNotes(o publishOptions) (string, error) {
	if o.notesFile != "" {
		data, err := os.ReadFile(o.notesFile)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(o.notes) != "" {
			return strings.TrimSpace(o.notes) + "\n\n" + strings.TrimSpace(string(data)), nil
		}
		return strings.TrimSpace(string(data)), nil
	}
	if strings.TrimSpace(o.notes) == "" {
		return "Nivra " + o.version, nil
	}
	return strings.TrimSpace(o.notes), nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "signed manifest JSON file")
	keySetPath := fs.String("keys", "site/keys/release-keys.json", "public key set JSON file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return errors.New("manifest is required")
	}
	envelope, err := readEnvelope(*manifestPath)
	if err != nil {
		return err
	}
	keys, err := readKeySet(*keySetPath)
	if err != nil {
		return err
	}
	pub, err := publicKeyByID(keys, envelope.Signature.KeyID)
	if err != nil {
		return err
	}
	if err := manifest.Verify(envelope, pub); err != nil {
		return err
	}
	fmt.Printf("Valid manifest: Nivra %s (%s)\n", envelope.Payload.Version, envelope.Payload.Channel)
	return nil
}

func cmdValidateSite(args []string) error {
	fs := flag.NewFlagSet("validate-site", flag.ContinueOnError)
	site := fs.String("site", "site", "site directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateSite(*site); err != nil {
		return err
	}
	fmt.Println("All release manifests are valid.")
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	owner := fs.String("owner", "doodooapp", "GitHub owner")
	repo := fs.String("repo", "nivra-releases", "GitHub repository")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Printf("Status page: https://%s.github.io/%s/\n", *owner, *repo)
	for _, channel := range []string{"alpha", "beta", "stable"} {
		fmt.Printf("%-6s: https://%s.github.io/%s/channels/%s/windows-x64.json\n", channel, *owner, *repo, channel)
	}
	return nil
}

func validateSite(site string) error {
	keys, err := readKeySet(filepath.Join(site, "keys", "release-keys.json"))
	if err != nil {
		return err
	}
	var files []string
	for _, root := range []string{filepath.Join(site, "channels"), filepath.Join(site, "releases")} {
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	sort.Strings(files)
	for _, path := range files {
		envelope, err := readEnvelope(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		pub, err := publicKeyByID(keys, envelope.Signature.KeyID)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := manifest.Verify(envelope, pub); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func writeKeySet(site string, pub ed25519.PublicKey) error {
	keyID := manifest.KeyID(pub)
	path := filepath.Join(site, "keys", "release-keys.json")
	createdAt := time.Now().UTC().Format(time.RFC3339)
	if existing, err := readKeySet(path); err == nil {
		for _, key := range existing.Keys {
			if key.ID == keyID {
				createdAt = key.CreatedAt
				break
			}
		}
	}
	set := manifest.KeySet{
		SchemaVersion: 1,
		ActiveKeyID:   keyID,
		Keys: []manifest.KeyRecord{{
			ID: keyID, Algorithm: "ed25519", PublicKey: base64.StdEncoding.EncodeToString(pub), CreatedAt: createdAt,
		}},
	}
	encoded, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(encoded, '\n'))
}

func verifySiteKey(site string, pub ed25519.PublicKey) error {
	set, err := readKeySet(filepath.Join(site, "keys", "release-keys.json"))
	if err != nil {
		return fmt.Errorf("read site public key set: %w", err)
	}
	if set.ActiveKeyID != manifest.KeyID(pub) {
		return fmt.Errorf("local signing key %s does not match site active key %s", manifest.KeyID(pub), set.ActiveKeyID)
	}
	trusted, err := publicKeyByID(set, set.ActiveKeyID)
	if err != nil {
		return err
	}
	if string(trusted) != string(pub) {
		return errors.New("site public key does not match local signing key")
	}
	return nil
}

func readKeySet(path string) (manifest.KeySet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.KeySet{}, err
	}
	var set manifest.KeySet
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&set); err != nil {
		return manifest.KeySet{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return manifest.KeySet{}, errors.New("unexpected trailing JSON data in key set")
	} else if !errors.Is(err, io.EOF) {
		return manifest.KeySet{}, err
	}
	if set.SchemaVersion != 1 || set.ActiveKeyID == "" || len(set.Keys) == 0 {
		return manifest.KeySet{}, errors.New("invalid release key set")
	}
	return set, nil
}

func publicKeyByID(set manifest.KeySet, id string) (ed25519.PublicKey, error) {
	for _, record := range set.Keys {
		if record.ID != id {
			continue
		}
		if record.Algorithm != "ed25519" {
			return nil, fmt.Errorf("unsupported key algorithm %q", record.Algorithm)
		}
		decoded, err := base64.StdEncoding.DecodeString(record.PublicKey)
		if err != nil {
			return nil, err
		}
		if len(decoded) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 public key length")
		}
		return ed25519.PublicKey(decoded), nil
	}
	return nil, fmt.Errorf("public key %q was not found", id)
}

func readEnvelope(path string) (manifest.Envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Envelope{}, err
	}
	return manifest.Parse(data)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func gitDirty(workdir string) (bool, error) {
	result, err := runner.Run(workdir, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(result.Stdout) != "", nil
}

func releaseExists(workdir, repo, tag string) bool {
	cmd := exec.Command("gh", "release", "view", tag, "--repo", repo)
	cmd.Dir = workdir
	return cmd.Run() == nil
}

type siteStatus struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Product       string                 `json:"product"`
	UpdatedAt     string                 `json:"updatedAt"`
	Channels      map[string]channelInfo `json:"channels"`
}

type channelInfo struct {
	Version     string `json:"version"`
	PublishedAt string `json:"publishedAt"`
	Manifest    string `json:"manifest"`
	Mandatory   bool   `json:"mandatory"`
	Rollout     int    `json:"rolloutPercent"`
}

func updateStatus(site string, payload manifest.Payload) error {
	path := filepath.Join(site, "status.json")
	status := siteStatus{SchemaVersion: 1, Product: "Nivra", Channels: map[string]channelInfo{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &status)
		if status.Channels == nil {
			status.Channels = map[string]channelInfo{}
		}
	}
	status.SchemaVersion = 1
	status.Product = "Nivra"
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	status.Channels[payload.Channel] = channelInfo{
		Version: payload.Version, PublishedAt: payload.PublishedAt,
		Manifest:  "channels/" + payload.Channel + "/windows-x64.json",
		Mandatory: payload.Mandatory, Rollout: payload.RolloutPercent,
	}
	encoded, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(encoded, '\n'))
}
