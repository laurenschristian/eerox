# Changelog

All notable changes are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- `device pause`/`unpause`/`rename` never took effect: the cloud API acknowledges writes to the 2.2 device
  endpoint and drops them. Device writes now go to 2.3.
- `device block`/`unblock` never took effect: blocking is the form-encoded `networks/{id}/blacklist`
  resource, not a `blacklisted` field on the device.
- Device writes now read the device back and fail with an error if the change did not apply, instead of
  printing `ok`.
- `profile pause`/`unpause`/`assign`/`unassign` now read the profile back and fail if the change did not
  apply, sharing the same check as device writes.
- Tests no longer deadlock on Windows (completion output filled the stdout pipe) and skip the Unix
  file-mode assertion there.

## [0.2.1] - 2026-09-22

### Fixed
- The Homebrew cask lost its quarantine-strip hook. goreleaser no longer emits it
  by default, and `.goreleaser.yaml` never carried one, so the next release would
  have published a cask whose unsigned binary macOS blocks on install: the `eerox`
  symlink dangles and the command fails with "no such file or directory". The
  published 0.2.0 cask only works because the hook was hand-edited into the tap,
  and that edit also gated it on `Hardware::CPU.arm?`, leaving Intel Macs broken.
  The hook is now in the config, unconditional on macOS, so releases carry it.


### Added
- `set` command to toggle network features (WPA3, SQM, band steering, UPnP, IPv6).
- `profile assign` / `profile unassign` to move a device in or out of a family profile.
- `dns` command to show and pin the network resolvers (`--set`, `--auto`, `--yes`).
- `doctor` command: checks config, session and reachability.
- `rename-batch`: bulk device rename from a `match<TAB>name` file, with `--dry-run`.
- Man pages, shell completions and a Homebrew formula in releases.

## [0.1.0]

### Added
- First release: login (app code flow, token in the macOS Keychain), account,
  networks, network detail, status, devices with filters and search, device
  inspect and rename/pause/block, eeros, reboot, profiles, reservations,
  forwards, guest wifi, speed test, `export --adguard`/`--hosts`, `raw`, and an
  MCP server with 19 tools.

[Unreleased]: https://github.com/laurenschristian/eerox/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/laurenschristian/eerox/releases/tag/v0.1.0
