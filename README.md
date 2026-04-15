# imghost

A minimal self-hosted file hosting server. Declare one or more on-disk roots in `config.toml` and imghost serves them over HTTP with a thin permission layer.

## Quick install

Linux / macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/1996fanrui/imghost/main/install.sh | bash
```

Windows (PowerShell):

```powershell
iwr https://raw.githubusercontent.com/1996fanrui/imghost/main/install.ps1 -UseBasicParsing | iex
```

See [`docs/installation.md`](docs/installation.md) for details.

## Usage

Every URL path starts with the root name declared in `config.toml`. For example, given the config above, `PUT /photos/cat.jpg` stores the file at `/mnt/nas/photos/cat.jpg`.

Write operations (`PUT`, `DELETE`) require `Authorization: Bearer <api_key>`. Read access depends on the object's ACL (defaults to `default_access`). Use the bare `?acl` query to manage per-object ACL — `public` or `private`. ACL rules apply at file, directory, or global granularity and are resolved most-specific-first; see [`docs/permissions.md`](docs/permissions.md) for the full model.

On first run, `imghostd` auto-generates a random `api_key` and writes it (0600) to the XDG config path (`~/.config/imghost/config.toml` on Linux; see [`docs/configuration.md`](docs/configuration.md) for macOS and Windows). Use that value as `<API_KEY>` below; the daemon listens on `127.0.0.1:34286` by default, so hit it from the same host.

```bash
TOKEN="Authorization: Bearer <API_KEY>"
BASE=http://localhost:34286

# Upload
curl -X PUT -H "$TOKEN" --data-binary @cat.jpg "$BASE/photos/cat.jpg"

# Download
curl "$BASE/photos/cat.jpg" -o cat.jpg

# Delete
curl -X DELETE -H "$TOKEN" "$BASE/photos/cat.jpg"

# Set ACL to private
curl -X PUT -H "$TOKEN" -H 'Content-Type: application/json' \
     -d '{"access":"private"}' "$BASE/photos/cat.jpg?acl"

# Inspect ACL
curl -H "$TOKEN" "$BASE/photos/cat.jpg?acl"

# Delete ACL (falls back to default_access)
curl -X DELETE -H "$TOKEN" "$BASE/photos/cat.jpg?acl"
```

## CLI

The `imghost` CLI is a thin helper for managing the local `imghostd` service. It reads the same `config.toml` as the daemon (and bootstraps one on first run just like `imghostd`), so `imghost` commands (other than `version`) fail fast when the config is unreadable or invalid.

```bash
go build -o imghost ./cmd/imghost

imghost version                     # print version, commit, build date, Go version
imghost service start               # start the imghostd background service
imghost service stop                # stop it
imghost service status              # show status
imghost service logs                # tail service logs (follow mode)
```

On Linux, `service` subcommands wrap `systemctl --user` and `journalctl --user-unit imghostd`. On macOS, they wrap `launchctl bootstrap|bootout|print` and `log show`. When the systemd user unit or launchd agent is missing, the CLI prints a platform-specific guidance message and exits non-zero. On Windows the CLI has no native service integration and every `imghost service` subcommand prints a guidance line and exits 0 — run `imghostd` directly or configure a Task Scheduler job.

## Documentation

- Configuration: [`docs/configuration.md`](docs/configuration.md)
- Architecture: [`docs/architecture_overview.md`](docs/architecture_overview.md)
- Permission model: [`docs/permissions.md`](docs/permissions.md)
- Installation: [`docs/installation.md`](docs/installation.md)
- Releasing (maintainers): [`docs/releasing.md`](docs/releasing.md)
- API spec: [`docs/swagger.yaml`](docs/swagger.yaml)
