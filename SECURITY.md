# Security Policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through GitHub Security
Advisories ("Report a vulnerability" on the repository's Security tab) rather
than a public issue. You will get an acknowledgement within a few days.

## What eerox stores

- The eero session token is the only secret. On macOS it is kept in the login
  Keychain (service `eerox`); with `--no-keychain` or on Linux it is written to
  the config file with mode 0600.
- The token grants full account access, exactly like a logged-in app. Treat it
  as a password. `eerox logout` deletes it from the Keychain and the file.
- eerox makes requests only to eero's API host (`api-user.e2ro.com`), or to
  `$EERO_BASE` if you override it for testing. It sends nothing anywhere else.

## Scope

eerox performs only operations an account owner can perform in the eero app. It
does not exploit the router or bypass ownership. The eero API is unofficial and
may change or restrict access at any time.
