# Forum

A server-rendered forum written in Go with SQLite persistence, email/password
and OAuth authentication, optional post images, and an HTTPS-focused security
layer. The application uses the standard `net/http` stack and plain HTML/CSS;
it does not require JavaScript.

## Features

- email/password registration, login, and logout
- GitHub and Google OAuth login with state validation and PKCE
- bcrypt password hashing
- opaque UUID session cookies backed by server-side SQLite sessions
- one active session per user, expiry checks, and logout invalidation
- public posts, post details, categories, comments, and images
- authenticated post, comment, like, and dislike actions
- category, created-post, and liked-post filters
- optional JPEG, PNG, and GIF uploads with an exact 20 MiB limit
- configurable HTTPS with an explicit TLS policy
- HTTP server timeouts and per-client rate limiting
- secure cookies, security response headers, and panic recovery
- SQLite migrations and persistent Docker volumes
- unit, integration, HTTPS, and race-detector tests

## Requirements

- Go 1.25 or newer
- a C compiler for `github.com/mattn/go-sqlite3`
- OpenSSL for local certificate generation
- Docker and Docker Compose for the container workflow

The only direct non-standard Go dependencies are the exercise-approved
SQLite, bcrypt, and UUID packages.

## Quick start with HTTPS

Generate a self-signed certificate:

```bash
make cert
```

The command creates:

```text
certs/localhost.crt
certs/localhost.key
```

The certificate has Subject Alternative Names for `localhost` and
`127.0.0.1`. The generation script refuses to overwrite either file. Generated
certificates and private keys are ignored by Git and excluded from the Docker
build context.

Create the local environment file:

```bash
cp .env.example .env
```

The example is ready for HTTPS without OAuth. Start the forum:

```bash
make run
```

Open:

```text
https://localhost:8443
```

A browser warning is expected because the certificate is self-signed. This is
appropriate for local development and auditing; use a certificate issued by a
trusted authority for a public deployment.

The certificate can be inspected with:

```bash
make verify-cert
```

### Explicit plain-HTTP development mode

Plain HTTP is available only when selected explicitly. For a temporary local
run:

```bash
FORUM_ADDRESS=:8080 \
FORUM_HTTPS_ENABLED=false \
FORUM_TLS_CERT_FILE= \
FORUM_TLS_KEY_FILE= \
FORUM_SECURE_COOKIE=false \
go run ./cmd/forum
```

Open `http://localhost:8080`. Do not use this mode for the security audit or a
public deployment.

## Configuration

Configuration is loaded from environment variables. The program validates it
before opening the database or serving requests.

| Variable | Default | Purpose |
|---|---|---|
| `FORUM_ADDRESS` | `:8080` | Server listen address |
| `FORUM_DATABASE_PATH` | `data/forum.db` | SQLite database path |
| `FORUM_SESSION_DURATION` | `24h` | Server-side session lifetime |
| `FORUM_COOKIE_NAME` | `forum_session` | Authentication cookie name |
| `FORUM_SECURE_COOKIE` | `false` | Force secure cookies, including behind trusted TLS termination |
| `FORUM_HTTPS_ENABLED` | `false` | Select direct HTTPS serving |
| `FORUM_TLS_CERT_FILE` | empty | PEM certificate path |
| `FORUM_TLS_KEY_FILE` | empty | PEM private-key path |
| `GITHUB_CLIENT_ID` | empty | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | empty | GitHub OAuth client secret |
| `GITHUB_REDIRECT_URL` | empty | GitHub callback URL |
| `GOOGLE_CLIENT_ID` | empty | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | empty | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | empty | Google callback URL |

When HTTPS is enabled, both TLS file paths are required. A missing, malformed,
or mismatched certificate/key pair stops startup. Setting only one of an OAuth
provider's three variables also stops startup; leave all three empty to disable
that provider.

HTTPS always forces secure session and OAuth state cookies, even if
`FORUM_SECURE_COOKIE=false` was supplied:

```text
effective secure cookie = HTTPS enabled OR secure cookie explicitly enabled
```

### OAuth callbacks

For the local HTTPS run, configure providers with these exact callback URLs:

```text
https://localhost:8443/auth/github/callback
https://localhost:8443/auth/google/callback
```

Put real credentials only in `.env` or a deployment secret manager. Never add
them to `.env.example`, Compose, the Dockerfile, source code, logs, or commits.

## Docker HTTPS

Generate the certificate first, then allow the non-root container process to
read the mounted key through a supplementary group:

```bash
make cert
cp .env.example .env
host_cert_gid="$(id -g)"
chgrp "$host_cert_gid" certs certs/localhost.crt certs/localhost.key
chmod 750 certs
chmod 644 certs/localhost.crt
chmod 640 certs/localhost.key
sed -i "s/^FORUM_CERT_GID=.*/FORUM_CERT_GID=$host_cert_gid/" .env
docker compose up --build
```

