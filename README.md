# eerox

CLI and MCP server for [eero](https://eero.com) mesh networks. One static binary, talks to the same cloud API the eero app uses, no eero Plus needed.

```
eerox login me@example.com     # code arrives by email or SMS, token goes to the macOS Keychain
eerox status                   # Home: internet connected, mesh connected, 3 eeros, 41/58 devices online
eerox devices                  # every client: id, name, ip, mac, state, link, node, profile, last seen
eerox devices --online --wireless
eerox devices printer          # substring search
eerox device 10.0.4.212        # inspect by id, ip, mac, nickname or hostname
eerox device rename 10.0.4.212 "Office printer"
eerox device pause "Kids iPad" | unpause | block | unblock
eerox eeros                    # nodes: model, ip, gateway/extender, mesh quality, clients, firmware
eerox network                  # WAN ip, ISP, DHCP, DNS, health, last speed test, firmware
eerox speedtest                # run one from the gateway
eerox profiles                 # family profiles
eerox profile pause Kids
eerox reservations             # DHCP reservations; add <ip> <mac> [desc]; rm <id|ip|mac>
eerox forwards                 # port forwards
eerox guest --on --password s3cret
eerox reboot Office --yes      # one node; no argument reboots the network
eerox export --adguard         # JSON name + ip + mac + tag for AdGuard Home clients
eerox export --hosts           # ip<TAB>hostname lines
eerox raw networks/123/eeros   # any API path
eerox mcp                      # MCP server over stdio
```

Add `--json` to any command for machine-readable output.

## Install

```
go install github.com/laurenschristian/eerox@latest
```

Or grab a binary from [Releases](https://github.com/laurenschristian/eerox/releases).

## Login

eero has no API keys. Login is the app flow: identifier, then a one-time code.

```
eerox login me@example.com
```

The session token is stored in the macOS Keychain (service `eerox`) or, with `--no-keychain` or on Linux, in the config file with mode 0600 (`~/.config/eerox/config.yaml`, `~/Library/Application Support/eerox/config.yaml`, or `$EEROX_CONFIG`). Sessions refresh themselves; when eero invalidates one you get `run eerox login`.

Env: `EERO_TOKEN`, `EERO_NETWORK`, `EEROX_CONFIG`. Config keys: `login`, `network`, `token`, `token_cmd` (a shell command that prints the token, e.g. `op read ...`), `keychain`.

## MCP

`eerox mcp` exposes 19 tools: `eero_account`, `eero_network`, `eero_devices`, `eero_device`, `eero_rename_device`, `eero_pause_device`, `eero_block_device`, `eero_eeros`, `eero_reboot`, `eero_profiles`, `eero_pause_profile`, `eero_reservations`, `eero_add_reservation`, `eero_delete_reservation`, `eero_forwards`, `eero_guest_network`, `eero_set_guest_network`, `eero_speedtest`, `eero_raw`.

```
claude mcp add eero -- eerox mcp
```

Any stdio MCP client works: command `eerox`, args `["mcp"]`.

## With AdGuard Home

`eerox export --adguard | adgctl client import` names every device in [adgctl](https://github.com/laurenschristian/adgctl) so the DNS query log shows "Kids iPad" instead of 10.0.4.212.

## Development

```
make hooks    # gofmt, dash check, gitleaks, build, lint on commit; tests + coverage floor on push
make test
make cover    # floor in scripts/coverage.sh
make lint     # golangci-lint, config in .golangci.yml
make sec      # gosec
```

## Notes

- The API is unofficial (reverse engineered from the app, see 343max/eero-client). Field names follow the API verbatim.
- Amazon-account logins are not supported by this flow; use the email or phone on the eero account.
- `reboot` needs `--yes`. The MCP tool asks the agent to confirm with you first.

MIT.
