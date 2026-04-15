# imghost

A minimal self-hosted file hosting server. Drop it in front of any directory, and it instantly serves your files over HTTP with a clean URL.

## Deployment

Deploy with Docker Compose:

```bash
API_KEY=<API_KEY> docker compose up -d --build
```

The server listens on `http://<host>:34286`. All files served by imghost live under the container's `/data` directory (mapped to `./data` on the host by default). The permission DB is stored under `~/.local/state/imghost/`.

### Mounting extra host directories

Edit `docker-compose.yml` and add one line per directory under `volumes:`:

```yaml
    volumes:
      - ./data:/data
      - /mnt/nas/photos:/data/photos
      - /home/user/docs:/data/docs
```

Then `docker compose up -d`.

### Environment variables

| Variable          | Default      | Description                                             |
| ----------------- | ------------ | ------------------------------------------------------- |
| `API_KEY`         | `change-me`  | Bearer token required for write APIs.                   |
| `DEFAULT_ACCESS`  | `public`     | Default access for new objects: `public` / `private`.   |
| `PUID` / `PGID`   | `1000`       | uid:gid the server drops to after startup.              |

## Usage

Every URL path maps 1:1 to a file under `/data`. For example, `PUT /photos/cat.jpg` stores the file at `/data/photos/cat.jpg` inside the container.

Write operations (`PUT`, `DELETE`) require `Authorization: Bearer <API_KEY>`. Read access depends on the object's ACL (defaults to `DEFAULT_ACCESS`). Use the bare `?acl` query to manage per-object ACL — `public` or `private`. ACL rules apply at file, directory, or global granularity and are resolved most-specific-first; see [`docs/permissions.md`](docs/permissions.md) for the full model.

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

# Delete ACL (falls back to DEFAULT_ACCESS)
curl -X DELETE -H "$TOKEN" "$BASE/photos/cat.jpg?acl"
```

See the full API reference at `http://localhost:34286/swagger/`.

### CLI wrapper

For a friendlier command surface, a bash wrapper is bundled at `scripts/imghost`:

```bash
export IMGHOST_URL=http://localhost:34286
export IMGHOST_API_KEY=<API_KEY>

scripts/imghost put /photos/cat.jpg ./cat.jpg
scripts/imghost get /photos/cat.jpg -o cat.jpg
scripts/imghost rm  /photos/cat.jpg
scripts/imghost acl set /photos/cat.jpg private
scripts/imghost acl get /photos/cat.jpg
scripts/imghost acl rm  /photos/cat.jpg
```

Copy or symlink it somewhere on `PATH` (e.g. `/usr/local/bin/imghost`) for global use.

## Documentation

- Architecture: [`docs/architecture_overview.md`](docs/architecture_overview.md)
- Permission model: [`docs/permissions.md`](docs/permissions.md)
- API spec: [`docs/swagger.yaml`](docs/swagger.yaml)
