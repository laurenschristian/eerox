# Changelog

All notable changes are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
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
