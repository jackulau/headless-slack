# slk — headless Slack from your terminal

`slk` is a CLI/TUI client for Slack. It reads channels, sends messages, and
streams realtime events without the official desktop or web app.

It connects to **real Slack** by reverse-engineering the same surface the web
app uses: a workspace user token (`xoxc-…`) plus the session cookie
(`xoxd-…`), both extracted automatically from your local Chrome profile.

> Inspired by the headless-Slack-for-agents architecture of
> [`AgentWorkforce/relaycast`][relaycast], but where relaycast is a Slack-shaped
> *server*, `slk` is a Slack *client* — so the same human can drive their real
> workspace from a shell.

[relaycast]: https://github.com/AgentWorkforce/relaycast

## What works

- `slk login` — pulls `xoxc` + `xoxd` from Chrome, stores them in the macOS keychain
- `slk channels` — list channels, DMs, private groups
- `slk read <channel> [-n 50]` — recent history
- `slk send <channel> <message…>` — post a message
- `slk dm <user> <message…>` — DM a user by name
- `slk watch [channel]` — stream realtime events
- `slk users` — list workspace members
- `slk search <query>` — server-side search + local cache fallback
- `slk tui` — Bubbletea TUI: sidebar of channels + message pane + input box

State is cached locally in SQLite (`~/Library/Application Support/slk/cache.sqlite`).

## Installation

Requires **Go 1.24+** and **macOS** (Chrome cookie decryption is macOS-only in
v1; Linux/Windows planned).

```
git clone <this repo>
cd Headless-Slack
make build       # produces ./slk
make install     # installs to $(go env GOBIN)
```

## First-time setup

1. Sign into Slack in **Google Chrome** (or Chrome Canary, or Brave). Do *not*
   use Firefox or Safari — the cookie decryption path reads Chromium's
   "Safe Storage" key from the macOS Keychain.

2. Close Chrome (it holds an exclusive lock on the cookie DB).

3. Run login:

   ```
   ./slk login
   ```

   You'll be prompted for the workspace URL/subdomain. The keychain will pop a
   prompt the first time — click *Always Allow*. The xoxc token is then
   fetched by HTTP-GETting your workspace homepage and parsing `boot_data` from
   the returned HTML.

4. Smoke-test:

   ```
   ./slk --team myco channels
   ```

   You'll see your channel list. After that, `--team` can be omitted; set
   `SLACK_TEAM=myco` once in your shell to pin a default.

## How it works

| Layer | What | File |
|---|---|---|
| Auth | Extract `xoxd` from Chrome cookies SQLite (AES-128-CBC v10 scheme, key from macOS Keychain), then GET workspace HTML and regex out `boot_data.api_token` for `xoxc`. | `internal/auth/` |
| Transport | `*http.Client` with a [utls](https://github.com/refraction-networking/utls) Chrome ClientHello so the JA3 fingerprint matches a real desktop Chrome (Slack's Anomaly Event Response system flags mismatched TLS/UA pairs). | `internal/api/transport.go` |
| Rate limiting | Token-bucket per Slack tier (Tier 2 `conversations.list`, Tier 3 `conversations.history`, special tier `chat.postMessage`). | `internal/api/ratelimit.go` |
| Web API | Typed methods over POST form-encoded calls to `https://<team>.slack.com/api/<method>`. Returns `*SlackError` for `ok:false`. | `internal/api/` |
| Realtime | `rtm.connect` → WSS WebSocket on `wss-primary.slack.com`. Despite official RTM deprecation for new apps, `xoxc` tokens still successfully open this socket in 2026 — this is what wee-slack does. | `internal/rtm/` |
| Storage | SQLite WAL with channels / users / messages / cursors tables. Cached locally so repeat reads are fast and offline-tolerant. | `internal/store/` |
| TUI | Bubbletea + bubbles (sidebar list, message viewport, text input). | `internal/tui/` |

See [`docs/TOKENS.md`](docs/TOKENS.md) for the gory details of token formats
and the boot-HTML extraction flow.

## Risks and limitations

- **This is not a sanctioned Slack client.** Workspace admins can see anomaly
  alerts on inconsistent fingerprint / unusual IP / excessive API calls.
  Workspace policy may prohibit non-official clients — check yours.
- The `xoxd` cookie's lifetime was shortened from 10 years to ~1 year in
  2024–2025; re-run `slk login` when you hit `invalid_auth`.
- File upload, voice/huddles, canvases, and Enterprise Grid cross-workspace
  search are **not implemented**.
- Chrome v127+ introduced "App-Bound Encryption" (v20 cookies). v1 only
  handles the v10 scheme used by the master cookie used for Slack auth; if
  decryption fails, paste the cookie value manually when prompted.
- Slack's `conversations.history` rate limit cut for non-Marketplace bot apps
  (May 2025) does **not** apply to user tokens; `slk` retains 50+/min, 1000
  objects per page.

## Verification

Everything is offline-testable via a local mock Slack server:

```
make test        # 35 tests across 9 packages
```

The integration test in `integration_test.go` drives the mock through the full
list → read → send → realtime path; the RTM loop is exercised against a
real-but-local WebSocket server.

## Disclaimer

`slk` uses undocumented Slack endpoints. They may change at any time, and use
may violate your workspace's Terms of Service. This is a research and personal
productivity tool — not for evading workspace admins or for any large-scale
automation. Be a good citizen.
