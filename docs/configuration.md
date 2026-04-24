# Configuration

filehub is configured entirely through one TOML file. There is no environment-variable override and no `--config` flag: both the daemon (`filehubd`) and the companion CLI (`filehub`, which wraps `version` and local `service` management) read the exact same file so their views of the system cannot diverge.

## File location

The path is resolved via [`github.com/adrg/xdg`](https://github.com/adrg/xdg) as `xdg.ConfigFile("filehub/config.toml")`:

| OS      | Default path                                             |
| ------- | -------------------------------------------------------- |
| Linux   | `$XDG_CONFIG_HOME/filehub/config.toml` (default `~/.config/filehub/config.toml`) |
| macOS   | `~/Library/Application Support/filehub/config.toml`      |
| Windows | `%APPDATA%\filehub\config.toml`                          |

Set `XDG_CONFIG_HOME` to redirect the config location (e.g. for tests).

## First-run bootstrap

When this file does not exist, `filehubd` (and the `filehub` CLI when it invokes `config.Load`) generates a random 256-bit `api_key` via `crypto/rand`, then writes a minimal `config.toml` at the path above with `0600` permissions. The generated file contains `listen_addr = "127.0.0.1:34286"` and the fresh `api_key`. Start-up continues with those values; subsequent runs read the file like any other. Rotate the key by editing `api_key`; delete the file to force a fresh bootstrap.

## Schema

| Field             | Type           | Default    | Notes                                                                                 |
| ----------------- | -------------- | ---------- | ------------------------------------------------------------------------------------- |
| `listen_addr`     | string         | `"127.0.0.1:34286"` | Passed directly to `http.Server.Addr`. Loopback-only by default; edit explicitly to bind all interfaces. |
| `api_key`         | string         | — (auto-generated on first run) | Bearer token required for all writes. Must be non-empty when the config file exists. |
| `default_access`  | `"public"`/`"private"` | `"public"` | Fallback access when neither the path nor any ancestor has an explicit rule. |
| `state_dir`       | string         | `""`       | Empty selects the XDG state default. When set, must be absolute after `~` expansion. |
| `[[root]]`        | array of table | —          | Optional. When omitted, the daemon injects a public `_default` root at `xdg.DataFile("filehub/data")`. |
| `root.name`       | string         | —          | URL first segment. Must be non-empty, not contain `/`, not equal `.`/`..`, and not clash with a reserved prefix. |
| `root.path`       | string         | —          | Absolute after `~` expansion, must exist as a directory.                              |
| `root.access`     | `"public"`/`"private"` | inherit `default_access` | Optional per-root override. |

### Reserved names

The following first-segment names are owned by the system and may not be used as a `root.name`: `swagger`.

## URL routing

With the example config below:

```toml
[[root]]
name = "photos"
path = "/mnt/nas/photos"
```

`GET /photos/trips/paris/eiffel.jpg` maps to `/mnt/nas/photos/trips/paris/eiffel.jpg`. Paths that cannot match any root return 404; reserved first segments are routed to their dedicated handlers (`/swagger`).

## State directory

`state_dir` is where bbolt stores the permission database (`filehub.db`).

- Empty → `xdg.StateFile("filehub/filehub.db")`:
  - Linux: `~/.local/state/filehub/filehub.db`
  - macOS: `~/Library/Application Support/filehub/filehub.db`
  - Windows: `%LOCALAPPDATA%\filehub\filehub.db`
- Non-empty → must be absolute; bbolt file is `<state_dir>/filehub.db`.

`~` at the start of `state_dir` or `root.path` is expanded to `os.UserHomeDir()`.

## Start-up validation (fail-fast)

`filehubd` refuses to start if any of the following hold; the process exits non-zero with an explanatory stderr line:

1. Config file unreadable / contains unknown keys / TOML parse error. (A missing file is not an error: it triggers first-run bootstrap above.)
2. `api_key` present in the file but empty.
3. Duplicate `root.name`.
4. `root.name` empty, contains `/`, equals `.` / `..`, or conflicts with a reserved prefix.
5. `root.path` not absolute after `~` expansion, does not exist, or is not a directory.
6. `default_access` or `root.access` is not `public` or `private`.
7. `state_dir` non-empty but not absolute after `~` expansion.
8. bbolt DB cannot be opened within the 5 s timeout (stale lock).

Zero `[[root]]` entries is **not** fatal; see the architecture overview's default-root-injection note.

See [`docs/examples/config.toml`](examples/config.toml) for a working template.
