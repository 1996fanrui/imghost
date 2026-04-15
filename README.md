# imghost

A minimal self-hosted file hosting server. Declare one or more on-disk roots in `config.toml` and imghost serves them over HTTP with a thin permission layer.

## Deployment

1. Download or build the `imghostd` binary (pure Go, no CGO):

   ```bash
   go build -o imghostd ./cmd/imghostd
   ```

2. Write `config.toml` to the XDG config path (`~/.config/imghost/config.toml` on Linux):

   ```toml
   listen_addr    = ":34286"
   api_key        = "<API_KEY>"
   default_access = "public"

   [[root]]
   name = "photos"
   path = "/mnt/nas/photos"
   ```

   See [`docs/examples/config.toml`](docs/examples/config.toml) and [`docs/configuration.md`](docs/configuration.md) for the full schema.

3. Run the daemon:

   ```bash
   ./imghostd
   ```

   The server listens on `http://<host>:34286` and fails fast if any config field is invalid (missing roots, non-directory paths, stale bbolt lock, etc.). API reference at `http://<host>:34286/swagger/index.html`.

## Usage

Every URL path starts with the root name declared in `config.toml`. For example, given the config above, `PUT /photos/cat.jpg` stores the file at `/mnt/nas/photos/cat.jpg`.

Write operations (`PUT`, `DELETE`) require `Authorization: Bearer <api_key>`. Read access depends on the object's ACL (defaults to `default_access`). Use the bare `?acl` query to manage per-object ACL — `public` or `private`. ACL rules apply at file, directory, or global granularity and are resolved most-specific-first; see [`docs/permissions.md`](docs/permissions.md) for the full model.

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

The `imghost` CLI is a thin helper for managing the local `imghostd` service. It reads the same `config.toml` as the daemon, so `imghost` commands (other than `version`) fail fast when the config is missing or invalid.

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
- API spec: [`docs/swagger.yaml`](docs/swagger.yaml)
