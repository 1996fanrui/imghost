# Architecture Overview

## System Architecture

imghost is a self-hosted file hosting service shipped as two native Go binaries:

- **`imghostd`** — the HTTP daemon. Long-running process. Serves file I/O and ACL management.
- **`imghost`** — the user-facing CLI (cobra-based). Currently exposes `version` and `service {start|stop|status|logs}`. The `service` subcommands are thin wrappers over the platform's native service manager; they do not talk to the daemon over the wire.

Both binaries read the exact same `config.toml` resolved through XDG (`xdg.ConfigFile("imghost/config.toml")`), so their views of roots, API key, and state cannot diverge. There are no flags and no environment overrides for the config path.

Inside `imghostd` the pipeline is a single in-process HTTP stack:

1. **Routing.** A chi router exposes two surfaces. The reserved surface owns `/swagger` and `/swagger/*` and serves the generated OpenAPI UI; a hand-written method gate returns `405 Method Not Allowed` with `Allow: GET` on any non-GET (the catch-all below would otherwise swallow the request, and chi's per-router `MethodNotAllowed` is not usable when a `/*` pattern matches every method). The catch-all surface dispatches `/<root>/<path>` by splitting the first URL segment, rejecting reserved names defensively, and looking up the root in the whitelist. Unknown first segment → 404.
2. **Path resolution.** For every matched request the router decodes the URL-escaped suffix, rejects any literal `..` segment, canonicalizes with `path.Clean`, joins under the root's physical directory with a `filepath.Rel` containment check, then runs `EvalSymlinks` on the target (or its nearest existing ancestor, so PUT of a new file still validates). Three distinct error types all map to HTTP 403. The resulting `(urlKey, physical, effectiveDefaultAccess)` triple is carried through the request context.
3. **Handlers.** A file handler (`GET`/`PUT`/`DELETE`) and an ACL handler (`GET`/`PUT`/`DELETE` selected by a bare `?acl` query key) consume the pre-resolved context. Writes always require bearer auth; `GET` requires auth only when the resolved access is `private`.
4. **Permission layer.** A resolver walks the URL key upward one segment at a time, returning the first explicit rule and otherwise falling back to the effective default (per-root `access` override if set, otherwise the global `default_access`). Explicit rules are stored as `public`/`private` values in an embedded bbolt database.
5. **Storage.** File I/O goes directly to the filesystem; uploads stream into a same-directory `*.tmp`, are fsynced, then atomically renamed. bbolt lives at `<state_dir>/imghost.db` (XDG state default when `state_dir` is unset).

No queue, no sidecar, no external database. TLS is out of scope and expected to terminate at a reverse proxy.

## Core Functionality and Usage

**Target user:** an individual running a small file host for themselves or their site, who wants a single binary behind a reverse proxy — not a multi-tenant product.

Typical flows:

1. **Serve.** `GET /<root>/<path>` — resolver decides public vs private; public is served directly, private requires `Authorization: Bearer <api_key>`.
2. **Upload.** `PUT /<root>/<path>` with bearer and raw body. Optional `X-Access: public|private` writes an explicit ACL atomically with the file; rollback is paired with rename success/failure.
3. **Delete.** `DELETE /<root>/<path>` with bearer. Directories are refused with 403.
4. **Manage ACL.** `GET|PUT|DELETE /<root>/<path>?acl`. The bare `?acl` query key is mandatory and must be the only query; otherwise 400.
5. **Discover API.** `GET /swagger/index.html`.
6. **Operate.** `imghost service start|stop|status|logs` manages the local `imghostd` process through the platform's native service manager.

For the concrete schema, status codes, and request/response shapes see `docs/configuration.md`, `docs/permissions.md`, and `docs/swagger.yaml`.

## Key Technical Constraints and External Dependencies

- **Pure Go, no CGO.** Cross-compiled and distributed as a native binary per platform. Two binaries ship together: `imghostd` and `imghost`.
- **No external services.** bbolt is an embedded KV store; the daemon owns its own file.
- **Config is the only input.** Both binaries read `xdg.ConfigFile("imghost/config.toml")`. No env overrides, no `--config`, no per-binary config files.
- **Fail-fast startup.** Any of the following abort `imghostd` before the listener comes up: unknown-key/unparseable config, duplicate or reserved root name, non-absolute or non-directory root path, invalid access value, non-absolute `state_dir`, or a stale bbolt lock not acquired within 5 s. A missing `config.toml` or a config with zero `[[root]]` entries is **not** fatal: the daemon injects a public `_default` root pointing at `xdg.DataFile("imghost/data")` (the directory is created on demand). `_default` is a reserved name that users cannot claim in their own `[[root]]` entries.
- **Service integration is platform-specific.** Linux wraps `systemctl --user` + `journalctl --user-unit`; macOS wraps `launchctl bootstrap|bootout|print` and `log show`; Windows has no native user-service surface, so every `imghost service` subcommand prints guidance and exits 0 to keep cross-platform scripts working.
- **TLS out of scope.** Terminate at a reverse proxy.
- **No domain awareness.** Upload responses return `{"path": "/<root>/<path>"}` — never a full URL.
- **Streaming-friendly HTTP timeouts.** `ReadHeaderTimeout` is set (10 s) to defend against Slowloris, but no `ReadTimeout` or `WriteTimeout` is applied, so large uploads and downloads are not time-boxed. Graceful shutdown drains for up to 30 s on `SIGINT`/`SIGTERM` before bbolt is closed.

## Important Design Decisions

**Two binaries, not one.** The daemon and the CLI have opposite lifecycles (long-running vs short-lived) and opposite permission needs (network listener vs local service socket). Splitting them keeps the daemon free of cobra/service-manager dependencies and lets the CLI stay trivial to run from scripts.

**Explicit root whitelist.** Every URL namespace must be declared as a `[[root]]` entry. Adding a namespace is a config-level change that fails fast if the directory is missing or not a directory, preventing accidental exposure of unintended parts of the host filesystem. Per-root `access` overrides live on the root object itself so the resolver can honor them without re-scanning config.

**Single source of reserved names.** A leaf package exposes `IsName`; both config validation (to reject a clashing `root.name`) and the router (to refuse a clashing first segment) import it. The leaf package imports neither, so no cycle is possible and new reserved names are one-line additions.

**Reserved-route method gate on `/swagger`.** chi's `/*` catch-all matches every method, so a default `MethodNotAllowed` path cannot fire for reserved routes. A hand-written gate on the reserved sub-router returns `405` with `Allow: GET` (RFC 9110 §15.5.6) instead of letting non-GET requests fall through to the catch-all.

**S3-style `?acl` subresource.** Same URL, different semantics selected by a bare `?acl` query key. Keeps the API surface small and keeps ACL rules addressable by the exact path they protect.

**Permission inheritance, most-specific wins.** Rules inherit up the tree. Directory-as-namespace matches creators' mental model ("make `/docs` private, except `/docs/public-notes`"), and the resolver is a simple suffix walk.

**Atomic write with same-directory temp file.** `os.CreateTemp(targetDir, "*.tmp")` + fsync + rename. Same-directory temp is load-bearing: a cross-filesystem rename would silently degrade to copy-then-delete and break atomicity. `X-Access`-driven permstore writes are rolled back on rename failure so file and ACL stay consistent.

**Path traversal defense in depth.** Four layers: URL-decode, segment-level `..` rejection, `filepath.Rel` containment check after join, and `EvalSymlinks` on the target (or nearest existing ancestor). All three distinct error types map to 403, so probing does not leak which defense tripped.

**Single API key.** Multi-tenant auth would triple complexity with no gain for the target user (a solo operator). Deferred, not designed in.

**bbolt with a bounded open timeout.** Opening the DB with a 5 s lock timeout turns a stale lock (previous process wedged) into a loud startup error instead of a silent hang — consistent with the fail-fast posture of the rest of config validation.
