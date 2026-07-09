## supabase-login

Authenticate the CLI with your Volcengine account.

This page covers every way to authenticate, and how `login` behaves in interactive terminals versus non-interactive environments (CI, AI agents, hosted sandboxes such as Coze).

### Which method should I use?

| Your situation | Use |
|---|---|
| Local machine with a desktop browser | `login` |
| Headless machine / remote shell / AI agent, but a human can open a browser somewhere | `login --remote` |
| CI or fully automated pipelines | AK/SK keys (no OAuth) |
| Credentials already obtained on another machine | `login --credential-file` |

### 1. Browser login

```bash
byted-supabase-cli login
```

Opens your local browser, runs OAuth 2.0 with PKCE, and receives the authorization code automatically via a localhost callback. Nothing to copy or paste.

Not usable in headless or agent environments — there is no browser, and the localhost callback is unreachable. Use method 2 or 3 instead.

### 2. Cross-device login

```bash
byted-supabase-cli login --remote
```

Prints an authorization URL. You open it in a browser **on any device**, sign in with your Volcengine account, and receive an authorization code. How the code gets back to the CLI depends on whether the CLI is running interactively:

| Scenario | Behavior |
|---|---|
| Interactive terminal (a human at the keyboard) | The CLI prompts `Authorization code:` and blocks on stdin — paste the code and you are done. |
| Non-interactive / agent | The CLI creates a temp file (`byted-supabase-login-*.txt`, mode 0600), prints `Waiting for authorization code — write it to: <path>`, and polls that file. Once a valid code is written there, login completes and the temp file is deleted. |

No extra flag is needed: the CLI picks the right behavior automatically (see "Interactivity detection" below).

File polling details (non-interactive mode):

- **What to write**: the base64 string shown by the browser (it encodes `code=<authorization code>&state=<state>`) — exactly what you would paste into the interactive prompt.
- **Base64 tolerance**: standard, raw (unpadded), URL-safe, and raw URL-safe encodings are all accepted.
- **CSRF protection**: the decoded `state` must match the one embedded in the authorization URL, otherwise login fails.
- **Polling**: the file is read every second, with a 10-minute overall timeout.
- **Fast failure**: if the file content stays identical for two consecutive reads but still cannot be decoded, the CLI returns a format error immediately instead of waiting for the timeout.
- On success the CLI stores temporary STS credentials in the profile cache and reuses them until they expire.

### 3. Access keys (no OAuth)

For CI and fully automated pipelines, skip `login` entirely:

```bash
export VOLCENGINE_ACCESS_KEY=...
export VOLCENGINE_SECRET_KEY=...
export VOLCENGINE_REGION=cn-beijing
```

Or persist the keys into a profile:

```bash
byted-supabase-cli configure set --access-key ... --secret-key ... [--region ...]
```

No interaction of any kind. Environments that already carry role credentials (e.g. VeFaaS IAM sandboxes) need no explicit setup.

### 4. Import a credential cache

```bash
byted-supabase-cli login --credential-file <path>
```

Imports a credential cache file produced by a successful `login` elsewhere into the current profile. Cannot be combined with `--remote`.

### Interactivity detection

The CLI treats a session as **non-interactive** when either:

- stdin is not a TTY (equivalent to `test -t 0` failing), or
- `--agent yes` is set, or a known agent environment variable is detected.

Only stdin matters because the question is "can we show a prompt **and read** the answer" — reading happens on stdin. Where stdout goes is irrelevant: in `login | tee log`, stdout is a pipe but stdin is still the keyboard, so the session is still interactive.

The `--agent` flag overrides detection: `auto` (default, detects known agents via environment variables), `yes` (force non-interactive), `no` (force interactive).

### Behavior common to all methods

- **Region**:
  - `--region` given → validated and used;
  - omitted, interactive → prompts `Please enter region [cn-beijing]:` (Enter accepts the default);
  - omitted, non-interactive → uses the default silently and prints `Using default region: cn-beijing`.
- **Confirmations** (e.g. replacing credentials from an earlier sign-in): `--yes` auto-confirms. In non-interactive mode without `--yes`, the CLI fails fast with `confirmation required in non-interactive or agent mode; re-run with --yes` instead of blocking.

### Recipe: hosted AI agent environments (Coze, etc.)

In hosted terminals stdin is not a TTY, so a bare `login --remote` automatically uses non-interactive file polling — no flags needed:

```bash
# 1. Run in the background (creates the polling file, prints its path, does not block)
byted-supabase-cli login --remote > /tmp/login.out 2>&1 &
sleep 2; cat /tmp/login.out   # grab the authorization URL and "write it to: <path>"

# 2. Send the URL to the user. They sign in with their own Volcengine account
#    in their own browser, then the code goes into the polling file:
echo "<authorization string>" > <path printed above>
```

Notes:

- **Tenant isolation**: whoever's Volcengine account signs in is whose Supabase resources the CLI operates on — each organization uses its own account, so resources stay isolated.
- Keep the background `login` process alive for the whole flow; the user may take several minutes to complete the browser step.
- If stdin happens to be a TTY in your environment, force non-interactive mode with `--agent yes`.
