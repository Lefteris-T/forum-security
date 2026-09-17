# Forum Security Extension — Product Requirements

## Purpose

Extend the completed Go forum so it satisfies the security exercise and audit
without regressing authentication, OAuth, forum content, image uploads,
persistence, or Docker behavior.

The release adds transport security and denial-of-service controls around the
existing application. Password hashing and server-side UUID sessions already
exist and must be retained, verified, and hardened for HTTPS.

## Source Requirements

This PRD clarifies:

- `docs/exercise-security.md`
- `docs/audit-security.md`

The exercise and audit remain authoritative if this document conflicts with
them.

## Current Baseline

The existing application already provides:

- bcrypt password hashing
- email/password registration and login
- GitHub and Google OAuth
- opaque UUID session cookies
- server-side SQLite session records
- one active session per user
- session expiry and logout invalidation
- posts, comments, categories, reactions, and filters
- optional validated JPEG, PNG, and GIF uploads up to exactly 20 MiB
- public forum and image viewing
- safe template escaping and parameterized SQL
- request logging, panic recovery, tests, and graceful shutdown
- Docker persistence for SQLite and uploaded images

The current application starts with plain `ListenAndServe`, has no explicit TLS
configuration, has no configured server request timeouts, and has no request
rate limiter. Those are the primary release gaps.

## Release Scope

### HTTPS

The documented audit run serves the forum at an `https://` URL.

The application supports configuration for:

```text
FORUM_HTTPS_ENABLED
FORUM_TLS_CERT_FILE
FORUM_TLS_KEY_FILE
```

When HTTPS is enabled:

- both certificate and private-key paths are required
- the certificate/key pair is validated before the server is considered ready
- missing, unreadable, malformed, or mismatched files stop startup safely
- the forum uses Go's TLS server support
- forum and OAuth cookies are marked `Secure`

Plain HTTP may remain available only as an explicitly selected development or
test mode. The README and audit walkthrough use HTTPS.

An HTTP-to-HTTPS redirect listener is optional. If implemented, it uses a
configured HTTPS origin rather than an untrusted request `Host` value.

### Local certificate

The project provides a reproducible OpenSSL command or script that creates its
own local self-signed certificate and private key.

The certificate contains Subject Alternative Names for:

- DNS name `localhost`
- IP address `127.0.0.1`

The private key uses restrictive filesystem permissions, is never committed,
and is not copied into a Docker image layer. A self-signed certificate produces
an expected browser trust warning. A public deployment should use a trusted
certificate authority.

### TLS policy and cipher suites

The server has an explicit `tls.Config`.

The fixed policy is:

- minimum TLS version is TLS 1.2
- TLS 1.0 and TLS 1.1 are rejected
- TLS 1.2 permits only modern ECDHE suites using AES-GCM or
  ChaCha20-Poly1305
- TLS 1.3 remains enabled and its cipher suites are selected by Go
- RC4, 3DES, legacy CBC-only suites, and RSA key-exchange suites are not enabled
- insecure certificate-verification settings are not used

This policy is unit tested and explained in the README so the auditor can see
both the code and the reasoning.

### HTTP server timeouts

The server defines non-zero limits for:

- `ReadHeaderTimeout`
- `ReadTimeout`
- `WriteTimeout`
- `IdleTimeout`
- `MaxHeaderBytes`

The initial policy is:

```text
ReadHeaderTimeout: 5 seconds
ReadTimeout:        2 minutes
WriteTimeout:       2 minutes
IdleTimeout:        60 seconds
MaxHeaderBytes:     1 MiB
```

Integration testing must confirm that valid 20 MiB uploads still work. Values
may be tuned with evidence, but every final value remains explicit, tested, and
documented.

### Rate limiting

The application implements an in-process, standard-library rate limiter.

The fixed design is:

- limit by normalized direct peer IP
- parse IPv4 and IPv6 using safe host/port parsing
- do not trust `X-Forwarded-For` or similar headers without a future explicit
  trusted-proxy configuration
- use a token-bucket design or an equivalently testable rate/burst model
- synchronize shared client state for concurrent requests
- remove inactive client entries
- stop cleanup work during application shutdown
- use a controllable clock for deterministic tests

Initial policies are:

```text
Global:        120 requests/minute/IP
Login POST:      5 requests/minute/IP
Register POST:   5 requests/minute/IP
Post writes:    20 requests/minute/IP
Comment writes: 30 requests/minute/IP
Reactions:      60 requests/minute/IP
```

The implementation documents how these values map to rate and burst. Both
successful and failed login attempts consume allowance. Blocked requests return:

```text
429 Too Many Requests
```

The response is generic and does not reveal whether an account exists. Include
`Retry-After` when it can be calculated accurately.

Static-file behavior is considered explicitly so ordinary page loads are not
blocked unexpectedly while the general limit still protects the application.

### Password protection

Password behavior remains:

- registration hashes passwords with bcrypt before persistence
- login compares the submitted password with the stored bcrypt hash
- plaintext passwords are never stored or logged
- OAuth-only accounts do not invent a local password

The audit verifies this through tests and by inspecting `users.password_hash`
in SQLite after registration. Although the exercise uses the word
"encryption," bcrypt hashing is the correct password-storage control.

### Sessions and cookies

Session behavior remains server-side:

- the browser cookie contains only a unique UUID
- user identity, creation time, and expiry remain in SQLite
- malformed or expired IDs do not authenticate
- logout deletes the server-side session
- a new login replaces the user's previous active session

Cookie requirements are:

- `HttpOnly=true`
- `SameSite=Lax`
- `Secure=true` whenever HTTPS is enabled
- appropriate path and expiration behavior

