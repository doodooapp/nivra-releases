# Release security model

## Trust anchors

The root of trust is the Ed25519 public key embedded in the Nivra executable. The matching private key is generated locally and protected with Windows DPAPI. It is never committed, uploaded to GitHub, or placed in a release asset.

GitHub HTTPS protects transport, while the Ed25519 signature protects manifest authenticity independently of transport. SHA-256 and exact size protect the installer bytes referenced by the signed manifest.

## Threats addressed

- corrupted or truncated downloads;
- a modified installer at the signed URL;
- an attacker changing version, release notes, rollout, mandatory state, or download URL without the signing key;
- accidental publication of an unverified installer;
- stale local release metadata being overwritten by a dirty worktree;
- reuse of an existing version tag through the normal publisher.

## Threats not fully addressed

- compromise of the Windows account that can decrypt the active signing key;
- compromise of both the GitHub account and the release-signing key;
- malicious code introduced before the installer is built;
- operating-system compromise on a client machine;
- Windows SmartScreen reputation and publisher identity before code signing is introduced.

## Operational rules

1. Keep GitHub two-factor authentication enabled.
2. Keep the encrypted `.nivrakey` recovery backup offline or in a strongly protected vault.
3. Store the recovery password separately from the backup file.
4. Never send the private key, the DPAPI key file, or the recovery password through chat or email.
5. Never reuse a version number or overwrite a published installer.
6. Publish alpha first, validate it on a clean Windows machine, then promote a separately versioned build to beta or stable.
7. Add Authenticode code signing before broad public distribution.
8. Rotate the release key only through a client update that contains and trusts the next public key.

## Key rotation

A server-side change to `release-keys.json` is not sufficient because that file is not a root of trust. A future Nivra version must ship with both the old trusted key and the new key, after which a later version may remove the old key. The transition manifest must still be signed by a key already trusted by installed clients.