If `make cert` reports that files already exist, keep the existing pair or
remove it deliberately before generating a replacement. The script never
silently replaces a private key.

Open:

```text
https://localhost:8443
```

Compose mounts `./certs` read-only at `/run/certs`; the private key is not baked
into the image. The application runs as a non-root user. SQLite data and post
images use the named volumes `forum-data` and `forum-uploads`.

Stop the containers without deleting persistent data:

```bash
docker compose down
```

Avoid `docker compose down -v` unless both the database and uploaded images
should be deleted.

The helper scripts provide the same common operations:

```bash
./scripts/build.sh
./scripts/run.sh
./scripts/stop.sh
```

## Security design

### TLS policy

The server uses an explicit `tls.Config` with TLS 1.2 as the minimum. TLS 1.0
and 1.1 are therefore rejected. TLS 1.2 is restricted to ECDHE key exchange
with AES-GCM or ChaCha20-Poly1305 authenticated encryption. RC4, 3DES,
CBC-only suites, and static RSA key exchange are not enabled.

TLS 1.2 remains available for reasonable client compatibility while the
explicit ECDHE/AEAD list removes its legacy choices. TLS 1.3 remains enabled;
Go deliberately controls its cipher suites because they are not configured
through `tls.Config.CipherSuites`. The application does not use
`InsecureSkipVerify`.

### HTTP server policy

The server uses non-zero resource limits:

| Setting | Value | Protection |
|---|---:|---|
| `ReadHeaderTimeout` | 5 seconds | Slow request headers |
| `ReadTimeout` | 2 minutes | Slow or stalled request bodies |
| `WriteTimeout` | 2 minutes | Stalled response clients |
| `IdleTimeout` | 60 seconds | Excessive idle keep-alive connections |
| `MaxHeaderBytes` | 1 MiB | Oversized request headers |

The two-minute body and response limits leave room for legitimate 20 MiB image
uploads while still bounding slow clients.

### Rate limiting

An in-memory token bucket protects each normalized direct peer IP:

| Scope | Refill rate | Burst |
|---|---:|---:|
| All requests | 120/minute | 120 |
| Login `POST` | 5/minute | 5 |
| Registration `POST` | 5/minute | 5 |
| Post creation | 20/minute | 20 |
| Comment creation | 30/minute | 30 |
| Reactions | 60/minute | 60 |

Every request consumes global allowance. Matching state-changing requests also
consume allowance from their route-specific bucket. Successful and failed
login attempts count equally. A blocked request receives a generic `429 Too
Many Requests` response and `Retry-After` header.

The key comes from `Request.RemoteAddr`; IPv4 and IPv6 are normalized and
forwarding headers are ignored. Do not trust `X-Forwarded-For` without an
explicit trusted-proxy boundary. When deploying behind a proxy, enforce limits
there or add a reviewed trusted-proxy configuration.

Buckets are mutex-protected, inactive entries are removed, and cleanup
goroutines stop during application shutdown. Limiter state is local to one
application instance and resets after restart. Users sharing a public IP also
share its allowance. Static assets remain subject to the global limit but not
the stricter write limits.

### Passwords, sessions, and cookies

- registration hashes passwords with `bcrypt.GenerateFromPassword`
- login verifies hashes with `bcrypt.CompareHashAndPassword`
- plaintext passwords are neither persisted nor logged
- OAuth-only accounts do not receive invented passwords
- session identifiers are generated UUIDs
- cookies contain only the opaque UUID, never user or authorization state
- session ownership and expiry are stored in SQLite
- a new login replaces the user's previous session
- logout deletes the server-side session
- malformed, missing, unknown, and expired session IDs do not authenticate
- session and OAuth state cookies use `HttpOnly` and `SameSite=Lax`
- direct HTTPS forces the `Secure` attribute

### Browser and HTTP hardening

All application responses, including static files and errors, receive:

```text
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: same-origin
```

Exact static files and uploaded image URLs remain public. Directory requests
such as `/static/` and `/static/uploads/` return `404` instead of listing their
contents.

HSTS is intentionally disabled for local self-signed certificates. Content
Security Policy and an HTTP-to-HTTPS redirect are separate deployment choices
and are not enabled here.

### Errors and logging

Invalid configuration and TLS files fail before serving. Browser-facing
internal errors are generic and do not expose database messages, local paths,
certificate contents, or stack traces. Panic recovery converts handler panics
to `500 Internal Server Error` and keeps the server available.

Request logs contain the method, URL path, and final status. They omit query
strings, request bodies, cookies, authorization headers, passwords, OAuth
tokens and secrets, private keys, certificate contents, and SQL values.