The effective rule is:

```text
secure cookie = HTTPS enabled OR explicitly configured secure cookie
```

The same HTTPS rule applies to OAuth state cookies. Cookies contain no email,
username, password, authorization role, or serialized session state.

### Error handling

Startup failures are returned clearly to the operator without exposing secrets.
This includes invalid configuration, certificate/key failures, bind failures,
serve failures, and shutdown failures.

Browser-facing errors use safe HTTP responses:

- malformed client input: appropriate `400` response
- unauthenticated access: `401 Unauthorized`
- missing resource: `404 Not Found`
- unsupported method: `405 Method Not Allowed`
- rate limit exceeded: `429 Too Many Requests`
- unexpected internal failure: generic `500 Internal Server Error`

The application continues serving after a recoverable handler panic. Logs may
contain useful method/path/status context but never passwords, cookies, OAuth
tokens or secrets, private keys, certificate contents, request bodies, SQL
values, stack traces sent to clients, or sensitive filesystem paths.

### Docker

Docker Compose supports the documented HTTPS audit run.

- the chosen HTTPS port is exposed and mapped
- HTTPS and certificate paths are configured through environment values
- certificate files are mounted read-only at runtime
- the private key is not baked into an image layer
- the non-root process can read the mounted key without making it broadly
  accessible
- secure cookies are enabled
- OAuth callback examples use the matching `https://` URL
- existing database and upload volumes remain unchanged and persistent

## Additional Hardening

After mandatory requirements pass, the release may add:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: same-origin`
- disabled directory listings under static paths

Content Security Policy requires compatibility testing with all templates,
styles, images, and OAuth flows. HSTS is used only for an intentional deployment
with a trusted certificate and is not required for local self-signed testing.

These controls must not delay HTTPS, TLS, timeout, rate-limit, password, and
session audit requirements.

## Architecture

The security extension preserves existing package responsibilities:

```text
environment
-> config validation
-> application/server construction
-> TLS listener and timeout policy
-> logging/recovery/rate-limit middleware
-> authentication middleware
-> router/handlers
-> services/repositories
-> SQLite and managed upload storage
```

- `internal/config` parses and validates runtime values
- `internal/app` constructs TLS, the HTTP server, dependencies, and lifecycle
- `internal/web/middleware` owns request rate limiting
- `internal/session` owns opaque session cookies
- existing handlers, services, and repositories retain their current business
  and persistence responsibilities

Rate limiting does not belong inside authentication services, and TLS does not
belong inside individual HTTP handlers.

## Baseline Regression Requirements

The following continue to work without changed user-facing semantics:

- local registration, login, and logout
- GitHub and Google OAuth
- OAuth state, PKCE, provider timeouts, identity, and collision behavior
- public home, post detail, category, and static image routes
- authenticated post, comment, and reaction routes
- created-post and liked-post filters
- UUID session expiry and replacement
- text-only posts
- JPEG, PNG, and GIF image posts
- exact image-size enforcement and safe image cleanup
- guest image viewing
- SQLite migrations, constraints, foreign keys, and transactions
- Docker database and upload persistence
- server-rendered HTML/CSS without JavaScript

## Technology Constraints

- Go standard library for TLS, HTTP timeouts, certificates, concurrency, and
  rate limiting
- existing SQLite driver
- existing `golang.org/x/crypto/bcrypt` dependency
- existing UUID dependency
- no new third-party security, proxy, TLS, or rate-limit package
- server-rendered HTML and CSS
- Docker-compatible runtime

## Testing Requirements

Each implementation phase includes focused tests. Final verification includes:

```text
gofmt on changed Go files
go vet ./...
go test ./...
go test -race ./...
go build ./...
docker compose build
```

Automated coverage includes:

- HTTPS configuration validation
- TLS minimum version and cipher policy
- server timeout policy
- valid and invalid certificate/key pairs
- HTTPS startup and graceful shutdown
- secure session and OAuth cookies
- unique and malformed UUID sessions
- allowed, blocked, refilled, independent, concurrent, and cleaned limiter state
- HTTP `429` behavior
- safe error responses and logging
- all existing authentication, forum, image, and persistence regressions

The race detector must cover concurrent limiter access and lifecycle shutdown.

## Manual Audit Acceptance

The release is ready when a reviewer can:

1. Generate the documented local certificate.
2. Start the forum using documented HTTPS configuration.
3. Open the forum at an `https://` URL.
4. Inspect the explicit TLS version and cipher policy.
5. Inspect non-zero read, write, header, and idle timeouts.
6. Exceed a rate limit and receive `429 Too Many Requests`.
7. Register a user and confirm SQLite stores a bcrypt hash, not plaintext.
8. Log in and confirm the cookie is a UUID with `Secure`, `HttpOnly`, and
   `SameSite=Lax`.
9. Confirm the UUID maps to server-side session state.
10. Demonstrate configurable certificate paths and safe invalid-config failure.
11. Confirm only allowed packages are used.
12. Exercise the existing forum and image flow without crashes or regressions.
13. Run the full test, race, vet, build, and Docker checks successfully.

## Out Of Scope For The Passing Release

- encrypted or password-protected SQLite database, which is bonus work
- production certificate-authority automation
- automatic certificate renewal
- a general-purpose reverse-proxy trust system
- distributed rate limiting across multiple application instances
- account lockout or administrator unlock workflows
- new OAuth providers, account linking, password reset, moderation, or roles
- replacing the existing forum or image-upload architecture
- JavaScript, SPA frameworks, or a public JSON API
