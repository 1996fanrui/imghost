# Permission Model

imghost protects reads with a simple, hierarchical ACL. Writes (`PUT`, `DELETE`) always require `Authorization: Bearer <API_KEY>`; this document covers **read** access only.

## Access values

Every path resolves to one of two access values:

- `public` — anyone can `GET` the object.
- `private` — `GET` requires `Authorization: Bearer <API_KEY>`.

## Three granularities

Rules can be attached at three levels. Each level is just a path stored in the permission DB — the level is implied by what that path refers to:

| Granularity | Example rule key | Affects                                                           |
| ----------- | ---------------- | ----------------------------------------------------------------- |
| File        | `/photos/cat.jpg`| Exactly that file.                                                |
| Directory   | `/photos`        | Every descendant with no more-specific rule.                      |
| Global      | `default_access` | Everything with no explicit rule anywhere up the tree.            |

File and directory rules are created with `PUT /<path>?acl`. The global default comes from the `default_access` field in `config.toml` (`public` if unset).

## Resolution priority

On every read, imghost resolves the effective access by walking the request path **from the leaf upward**, returning the first explicit rule it finds, and falling back to the global default if none exists.

```
/photos/trips/paris/eiffel.jpg
        → check /photos/trips/paris/eiffel.jpg
        → check /photos/trips/paris
        → check /photos/trips
        → check /photos
        → check /
        → fall back to default_access
```

**Priority, most-specific wins:** file rule > nearer directory rule > farther directory rule > `default_access`.

### Example

Given:

- `default_access=public`
- Rule on `/docs` → `private`
- Rule on `/docs/public-notes` → `public`
- Rule on `/docs/public-notes/draft.md` → `private`

| Request                               | Effective | Why                                      |
| ------------------------------------- | --------- | ---------------------------------------- |
| `GET /photos/cat.jpg`                 | public    | no rule up the tree → `default_access`   |
| `GET /docs/report.pdf`                | private   | inherits `/docs`                         |
| `GET /docs/public-notes/intro.md`     | public    | `/docs/public-notes` overrides `/docs`   |
| `GET /docs/public-notes/draft.md`     | private   | file rule beats ancestor rules           |

## Managing rules

```bash
TOKEN="Authorization: Bearer <API_KEY>"
BASE=http://localhost:34286

# Set a directory rule (affects every descendant without a nearer rule)
curl -X PUT -H "$TOKEN" -H 'Content-Type: application/json' \
     -d '{"access":"private"}' "$BASE/docs?acl"

# Override for one sub-tree
curl -X PUT -H "$TOKEN" -H 'Content-Type: application/json' \
     -d '{"access":"public"}' "$BASE/docs/public-notes?acl"

# Inspect the explicit rule at a path (404 if none)
curl -H "$TOKEN" "$BASE/docs?acl"

# Remove the explicit rule — access falls back to the next level up
curl -X DELETE -H "$TOKEN" "$BASE/docs?acl"
```

Notes:

- `GET /<path>?acl` returns the **explicit** rule at that exact path, or 404 if there is none. It does not perform upward resolution — use a normal `GET /<path>` to observe the effective access.
- Deleting a rule does not delete descendant rules; it only removes the one at that exact path.
- Rules can be attached to paths that do not (yet) exist on disk. This lets you pre-declare a policy before uploading.
