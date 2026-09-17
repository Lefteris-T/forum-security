# AGENTS.md — Forum Security Extension Guide

## Project

This repository contains a completed Go forum with email/password and OAuth
authentication, UUID server-side sessions, posts, comments, categories,
reactions, filters, optional image uploads, SQLite persistence, server-rendered
templates, tests, and Docker support.

The active extension is forum security. Add HTTPS, explicit TLS settings,
server timeouts, certificate configuration, rate limiting, and HTTPS cookie
hardening without reimplementing completed functionality.

Preserve all existing forum and image-upload behavior. Do not add unrelated
features such as new OAuth providers, account linking, password reset,
moderation, roles, JavaScript, or a public JSON API.

## Source Of Truth

When documents disagree, follow this order:

1. `docs/exercise-security.md` — supplied security subject
2. `docs/audit-security.md` — security auditor checks
3. `docs/PRD.md` — clarified behavior and fixed safety decisions
4. `docs/tasks.md` — learning-oriented implementation order and acceptance
   checks
5. `README.md` — commands verified against the completed implementation
6. existing tests and code — completed forum baseline and regression behavior

Do not silently reinterpret an exercise requirement. Record a genuinely
unclear product or security decision in the PRD and task plan before
implementing it.

## Mandatory Scope

- serve the forum through HTTPS during the audit
- load a configurable certificate and private key
- provide instructions for generating a local self-signed certificate
- configure Go TLS deliberately, including the minimum version and cipher
  policy
- configure read-header, read, write, and idle server timeouts
- implement per-client rate limiting and return `429 Too Many Requests`
- retain bcrypt password hashing and verify plaintext is never stored
- retain unique UUID session identifiers and server-side session state
- enforce secure session and OAuth cookies when HTTPS is enabled
- handle configuration, certificate, network, HTTP, and internal errors safely
- use only standard Go packages plus the already allowed SQLite, bcrypt, and
  UUID dependencies
- add focused tests and keep the full regression suite passing
- document a reproducible audit walkthrough

Database encryption/password protection is bonus work. Do not begin it until
all mandatory requirements pass.

## Baseline Regression Rules

Preserve:

- email/password registration, login, and logout
- GitHub and Google OAuth login
- OAuth state, PKCE, provider identity, collision, and secret-handling rules
- UUID sessions, expiry, logout invalidation, and one active session per user
- public forum/category reading
- authenticated post, comment, and reaction behavior
- category, created-post, and liked-post filters
- optional JPEG, PNG, and GIF post images
- exact 20 MiB image limit and safe image validation/storage
- guest access to post images
- SQLite migrations, foreign keys, and transactions
- Docker persistence for the database and uploads
- server-rendered HTML/CSS without JavaScript
- parameterized SQL and `html/template` escaping

Security work must not bypass authentication, weaken upload validation, expose
internal data, or make existing public routes private.

## Architecture

Keep security controls at their natural boundaries:

```text
environment configuration
-> application and server construction
-> TLS listener and server timeouts
-> logging/recovery/security/rate-limit middleware
-> authentication middleware
-> router and handlers
-> services and repositories
-> SQLite/filesystem
```

- configuration owns parsing and validation of environment values
- application startup owns certificate loading, TLS/server construction,
  lifecycle, and clean shutdown
- middleware owns request-level rate limiting and security headers
- the session package owns opaque cookie creation and validation
- services retain authentication and business rules
- repositories retain parameterized persistence logic

Use focused interfaces only at useful test boundaries. Do not add abstraction
layers merely to increase interface count.

## HTTPS And Certificate Rules

- use the standard library TLS and HTTP server support
- make HTTPS, certificate path, and private-key path explicit configuration
- when HTTPS is enabled, require both certificate and key paths
- fail startup clearly for missing, malformed, or mismatched certificate files
- never log private-key contents
- generate local certificates with Subject Alternative Names for `localhost`
  and `127.0.0.1`
- keep private keys, runtime certificates, and environment files out of Git and
  container image layers
