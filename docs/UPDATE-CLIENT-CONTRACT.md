# Nivra update-client contract

## Feed selection

The Windows x64 client selects exactly one channel endpoint:

```text
https://doodooapp.github.io/nivra-releases/channels/{channel}/windows-x64.json
```

Allowed channel values are `alpha`, `beta`, and `stable`.

## Required verification order

The client must perform these checks in order:

1. Fetch with HTTPS, a short timeout, response-size limit, redirect limit, and `Cache-Control: no-cache`.
2. Parse strict JSON and reject unknown top-level or payload fields.
3. Require `schemaVersion == 1`, `product == "Nivra"`, `platform == "windows"`, and `architecture == "x64"`.
4. Verify the Ed25519 signature over the compact JSON encoding of the `payload` object with the public key embedded in the executable.
5. Require the signature `keyId` to match the embedded key.
6. Compare semantic versions and ignore versions that are not newer than the installed version.
7. Apply channel, minimum-version, mandatory-update, and deterministic rollout rules.
8. Download the installer into a newly created directory below `%LOCALAPPDATA%\Nivra\Updates`.
9. Enforce a maximum download size and require the exact manifest byte size.
10. Compute SHA-256 and compare it in constant time with `asset.sha256`.
11. Launch the installer only after every check succeeds.
12. Record pending update state and verify the installed version after restart.

## Signature bytes

The publisher signs the exact UTF-8 output of Go `encoding/json.Marshal(payload)` using the `Payload` struct field order defined in `internal/manifest/manifest.go`. The app implementation should reproduce the same canonical field order rather than re-serializing an unordered map.

## Rollout

Recommended deterministic bucket:

```text
bucket = first_uint32(SHA256(installation_id + "\n" + version)) % 100
eligible = bucket < rolloutPercent
```

`installation_id` must be a random local UUID. It must not contain a username, device name, hardware serial, email address, or other personal data. The release server does not need to receive it.

## Update user experience

- Check in the background no more than once every six hours, and on explicit user request.
- Never install silently while the user has unsaved work.
- Show version, channel, release notes, download size, and whether the update is mandatory.
- Allow `Download and restart` or `Later` for normal updates.
- For a mandatory update, allow the user to save work before installation.
- Preserve a manual `Check for updates` action and visible error details.
- Do not fall back to an unsigned installer or web page.

## Failure behavior

A missing channel manifest means no release is published for that channel. Network errors, malformed JSON, signature failures, digest mismatches, and installer-launch failures must leave the current Nivra installation untouched.
