# Changelog

## 1.0.1

- Fixed the first-run crash when `doodooapp/nivra-releases` does not yet exist.
- Native command probes no longer become terminating `NativeCommandError` exceptions in Windows PowerShell 5.1.
- Setup now creates the GitHub repository before continuing and verifies that it exists afterwards.
- Setup can safely resume the partially initialized `Documents\Nivra Release Server` directory left by version 1.0.0.
- Existing `.git` data, signing keys, manifests, and release files are preserved.
- Repaired missing or incorrect `origin` remotes and normalized the branch to `main`.
- Added clearer errors for GitHub authentication, repository creation, push, and Pages activation.
- Added retry logic for the first GitHub Pages workflow dispatch.
- Updated the release tool version to 1.0.1.
