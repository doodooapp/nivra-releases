# Nivra Release Server

**Release tooling version: 1.0.1**

This repository is the public update infrastructure for the Nivra desktop application.
It intentionally contains **no Nivra application source code**. It hosts:

- small signed update manifests through GitHub Pages;
- Windows installers as GitHub Release assets;
- a release status page;
- the public Ed25519 verification key;
- release publishing and validation tooling.

## Architecture

```text
Nivra desktop app
       |
       | HTTPS: signed JSON manifest
       v
GitHub Pages
  channels/alpha/windows-x64.json
  channels/beta/windows-x64.json
  channels/stable/windows-x64.json
       |
       | signed installer URL + SHA-256
       v
GitHub Releases
  Nivra-Setup-<version>.exe
```

The installer binaries are not stored in the Pages site. Pages only serves small metadata files. Every manifest is signed with an offline Ed25519 key, and every installer is identified by its SHA-256 digest and exact byte size.

## One-time setup on Windows

1. Extract the release-server ZIP.
2. Double-click **`SETUP-RELEASE-SERVER.cmd`**.
3. Read the public-repository warning and type `PUBLIC`.
4. Complete the GitHub browser login when requested.
5. Create and safely store the encrypted signing-key recovery backup.

The setup installs Git and GitHub CLI with WinGet when required, creates `doodooapp/nivra-releases`, enables GitHub Pages, generates the signing key, and pushes the initial site. Version 1.0.1 is resumable: it safely continues from the partially initialized local folder left by a failed 1.0.0 attempt.

The persistent working copy is stored in:

```text
Documents\Nivra Release Server
```

The active signing key is protected by Windows DPAPI and stored outside the repository under the current Windows profile.

## Publish a Nivra update

The simplest method is to double-click **`PUBLISH-NIVRA.cmd`** and provide:

- the new `Nivra-Setup-<version>.exe`;
- a new semantic version;
- the target channel: `alpha`, `beta`, or `stable`;
- release notes.

PowerShell automation example:

```powershell
.\scripts\Publish-NivraRelease.ps1 `
  -Installer "C:\Builds\Nivra-Setup-0.1.0-alpha.3.exe" `
  -Version "0.1.0-alpha.3" `
  -Channel alpha `
  -MinimumVersion "0.1.0-alpha.2" `
  -Rollout 100
```

The publisher performs the following transaction:

1. verifies a clean local release repository;
2. computes the installer SHA-256 and size;
3. creates a draft GitHub Release and uploads the installer;
4. publishes the GitHub Release;
5. creates and Ed25519-signs the channel manifest;
6. validates all manifests in the repository;
7. commits and pushes the metadata;
8. lets GitHub Actions deploy the updated Pages feed.

A version must never be reused. Publish `0.1.0-alpha.4` instead of replacing the assets for `0.1.0-alpha.3`.

## Update endpoints

```text
Status:
https://doodooapp.github.io/nivra-releases/

Alpha:
https://doodooapp.github.io/nivra-releases/channels/alpha/windows-x64.json

Beta:
https://doodooapp.github.io/nivra-releases/channels/beta/windows-x64.json

Stable:
https://doodooapp.github.io/nivra-releases/channels/stable/windows-x64.json
```

Nivra should use `Cache-Control: no-cache` when checking these endpoints and verify the Ed25519 signature before trusting any URL, version, release note, rollout value, or digest in a manifest.

## Release channels

- **Alpha** — development builds and incomplete features.
- **Beta** — broader testing after alpha validation.
- **Stable** — production builds intended for normal users.

The application stores the selected channel locally. Stable users must not automatically receive alpha or beta builds.

## Gradual rollout

`rolloutPercent` can be set between `0` and `100`. Nivra should hash a locally generated, non-personal installation identifier with the release version and include the installation only when the deterministic bucket falls within the rollout percentage. This keeps rollout decisions stable without sending telemetry to the release service.

## Signing-key recovery

Run **`BACKUP-SIGNING-KEY.cmd`** whenever a new backup is needed. The generated `.nivrakey` file is encrypted with the recovery password. Store the file and password separately.

On a replacement computer:

1. install the release-server package;
2. copy the `.nivrakey` file locally;
3. run **`RESTORE-SIGNING-KEY.cmd`**;
4. verify that the displayed key ID matches `site/keys/release-keys.json`.

Losing both the original Windows key and the encrypted recovery backup means existing Nivra installations cannot safely trust newly published updates.

## Validation

```powershell
go test ./...
go run ./cmd/nivra-release validate-site --site site
```

GitHub Actions runs the same checks before Pages deployment.

See [`docs/UPDATE-CLIENT-CONTRACT.md`](docs/UPDATE-CLIENT-CONTRACT.md) for the application-side implementation contract and [`docs/SECURITY.md`](docs/SECURITY.md) for the trust model.