## Forum behavior

### Routes

Public routes:

```text
GET  /
GET  /register
POST /register
GET  /login
POST /login
GET  /auth/github
GET  /auth/github/callback
GET  /auth/google
GET  /auth/google/callback
GET  /posts/{id}
GET  /static/{file}
```

Authenticated routes:

```text
POST /logout
GET  /posts/new
POST /posts
POST /posts/{id}/comments
POST /posts/{id}/react
POST /comments/{id}/react
```

OAuth routes are mounted only for configured providers.

Useful filters:

```text
/?category={category-id}
/?filter=created
/?filter=liked
```

The category filter is public. Created and liked filters require a session.

### HTTP statuses

| Status | Meaning |
|---:|---|
| `200` | Successful page or file response |
| `303` | Successful state-changing form redirect |
| `400` | Malformed or invalid request |
| `401` | Authentication required or invalid credentials |
| `403` | Authenticated user lacks permission |
| `404` | Resource or route not found |
| `405` | Method not allowed; includes `Allow` where applicable |
| `409` | Duplicate email or username |
| `429` | Rate limit exceeded |
| `500` | Generic unexpected internal failure |
| `502` | OAuth provider failure |

### Post images

One optional image may accompany a post. Supported formats are JPEG, PNG, and
GIF, with an exact maximum of 20 MiB (`20,971,520` bytes). The server checks
the bytes rather than trusting the filename, declared content type, or
`Content-Length`, and decodes the image before publication.

Validated files are written atomically under `static/uploads` with generated
UUID filenames. SQLite stores only their public path. Failed post creation
cleans up a newly written image. Uploading requires authentication, while
reading an image remains public.

## Persistence and architecture

Migrations in `migrations/` run automatically at startup. The principal tables
are:

```text
users
sessions
oauth_accounts
categories
posts
post_categories
comments
post_reactions
comment_reactions
```

Foreign keys are enabled. Multi-row operations that must remain consistent use
transactions, and repository SQL uses parameters.

Request processing follows these boundaries:

```text
environment configuration
-> application and HTTPS server lifecycle
-> logging / recovery / security headers / rate limiting
-> authentication
-> router and handlers
-> validation and services
-> repositories
-> SQLite and managed upload storage
```

Important directories:

```text
cmd/forum/              executable entry point
internal/app/           application wiring and server lifecycle
internal/config/        environment parsing and validation
internal/database/      SQLite opening and migrations
internal/oauth/         OAuth providers, state, PKCE, and callbacks
internal/repository/    parameterized persistence
internal/service/       authentication and forum business rules
internal/session/       UUID cookie mechanics
internal/upload/        image validation and atomic storage
internal/web/           router, handlers, middleware, and views
migrations/             ordered database schema migrations
templates/              server-rendered HTML
static/                 CSS, background art, and runtime uploads
docs/                   requirements, task plan, and audit checklist
```

## Testing and verification

Run the standard verification gate:

```bash
gofmt -w $(git ls-files '*.go')
go vet ./...
go test ./...
go test -race ./...
go build ./...
docker compose build
```

The suite covers configuration, TLS policy, certificate failures, HTTPS
requests, server shutdown, rate limits, headers, panic recovery, authentication,
OAuth, UUID sessions, migrations, repositories, forum behavior, upload
boundaries, templates, and real temporary SQLite databases.

## Security audit walkthrough

1. Generate the certificate with `make cert` and inspect it with
   `make verify-cert`.
2. Start the HTTPS Compose deployment and open
   `https://localhost:8443`.
3. Confirm the negotiated connection is HTTPS and TLS 1.0/1.1 are unavailable.
4. Register a user and inspect `users.password_hash` in SQLite; it must be a
   bcrypt hash, never the submitted password.
5. Log in and inspect `forum_session`; its value must be a UUID with `Secure`,
   `HttpOnly`, and `SameSite=Lax` attributes.
6. Confirm session identity and expiry exist in the `sessions` table rather
   than in the cookie.
7. Submit more than five login attempts from one client within the burst and
   confirm a generic `429` response with `Retry-After`.
8. Confirm `/static/style.css` and exact uploaded image URLs work while
   `/static/` and `/static/uploads/` return `404`.
9. Inspect the security headers on successful, error, and static responses.
10. Run the complete verification gate and confirm no secret, certificate,
    database, runtime upload, log, or build artifact is tracked by Git.

Database encryption is bonus work and is not implemented. Passwords are
correctly protected with one-way bcrypt hashing; the SQLite database itself is
not password-encrypted.

The authoritative security requirements and detailed phase plan are in
[`docs/exercise-security.md`](docs/exercise-security.md),
[`docs/audit-security.md`](docs/audit-security.md), and
[`docs/tasks.md`](docs/tasks.md).
