# Configuration

imghost is configured entirely through one TOML file. There is no environment-variable override and no `--config` flag: both the daemon (`imghostd`) and the companion CLI (`imghost`, which wraps `version` and local `service` management) read the exact same file so their views of the system cannot diverge.

## File location

The path is resolved via [`github.com/adrg/xdg`](https://github.com/adrg/xdg) as `xdg.ConfigFile("imghost/config.toml")`:

| OS      | Default path                                             |
| ------- | -------------------------------------------------------- |
| Linux   | `$XDG_CONFIG_HOME/imghost/config.toml` (default `~/.config/imghost/config.toml`) |
| macOS   | `~/Library/Application Support/imghost/config.toml`      |
| Windows | `%APPDATA%\imghost\config.toml`                          |

Set `XDG_CONFIG_HOME` to redirect the config location (e.g. for tests).

## Schema

| Field             | Type           | Default    | Notes                                                                                 |
| ----------------- | -------------- | ---------- | ------------------------------------------------------------------------------------- |
| `listen_addr`     | string         | `":34286"` | Passed directly to `http.Server.Addr`.                                                |
| `api_key`         | string         | `"change-me"` | Bearer token required for all writes.                                              |
| `default_access`  | `"public"`/`"private"` | `"public"` | Fallback access when neither the path nor any ancestor has an explicit rule. |
| `state_dir`       | string         | `""`       | Empty selects the XDG state default. When set, must be absolute after `~` expansion. |
| `[[root]]`        | array of table | —          | At least one required.                                                                |
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

`state_dir` is where bbolt stores the permission database (`imghost.db`).

- Empty → `xdg.StateFile("imghost/imghost.db")`:
  - Linux: `~/.local/state/imghost/imghost.db`
  - macOS: `~/Library/Application Support/imghost/imghost.db`
  - Windows: `%LOCALAPPDATA%\imghost\imghost.db`
- Non-empty → must be absolute; bbolt file is `<state_dir>/imghost.db`.

`~` at the start of `state_dir` or `root.path` is expanded to `os.UserHomeDir()`.

## Start-up validation (fail-fast)

`imghostd` refuses to start if any of the following hold; the process exits non-zero with an explanatory stderr line:

1. Config file missing / unreadable / contains unknown keys / TOML parse error.
2. `len(roots) == 0`.
3. Duplicate `root.name`.
4. `root.name` empty, contains `/`, equals `.` / `..`, or conflicts with a reserved prefix.
5. `root.path` not absolute after `~` expansion, does not exist, or is not a directory.
6. `default_access` or `root.access` is not `public` or `private`.
7. `state_dir` non-empty but not absolute after `~` expansion.
8. bbolt DB cannot be opened within the 5 s timeout (stale lock).

See [`docs/examples/config.toml`](examples/config.toml) for a working template.
