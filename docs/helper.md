# Forum Security Audit — Quick Answers

Use this as a concise speaking guide for `docs/audit-security.md`.

## Functional

### Does the forum use HTTPS?

Yes. The audit URL is `https://localhost:8443`. When
`FORUM_HTTPS_ENABLED=true`, `internal/app/app.go` uses `ListenAndServeTLS` and
validates the certificate/key pair before serving.

### Does it implement cipher suites?

Yes. `internal/app/tls.go` explicitly permits modern TLS 1.2 ECDHE suites with
AES-GCM or ChaCha20-Poly1305. RC4, 3DES, CBC-only suites, and static RSA key
exchange are not enabled. Go manages TLS 1.3 suites.

Evidence: `internal/app/tls.go` and `internal/app/tls_test.go`.

### Is the Go TLS configuration secure?

Yes:

- minimum version is TLS 1.2, rejecting TLS 1.0 and 1.1
- TLS 1.2 uses an explicit modern cipher list
- TLS 1.3 remains enabled and Go-managed
- `InsecureSkipVerify` is not used
- missing, malformed, or mismatched certificates stop startup

Evidence: `internal/app/tls.go`, `internal/app/server.go`, and
`internal/app/https_test.go`.

### Are server timeouts configured?

Yes. `internal/app/server.go` configures:

| Setting | Value |
|---|---:|
| `ReadHeaderTimeout` | 5 seconds |
| `ReadTimeout` | 2 minutes |
| `WriteTimeout` | 2 minutes |
| `IdleTimeout` | 60 seconds |
| `MaxHeaderBytes` | 1 MiB |

These reduce slow-client and Slowloris-style resource exhaustion while allowing
legitimate 20 MiB uploads. Evidence: `internal/app/server_test.go`.

### Is rate limiting implemented?

Yes. A concurrent token bucket limits each normalized direct client IP:

| Scope | Rate/burst |
|---|---:|
| All requests | 120/minute |
| Login | 5/minute |
| Registration | 5/minute |
| Post creation | 20/minute |
| Comment creation | 30/minute |
| Reactions | 60/minute |

The sixth immediate login attempt returns `429 Too Many Requests` with
`Retry-After: 12` and a safe `GET /login` link. Successful and failed attempts
both count. IPv4/IPv6 are normalized, forwarding headers are ignored, stale
buckets are removed, and cleanup stops during shutdown.

Evidence: `internal/web/middleware/rate_limit.go`,
`internal/web/middleware/rate_limit_http.go`, and their tests.

### Are passwords protected?

Yes. Passwords are correctly protected with one-way bcrypt hashing:

- registration uses `bcrypt.GenerateFromPassword`
- login uses `bcrypt.CompareHashAndPassword`
- plaintext passwords are not stored or logged

Check the database:

```bash
sqlite3 data/forum.db
```

```sql
.headers on
.mode column
SELECT id, email, username, password_hash FROM users;
```

The stored value must be a bcrypt hash, not the original password. Evidence:
`internal/service/password.go` and its tests.

### Does the session cookie contain a UUID?

Yes. `internal/session/manager.go` generates IDs with `uuid.NewString()`. The
cookie contains only the opaque UUID; identity and expiry remain in SQLite.

```text
HttpOnly=true
SameSite=Lax
Secure=true when HTTPS is enabled
```

Malformed, unknown, and expired IDs do not authenticate. Login replaces the
old session and logout deletes it. Verify `forum_session` in browser developer
tools and optionally query `SELECT id, user_id, expires_at FROM sessions;`.

### Can certificate paths be configured?

Yes, through environment variables:

```env
FORUM_HTTPS_ENABLED=true
FORUM_TLS_CERT_FILE=certs/localhost.crt
FORUM_TLS_KEY_FILE=certs/localhost.key
```

Both paths are mandatory in HTTPS mode. Docker mounts `./certs` read-only at
`/run/certs`; the key is not copied into the image.

Evidence: `.env.example`, `internal/config/config.go`, and `compose.yml`.

### Are only allowed packages used?

Yes. The direct dependencies in `go.mod` are only:

```text
github.com/mattn/go-sqlite3
golang.org/x/crypto
github.com/google/uuid
```

TLS, HTTP timeouts, and rate limiting use the Go standard library.

### Are website and technical errors handled?

Yes. The application uses appropriate `400`, `401`, `404`, `405`, `409`,
`429`, `500`, and `502` responses.

- expected form errors are shown safely in the relevant page
- oversized images re-render the post form with `400`
- internal failures return a generic `500`
- panic recovery keeps the server running
- logs exclude passwords, cookies, bodies, secrets, and OAuth tokens
- certificate, bind, serving, and shutdown failures are returned safely

Evidence: handler/integration tests, `recovery_test.go`, and `logging_test.go`.


## General and bonus

### Does it generate its own certificate?

Yes:

```bash
make cert
make verify-cert
```

`scripts/generate-cert.sh` creates a self-signed certificate with SANs for
`localhost` and `127.0.0.1`, applies restrictive permissions, and refuses to
overwrite an existing key. A browser warning is expected locally.


## SQLite reminders

```sql
.tables
.schema users
.headers on
.mode column
SELECT * FROM users;
```