- mount certificates read-only into Docker when running through Compose
- keep plain HTTP only as an explicitly selected development/test mode
- the documented audit run must use an `https://` URL

Self-signed certificates are acceptable for local/audit use and will cause an
expected browser trust warning. Public deployment should use a trusted
certificate authority.

## TLS And Server Rules

- reject TLS 1.0 and TLS 1.1
- support TLS 1.2 only with an explicit modern ECDHE cipher list
- let Go select safe TLS 1.3 cipher suites
- do not enable RC4, 3DES, legacy CBC-only suites, RSA key exchange, or
  insecure certificate verification
- configure non-zero `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and
  `IdleTimeout`
- keep timeout values compatible with legitimate 20 MiB image uploads
- retain signal-aware graceful shutdown
- return startup and shutdown failures without hiding them or leaking secrets

TLS policy and timeout construction must be directly unit tested rather than
proved only through comments.

## Rate-Limiting Rules

- use only standard-library packages
- limit by normalized direct peer IP
- parse IPv4 and IPv6 safely
- do not trust forwarding headers without an explicit trusted-proxy design
- apply a general request limit and stricter rules to login and registration
- count both failed and successful login attempts
- apply appropriate rules to state-changing post, comment, and reaction routes
- return `429 Too Many Requests` with a generic response
- include `Retry-After` when it can be calculated correctly
- protect shared limiter state from concurrent access
- remove inactive client state so memory cannot grow without bound
- stop cleanup work during application shutdown
- use a controllable clock in tests; avoid slow, timing-sensitive sleeps
- verify concurrent behavior with the race detector

Middleware ordering must keep logging and recovery effective for rejected
requests and must not leak whether an account exists.

## Password, Session, And Cookie Rules

- retain `bcrypt.GenerateFromPassword` for storage and
  `bcrypt.CompareHashAndPassword` for login verification
- never store or log plaintext passwords
- retain server-generated UUID session identifiers
- keep user identity, expiry, and other session state in SQLite
- reject malformed and expired session IDs
- retain logout deletion and single-active-session replacement
- retain `HttpOnly` and `SameSite=Lax`
- HTTPS must force `Secure` on forum session and OAuth state cookies
- cookies must not contain email, username, password, or serialized user state

## Error And Logging Rules

- fail invalid startup configuration before serving requests
- return standard, safe HTTP statuses including `400`, `401`, `404`, `405`,
  `429`, and `500`
- keep browser-facing internal errors generic
- log enough context for operators without logging request bodies, passwords,
  cookies, OAuth tokens/secrets, private keys, SQL values, or filesystem secrets
- preserve panic recovery so one request does not terminate the server
- do not expose certificate contents, stack traces, database errors, or local
  paths to clients

## Additional Hardening

After mandatory work passes, it is reasonable to add:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: same-origin`
- disabled static directory listings

Treat CSP, HSTS, and HTTP-to-HTTPS redirect as separate, explicitly tested
decisions. HSTS should not interfere with local self-signed certificate testing.

## Testing And Audit Rules

- work in the order in `docs/tasks.md`
- write focused tests with each behavior rather than in one final batch
- use temporary certificate files and `t.TempDir()` in tests
- never include a real private key or secret in a fixture
- preserve all authentication, OAuth, forum, image, and Docker regression tests
- run focused tests after each change and `go test ./...` before each commit
- before completion run `gofmt`, `go vet ./...`, `go test ./...`,
  `go test -race ./...`, `go build ./...`, and the Docker build
- manually rehearse every question in `docs/audit-security.md`
- keep commits small, coherent, tested, and explainable to a learner
- do not claim commands or behavior in the README until they have been run

## Repository Safety Rules

- keep secrets, tokens, private keys, `.env` files, runtime databases, uploaded
  images, logs, temporary files, caches, and build artifacts out of Git
- preserve unrelated working-tree changes
- do not edit previously applied migrations for security work
- do not add a third-party rate-limit or TLS dependency
- do not commit generated certificates solely to make tests or Docker work
