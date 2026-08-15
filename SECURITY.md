# Security Policy

GWS Connector handles OAuth credentials for Gmail, Calendar, and Drive. Security
is a core design goal, and this document explains the security model and how to
report vulnerabilities.

## Security model

- **Bring-your-own credentials.** You create the OAuth client in your own Google
  Cloud project and provide the Client ID and Client Secret. There is no shared
  or third-party OAuth app — Google's consent screen shows *your* app, not ours.
- **No third-party servers.** The connector runs entirely on your machine and
  talks directly to Google's APIs over stdio. No data is proxied, relayed, or
  sent to any server operated by this project.
- **Secrets in the OS keychain.** Client secrets and OAuth tokens are stored in
  the operating system keychain (macOS Keychain, GNOME Keyring, Windows
  Credential Manager). On Linux without a keyring, an encrypted file fallback is
  used with `0600` permissions.
- **Registry contains no secrets.** The account registry
  (`~/.claude/channels/gws/accounts.json`) stores only labels, emails, and
  Client IDs — never secrets.
- **Least data.** Tokens are never logged or included in error messages. Read
  and write tools are annotated with MCP `readOnlyHint` / `destructiveHint` so
  MCP clients can gate or auto-approve them appropriately.

See the [Privacy Policy](https://orieg.github.io/gws-connector/privacy.html) for
the full data-handling description.

## Supported versions

Security fixes are applied to the latest released version. Please upgrade to the
[latest release](https://github.com/orieg/gws-connector/releases/latest) before
reporting an issue.

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately via GitHub's
[private vulnerability reporting](https://github.com/orieg/gws-connector/security/advisories/new)
("Report a vulnerability" under the repository's **Security** tab). If you cannot
use that channel, email the maintainer at **nicolas@brousse.info** with the
subject line `SECURITY: gws-connector`.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce (a proof of concept if possible)
- Affected version(s) and platform

You will receive an acknowledgement, and the maintainer will work with you on a
fix and coordinated disclosure. Please give a reasonable window to release a fix
before any public disclosure.

## Scope

In scope: the connector binary and its handling of credentials, tokens, OAuth
flows, keychain storage, and Google API requests.

Out of scope: vulnerabilities in Google's APIs, the OS keychain implementations,
the MCP client (Claude Code, Gemini CLI, etc.), or issues that require an already
fully-compromised local machine.
