# eerox

[![CI](https://github.com/laurenschristian/eerox/actions/workflows/ci.yml/badge.svg)](https://github.com/laurenschristian/eerox/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/laurenschristian/eerox.svg)](https://pkg.go.dev/github.com/laurenschristian/eerox)
[![Go Report Card](https://goreportcard.com/badge/github.com/laurenschristian/eerox)](https://goreportcard.com/report/github.com/laurenschristian/eerox)
[![Release](https://img.shields.io/github/v/release/laurenschristian/eerox)](https://github.com/laurenschristian/eerox/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fast, single-binary CLI and [MCP](https://modelcontextprotocol.io) server for [eero](https://eero.com) mesh networks. It talks to the same cloud API the eero app uses, so it works on any eero without eero Plus, and needs nothing installed on the router.

```console
$ eerox status
James: internet connected, mesh connected, 5 eeros, 60/91 devices online, wan 146.86.147.119

$ eerox devices --online
ID            NAME                    IP           MAC                STATE   LINK      NODE        LAST SEEN
9c3e5336bf95  Apple TV | Family Room  10.0.4.78    9c:3e:53:36:bf:95  online  wired     Tech Panel  1m
a2777e98c989  MacBook | Ben           10.0.4.20    a2:77:7e:98:c9:89  online  5GHz 5/5  Office      1m
...

$ eerox device pause "Kids iPad"
ok: Kids iPad (b2)
```

## Why

The eero app is the only first-party way to see or change anything on the network. eerox puts the same operations in your terminal and in front of an AI agent: scriptable, greppable, `--json` on every command, and a rename-batch workflow the app has no equivalent for.

## Features

- **Read**: account, networks, per-network health, mesh nodes, every client device with node, band, signal and last-seen.
- **Control**: rename, pause, unpause, block and unblock devices; pause family profiles; reboot a node or the network; run a gateway speed test; manage DHCP reservations; toggle guest wifi.
- **Batch rename**: name many devices at once from a `match<TAB>name` file, matching on id, ip, mac, nickname or hostname, with `--dry-run`.
- **DNS control**: show the network resolvers and pin them, so a single private resolver (e.g. Pi-hole or AdGuard Home) actually enforces filtering.
- **Export**: device inventory as JSON for [AdGuard Home](https://github.com/laurenschristian/adgctl) clients, or as a hosts file.
- **MCP server**: 19 tools over stdio, so Claude or Cursor can answer "who is on the wifi and pause the kids' iPad".
- **Escape hatch**: `eerox raw <path>` calls any API endpoint directly.

## Install

Homebrew:

```console
brew install laurenschristian/tap/eerox
```

Go:

```console
go install github.com/laurenschristian/eerox@latest
```

Script (macOS/Linux):

```console
curl -fsSL https://raw.githubusercontent.com/laurenschristian/eerox/main/scripts/install.sh | sh
```

Or download a binary from [Releases](https://github.com/laurenschristian/eerox/releases). Man pages and shell completions ship in the archive and the Homebrew formula.

## Login

eero has no API keys. Login mirrors the app: an identifier, then a one-time code.

```console
$ eerox login me@example.com
verification code: 123456
logged in as me@example.com, network 3510114, token in macOS Keychain (service eerox)
```

The session token is stored in the macOS Keychain (service `eerox`). With `--no-keychain`, or on Linux, it lives in the config file with mode 0600. Sessions refresh themselves; if eero invalidates one, any command tells you to `eerox login` again. Amazon-account logins are not supported by this flow; use the email or phone on the eero account.

`eerox doctor` checks config, session and reachability in one shot.

## Configuration

Precedence is flags, then environment, then the config file.

| | |
|---|---|
| Env | `EERO_TOKEN`, `EERO_NETWORK`, `EEROX_CONFIG` |
| File | `~/Library/Application Support/eerox/config.yaml` (macOS), `~/.config/eerox/config.yaml` (Linux), or `$EEROX_CONFIG` |
| Keys | `login`, `network`, `token`, `token_cmd` (a command that prints the token, e.g. `op read ...`), `keychain` |

Most people never touch the file: `eerox login` writes it and the network id is discovered and cached on first use. Pass `--network <id>` to target a second network.

## Commands

Run `eerox <command> --help`, or read the generated reference in [`docs/cli`](docs/cli) (also installed as man pages). Highlights:

```console
eerox status                         # one-line health
eerox devices [search] [--online|--offline|--wired|--wireless|--guest|--paused] [--profile NAME]
eerox device <id|ip|mac|name>        # full detail for one device
eerox device rename <device> <name>  # pause | unpause | block | unblock
eerox rename-batch names.txt         # bulk rename from a file (--dry-run)
eerox eeros                          # mesh nodes and health
eerox network                        # WAN, ISP, DHCP, DNS, firmware, last speed test
eerox dns [--set 10.0.4.70 --yes]    # show or pin the network resolvers
eerox speedtest                      # run one from the gateway
eerox profiles ; eerox profile pause Kids
eerox reservations [add <ip> <mac> [desc] | rm <id|ip|mac>]
eerox forwards ; eerox guest [--on|--off|--password ... |--name ...]
eerox reboot [<eero>] --yes          # one node, or the whole network
eerox export --adguard | --hosts     # inventory for other tools
eerox raw networks/<id>/eeros        # any API path
eerox mcp                            # MCP server over stdio
```

## MCP

```console
claude mcp add eero -- eerox mcp
```

Any stdio MCP client works: command `eerox`, args `["mcp"]`. It exposes 19 tools (`eero_status`-style reads plus device, profile, reservation, guest and reboot writes). Reboot asks the agent to confirm with you first.

## Pairs with AdGuard Home

```console
eerox export --adguard | adgctl client import --update
```

This names every AdGuard Home client from eero, so the DNS query log reads "Kids iPad" instead of `10.0.4.212`. See [adgctl](https://github.com/laurenschristian/adgctl).

## Development

```console
make hooks     # install git hooks: gofmt, dash check, gitleaks, build, lint on commit; tests + coverage floor on push
make test      # go test -race -shuffle=on
make cover     # coverage with the floor in scripts/coverage.sh
make lint      # golangci-lint (.golangci.yml)
make sec       # gosec
make docs      # regenerate man/ and docs/cli/
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Notes and limits

- The eero API is unofficial, reverse engineered from the app (credit: [343max/eero-client](https://github.com/343max/eero-client)). Field names follow the API verbatim. It can change without notice.
- eerox is not affiliated with or endorsed by eero LLC or Amazon.
- Write operations you can do in the app; nothing here bypasses account ownership.

## License

MIT. See [LICENSE](LICENSE).
