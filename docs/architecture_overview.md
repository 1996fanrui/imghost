# Architecture Overview

## System Architecture

`imghost` is a single-binary self-hosted file hosting service for blog/content creators. It exposes any mounted local directory over HTTP with a thin permission layer on top.

The process is one Go binary with no external runtime dependencies. Its in-process components are:

- **HTTP layer** — routes three URL shapes: `GET /swagger/*` (API docs), `/<path>?acl` (permission subresource), and `/<path>` (file operations). Routing is done by a chi router; a catch-all handler dispatches between file and ACL handlers based on query inspection.
- **Auth** — a single `API_KEY` bearer token checked with constant-time comparison. Private-file GET is the only conditional auth path; everything else (all writes, all ACL) unconditionally requires auth.
- **Path safety** — every request path is decoded, segment-checked for `..`, canonicalized, joined under `DATA_DIR`, and checked against symlink escapes before any filesystem syscall.
- **File storage** — direct filesystem I/O under `DATA_DIR`. Uploads are streamed into a same-directory `*.tmp` file, fsynced, then atomically renamed. Writes are fail-fast: any failure removes the temp file and rolls back any permstore change.
- **Permission store** — a single bbolt database at a fixed path (not configurable). One bucket (`permissions`) maps normalized paths to `public` or `private`.
- **Permission resolver** — when deciding effective access for a GET, walks the path upward one segment at a time, returning the first explicit rule found, falling back to the global default.

All components run in the same process; no message queue, no sidecar, no external DB.

## Core Functionality and Usage

**Primary use case**: a creator mounts one or more host directories via Docker volumes under `DATA_DIR`, runs the container behind a reverse proxy (Caddy/Nginx/Traefik for TLS), and serves files via public URLs. Private files require the API key.

Typical flows:

1. **Serve**: `GET /photos/a.jpg` — resolver decides public or private; public is served directly, private requires bearer.
2. **Upload**: `PUT /photos/a.jpg` with bearer and file body. Optional `X-Access: public|private` header writes an explicit ACL. Response is `201 Created` with `{"path": "/photos/a.jpg"}` — no host, no domain (the service doesn't know its own public URL).
3. **Delete**: `DELETE /photos/a.jpg` with bearer. Directory targets return 403 (directory deletion is out of scope).
4. **Manage ACL**: `GET|PUT|DELETE /photos/a.jpg?acl` manages the explicit rule for that path. `?acl` must be a bare query key (no value, no other keys) — strict to avoid ambiguity with the file handler.
5. **Discover API**: `GET /swagger/index.html` serves the auto-generated interactive API docs.

## Key Technical Constraints and External Dependencies

- **Single binary, no CGO** — pure Go so the binary can be cross-compiled and shipped in an Alpine image. `CGO_ENABLED=0` enforced in Dockerfile.
- **Image size ≤ 30 MB** — requirement-driven (current ~26.7 MB). `-ldflags="-s -w"` strips symbol/debug info. Alpine base chosen over scratch to keep basic `sh`/`ls` for container debugging.
- **No external services** — no database, no cache, no message broker. bbolt is an embedded single-file KV store, mmap'd so hot keys are effectively in memory.
- **TLS is out of scope** — the service speaks plaintext HTTP only. TLS termination is a deployment-layer concern (reverse proxy).
- **No domain awareness** — response bodies never contain absolute URLs. Callers compose their own public URL.
- **Fixed bbolt path** (`/var/lib/imghost/imghost.db`) — a deliberate separation from `DATA_DIR` so the two cannot be confused or mixed. Startup config validates the separation.
- **Graceful shutdown** — SIGINT/SIGTERM triggers a 30s shutdown window before forcing close; bbolt is closed after HTTP drains.
- **No write/read timeout** on `http.Server` — streaming uploads/downloads must not be timed out by the server. Flow control is the reverse proxy's job.

## Important Design Decisions

**S3-style `?acl` subresource.** Files and their ACLs share the same URL path; the `?acl` bare query key switches semantics. This matches S3 convention, keeps the API surface small (one path, two meanings), and requires only 3 real HTTP endpoints. The tradeoff: swagger docs can't express the bare-query subresource cleanly in OpenAPI 2.0, so ACL docs appear under a synthetic `/{path}/acl` alias with a description noting the real URL form.

**Permission inheritance vs explicit-only.** Permissions inherit up the tree: setting `private` on `/docs` makes every descendant default to private. This supports the natural directory-as-namespace mental model for blog content and avoids forcing callers to set a rule per file. The resolver walks up at GET time (bbolt reads are mmap-fast, so this is acceptable without caching).

**Atomic write with same-directory temp file.** `os.CreateTemp(targetDir, "*.tmp")` + fsync + rename. The "same directory" constraint is load-bearing: `/tmp`-based temp files would cause `rename` to fall back to copy-then-delete across filesystems, losing atomicity. Rollback of `X-Access`-driven permstore writes is paired with rename success/failure so the two stores never diverge in a synchronous failure. Crash-window inconsistency (SIGKILL between permstore write and rename) is explicitly not handled — documented as a known MVP limit.

**Reserved paths and 405 for write methods.** `/swagger/*` is reserved in a single source-of-truth list. The chi router matches `/swagger/*` before the catch-all, so `PUT /swagger/foo` is handled by the swagger route and returns 405 Method Not Allowed (RFC 9110 §15.5.6) rather than being treated as a file write. This is the semantically correct behavior: the request is syntactically valid but the method isn't supported on that resource. Any future reserved route must be added to the same central list and routed with the same pattern.

**Reject directory targets.** `GET`/`DELETE` on a directory return 403. Directory listing is out of scope (the service is a file host, not a file browser); cascading directory delete is also out of scope to prevent accidental mass deletion.

**Path traversal defense in depth.** Four layers: (1) segment-level `..` rejection after URL decode (before any normalization, so `%2e%2e` can't slip through), (2) `filepath.Rel` check after join, (3) `EvalSymlinks` check for both existing targets and nearest existing ancestor on new-file paths, (4) all four steps are shared between file and ACL handlers via a single resolver function. Any one layer alone would be enough against most attacks; all four together make bypass require a filesystem-level race that the OS itself would have to allow.

**Single API key, not per-user tokens.** The project is personal-scale (blog/content creator). Multi-tenant auth would triple the complexity for no target-user benefit. The single bearer is the MVP; multi-user auth is deferred unless a concrete need arrives.
