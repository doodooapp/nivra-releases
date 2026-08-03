# Release operations

## Normal release

1. Build and test a new Nivra installer.
2. Run `PUBLISH-NIVRA.cmd`.
3. Wait for the `Deploy release service` GitHub Action to complete.
4. Run `CHECK-RELEASE-SERVER.cmd`.
5. Use a clean Windows test machine to check, download, install, restart, and verify the update.
6. Increase rollout gradually when appropriate.

## Rollback

Do not mutate the compromised or defective release. Build a new version containing the correction and publish it. If an update must immediately stop spreading, publish a new signed channel manifest with `rolloutPercent` set to `0` through a dedicated future channel-management command, or temporarily remove the channel manifest in an emergency. Existing downloads cannot be recalled.

## Monitoring

GitHub Actions reports deployment success or failure. The public `health.json` confirms Pages availability, while `status.json` identifies the versions assigned to each channel.

## Migration to a custom domain

A future domain such as `updates.nivra.app` can point to the same Pages site or a CDN. Keep the manifest schema and signing key unchanged. Ship the new endpoint in Nivra before removing the old GitHub Pages URL.
