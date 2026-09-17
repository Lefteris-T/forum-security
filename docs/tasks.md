# Forum Security Extension — Learning And Implementation Plan

## Goal

Extend the existing forum with the mandatory security requirements from:

1. `docs/exercise-security.md`
2. `docs/audit-security.md`

Preserve the completed forum, OAuth, image-upload, SQLite, Docker, password,
and session behavior. Build the extension in small, testable commits so every
security control can be explained during the audit.

The existing architecture remains:

```text
cmd/forum
-> configuration and application lifecycle
-> HTTP/TLS server and middleware
-> router and handlers
-> services
-> repositories
-> SQLite
```

Do not rewrite authentication, sessions, or password handling. Extend and
verify the current implementation.

## Source Of Truth

If documents disagree, follow this order for the security extension:

1. `docs/exercise-security.md`
2. `docs/audit-security.md`
3. `docs/PRD.md`, after it is aligned with this extension
4. this task plan
5. `README.md`
6. existing tests and code

Before production changes, update stale project guidance that still describes
image upload as the current extension. Image upload remains required regression
behavior, but security is now the active extension.

## Scope Classification

### Mandatory for the audit

- serve the forum through HTTPS
- load a configurable certificate and private key
- configure TLS deliberately, including the minimum version and cipher policy
- configure HTTP server read, write, header, and idle timeouts
- implement rate limiting that returns `429 Too Many Requests`
- keep passwords stored as bcrypt hashes
- keep session identifiers as unique UUIDs
- keep session state on the server rather than inside the cookie
- enforce secure cookie behavior when HTTPS is enabled
- handle startup, TLS, HTTP, and internal errors safely
- use only allowed packages
- add focused tests and preserve all existing behavior
- document a reproducible HTTPS setup for the auditor

### Additional hardening after mandatory work

- basic security response headers
- disabling static directory listings
- optional HTTP-to-HTTPS redirect
- optional Content Security Policy
- optional HSTS for a trusted-certificate deployment

### Bonus, deferred until all mandatory checks pass

- password-protected or encrypted SQLite database

SQLite encryption is not part of the first passing implementation. It usually
requires a different SQLite build or extension and must not delay mandatory
work.

## Existing Security Baseline To Preserve

- bcrypt password hashing in `internal/service/password.go`
- parameterized SQL and `users.password_hash`
- opaque UUID session IDs in `internal/session/manager.go`
- server-side sessions in SQLite
- one active session per user
- session expiry and logout invalidation
- `HttpOnly` and `SameSite=Lax` cookies
- generic invalid-credentials behavior
- recovery and request-logging middleware
- graceful shutdown
- bounded and validated image uploads
- public image viewing for guests
- OAuth state and PKCE protections
- safe template escaping

## Working Method For Every Phase

Use this cycle for every behavior:

1. Explain the threat or audit requirement in plain language.
2. Identify the smallest architectural boundary that should own it.
3. Write a focused failing test where practical.
4. Implement only that behavior.
5. Run the focused tests.
6. Run `go test ./...` for regressions.
7. Review the diff and explain why it is safe.
8. Make one small commit.

Before each commit, be able to answer:

- What problem does this change prevent?
- Why does this code belong in this package?
- Which test proves the behavior?
- What happens when the operation fails?
- Did an existing forum feature change accidentally?

Do not postpone all tests until the end. The final test phase is a verification
gate, not the first time security behavior is tested.

---

# Phase 0 — Align Documents And Record The Baseline

## Learn

- distinguish transport security, application security, and data-at-rest
  security
- understand which requirements already exist and which are new
- understand that bcrypt hashes passwords; it does not encrypt them

## Tasks

- update `docs/AGENTS.md` and `docs/PRD.md` so the security exercise is the
  active extension and the completed image-upload work is baseline behavior
- record the source-of-truth order from this plan
- run the existing test suite before changing production code
- inspect `go.mod` and confirm that only the allowed direct dependencies are
  used
- record the current findings:
  - HTTP only
  - no explicit TLS configuration
  - no server request timeouts
  - no rate limiter
  - bcrypt and UUID sessions already implemented

## Acceptance

- project documents no longer disagree about the active exercise
- `go test ./...` passes before implementation begins
- no production code changes are made in this phase

## Suggested Commit

```text
docs: define forum security extension scope
```

---

# Phase 1 — Define And Validate HTTPS Configuration

## Learn

- separate configuration validation from server startup
- understand why certificate paths and private keys must not be hard-coded
- understand fail-fast startup behavior

## Decisions

Use configuration such as:

```text
FORUM_HTTPS_ENABLED
FORUM_TLS_CERT_FILE
FORUM_TLS_KEY_FILE
```

Keep plain HTTP only as an explicitly selected development/test mode. The
documented audit command must start HTTPS.

The invariant is:

```text
HTTPS enabled -> certificate path and key path are both required
```

Do not log or return private-key contents.

## Tasks

- extend `internal/config.Config` with HTTPS, certificate, and key settings
- parse and validate the HTTPS boolean
- reject configurations with only one of the certificate/key paths
- reject missing certificate/key paths when HTTPS is enabled
- decide whether file existence belongs in config validation or application
  startup, and test that boundary consistently
- update `.env.example` with placeholders only

## Tests

- HTTPS disabled permits empty certificate settings
- HTTPS enabled accepts both paths
- HTTPS enabled rejects a missing certificate path
- HTTPS enabled rejects a missing key path
- invalid boolean values fail clearly
- no error includes certificate or key contents

## Focused Check

```bash
go test ./internal/config
```

## Suggested Commit

```text
feat: add validated HTTPS configuration
```

---

# Phase 2 — Define The TLS Policy

## Learn

- understand certificates versus cipher suites
- understand TLS negotiation
- understand why TLS 1.3 cipher suites are selected by Go rather than through
  `tls.Config.CipherSuites`

## Policy

- use the standard library `crypto/tls`
- reject TLS 1.0 and TLS 1.1
- use TLS 1.2 as the minimum unless the final compatibility decision requires
  TLS 1.3 only
- if TLS 1.2 is supported, explicitly allow only modern ECDHE suites using
  AES-GCM or ChaCha20-Poly1305
- allow Go to manage TLS 1.3 cipher suites
- do not enable RC4, 3DES, CBC-only legacy suites, or RSA key-exchange suites
- document the reason for the policy

TLS 1.2 with an explicit modern suite list plus Go-managed TLS 1.3 is the
clearest policy to demonstrate to the auditor.

## Tasks

- add a small helper that constructs `*tls.Config`
- keep TLS construction separate from `Run` so it can be unit tested
- set `MinVersion`
- set the TLS 1.2 cipher list deliberately
- avoid insecure skip-verification settings

## Tests

- minimum version rejects TLS versions below the policy
- configured TLS 1.2 suites match the approved list
- weak suites are absent
- TLS configuration does not disable normal TLS 1.3 support

## Focused Check

```bash
go test ./internal/app
```

## Suggested Commit

```text
feat: define secure TLS policy
```

---

# Phase 3 — Harden The HTTP Server With Timeouts

## Learn

- understand slow-client and Slowloris-style resource exhaustion
- understand the difference between header, body, response, and keep-alive
  timeouts
- balance DoS protection with legitimate 20 MiB image uploads

## Initial Timeout Decision

Start with explicit, documented values such as:

```text
ReadHeaderTimeout: 5 seconds
ReadTimeout:        2 minutes
WriteTimeout:       2 minutes
IdleTimeout:        60 seconds
MaxHeaderBytes:     1 MiB
```

Review these values during integration testing. Do not choose a very short body
timeout that breaks valid uploads.

## Tasks

- create a testable helper that constructs `http.Server`
- set `Addr`, `Handler`, and `TLSConfig`
- set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`
- set a reasonable maximum header size
- retain the existing graceful shutdown behavior

## Tests

- every required timeout is non-zero and equals the chosen policy
- the TLS configuration is attached to the server
- the handler and address are preserved
- graceful shutdown still treats `http.ErrServerClosed` as expected

## Focused Check

```bash
go test ./internal/app
```

## Suggested Commit

```text
feat: configure secure server timeouts
```

---

# Phase 4 — Start And Stop The HTTPS Server Safely

## Learn

- understand the certificate/public-key and private-key pair
- understand how `ListenAndServeTLS` differs from `ListenAndServe`
- understand synchronous startup validation versus goroutine error reporting

## Tasks

- load or validate the certificate/key pair before reporting successful startup
- start the configured server with TLS when HTTPS is enabled
- keep explicit HTTP startup only for the selected development/test mode
- propagate bind, certificate, TLS, and serving failures clearly
- preserve signal-driven graceful shutdown
- ensure the server goroutine cannot block shutdown error handling

## Tests

- a valid temporary certificate/key pair can start an HTTPS test server
- a missing certificate fails startup clearly
- a missing key fails startup clearly
- a malformed certificate fails startup clearly
- a mismatched certificate/key pair fails startup clearly
- HTTPS requests succeed with a test client that trusts the test certificate
- shutdown completes without a leaked goroutine

Use temporary test certificates or fixtures that contain no real secret.

## Focused Check

```bash
go test ./internal/app
```

## Suggested Commit

```text
feat: serve forum over HTTPS
```

---

# Phase 5 — Generate Safe Local Certificates

## Learn

- understand why a self-signed certificate causes a browser trust warning
- understand Subject Alternative Names
- understand why the private key must stay secret

## Tasks

- add a small OpenSSL script or exact documented command
- generate a certificate with SAN entries for `localhost` and `127.0.0.1`
- use predictable local paths such as:

```text
certs/localhost.crt
certs/localhost.key
```

- give the private key restrictive permissions such as `0600`
- ignore generated certificates and keys in Git and Docker build context as
  appropriate
- do not overwrite an existing private key silently
- document the expected browser warning for an untrusted self-signed
  certificate
- never commit generated private keys

## Acceptance

- a new developer can generate a usable local certificate from the README
- the certificate is valid for `localhost`
- `git status` does not show generated certificate/private-key files
- the application starts with the generated pair

## Suggested Commit

```text
docs: add safe local certificate generation
```

---

# Phase 6 — Enforce Secure Session Cookies On HTTPS

## Learn

- understand `Secure`, `HttpOnly`, and `SameSite`
- understand why the cookie contains only an opaque identifier
- understand why session state remains in SQLite

## Policy

HTTPS mode must never create an authentication or OAuth state cookie without
the `Secure` flag.

A safe derived rule is:

```text
effective secure cookie = HTTPS enabled OR explicitly configured secure cookie
```

This keeps direct HTTPS safe and still permits secure cookies behind a trusted
TLS-terminating proxy.

## Tasks

- derive secure-cookie behavior from the validated configuration
- apply the rule to forum session cookies
- apply the same rule to OAuth state cookies
- preserve `HttpOnly`, `SameSite=Lax`, path, expiration, and logout clearing
- keep the cookie value as a UUID only
- keep server-side expiry checks and session replacement unchanged

## Tests

- HTTPS always produces a `Secure` session cookie
- HTTPS always produces secure OAuth state cookies
- cookie remains `HttpOnly` and `SameSite=Lax`
- cookie value parses as a UUID
- two newly created sessions have different UUIDs
- malformed UUID cookies do not authenticate
- expired sessions do not authenticate
- logout invalidates the server-side session
- a replacement login invalidates the previous session
- cookie contains no email, username, password, or serialized user state

## Focused Check

```bash
go test ./internal/config ./internal/session ./internal/oauth ./internal/web
```

## Suggested Commit

```text
fix: enforce secure cookies for HTTPS
```

---

# Phase 7 — Build The Rate Limiter Core

## Learn

- understand rate versus burst
- understand per-client state
- understand mutex protection, goroutine lifecycle, channels, and stale-state
  cleanup
- understand why tests should use a controllable clock instead of sleeping

## Design Decisions

- use only standard-library packages
- use a token-bucket or similarly defensible algorithm
- key buckets by normalized client IP and rule name
- obtain the direct peer IP from `Request.RemoteAddr`
- parse IPv4 and IPv6 safely with `net.SplitHostPort`
- do not trust `X-Forwarded-For` unless a future trusted-proxy configuration is
  explicitly added
- protect shared state for concurrent HTTP handlers
- inject or abstract time so tests are deterministic
- expire inactive entries so the map cannot grow forever
- stop any cleanup goroutine when the application shuts down

## Tasks

- implement the limiter independently of HTTP first
- define rate, burst, and inactivity expiry
- implement deterministic refill/reset behavior
- implement stale-entry cleanup
- implement an idempotent lifecycle stop operation if a goroutine is used

## Tests

- requests/tokens within the allowance succeed
- the next request/token is rejected
- capacity refills after simulated time advances
- different client keys are independent
- different rule keys are independent
- stale entries are removed
- stopping cleanup more than once is safe
- concurrent access passes under the race detector

## Focused Checks

```bash
go test ./internal/web/middleware
go test -race ./internal/web/middleware
```

## Suggested Commit

```text
feat: add concurrent rate limiter core
```

---

# Phase 8 — Apply HTTP Rate-Limiting Rules

## Learn

- understand middleware ordering
- understand why login failures and successes must both consume allowance
- understand the difference between general DoS limiting and brute-force
  limiting

## Initial Rules

Use conservative starting values and adjust only with evidence:

```text
Global:        120 requests/minute/IP
Login POST:      5 requests/minute/IP
Register POST:   5 requests/minute/IP
Post writes:    20 requests/minute/IP
Comment writes: 30 requests/minute/IP
Reactions:      60 requests/minute/IP
```

Define whether the values represent a fixed window or token rate/burst in the
documentation. Avoid counting one request twice against the same rule by
accident.

## Tasks

- add HTTP middleware around the existing router/handlers
- apply a general per-IP rule
- apply stricter rules to login and registration POST requests
- apply suitable write limits to posts, comments, and reactions
- ensure successful and failed login attempts both count
- return `429 Too Many Requests` when blocked
- include a valid `Retry-After` header where the algorithm can calculate it
- keep the response generic and avoid revealing whether an account exists
- decide explicitly how static assets are treated so one page load is not
  unexpectedly blocked
- connect limiter cleanup to application shutdown
- keep request logging and panic recovery effective for `429` responses

## Tests

- ordinary browsing stays below the general limit
- repeated login attempts receive `429`
- successful and failed login attempts both count
- registration has an independent stricter rule
- write routes use their intended rules
- separate IPs have separate allowance
- IPv4 and IPv6 peer addresses are normalized correctly
- malformed `RemoteAddr` is handled safely
- forged forwarding headers do not bypass the direct-peer policy
- blocked responses contain no account information
- `Retry-After` is valid when present
- existing login, registration, posting, comment, reaction, static, and OAuth
  tests still pass

## Focused Checks

```bash
go test ./internal/web/middleware ./internal/web
go test -race ./internal/web/middleware ./internal/web
```

## Suggested Commit

```text
feat: enforce per-client HTTP rate limits
```

---

# Phase 9 — Make HTTPS Work In Docker

## Learn

- understand build-time files versus runtime secrets
- understand read-only certificate mounts
- understand container port mappings and environment configuration

## Tasks

- configure the container for the chosen HTTPS port
- provide certificate and key paths through environment variables
- mount the local certificate directory read-only rather than baking a private
  key into the image
- set secure-cookie behavior for the HTTPS container
- change local OAuth callback examples to `https://`
- preserve the database and upload named volumes
- verify the non-root container user can read mounted certificate files without
  making the private key public
- keep certificate keys out of the Docker build context and image layers

## Acceptance

- `docker compose up --build` serves an HTTPS URL
- the browser receives a secure UUID session cookie
- database and uploaded images survive container replacement
- no private key exists in Git history or the built image
- OAuth configuration examples use the actual HTTPS callback URL

## Suggested Commit

```text
chore: run forum HTTPS in Docker
```

---

# Phase 10 — Review Errors And Technical Failures

## Learn

- distinguish safe browser errors from detailed operator logs
- understand which failures happen before the HTTP server can respond

## Tasks

- review certificate, key, bind, serve, and shutdown errors
- review malformed request and rate-limit responses
- preserve generic browser responses for internal database/filesystem failures
- preserve recovery from handler panics
- ensure logs do not expose passwords, cookies, OAuth secrets, private keys,
  request bodies, SQL values, or certificate contents
- verify expected statuses including `400`, `401`, `404`, `405`, `429`, and
  `500`

## Tests

- invalid startup inputs fail with useful but non-secret errors
- representative internal errors return a generic `500`
- rate-limit rejection returns `429`
- malformed requests do not crash the server
- a recovered panic does not stop later requests
- request logging records the final status without logging secrets

## Suggested Commit

```text
test: verify safe security error handling
```

---

# Phase 11 — Optional Additional Hardening

Complete this phase only after Phases 0–10 pass.

## Part A — Basic Security Headers

Consider global middleware for:

```text
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: same-origin
```

Test headers on HTML, errors, and static responses.

Treat these separately:

- enable CSP only after verifying templates, CSS, images, and OAuth flows
- enable HSTS only for an intentional HTTPS deployment with a trusted
  certificate; avoid trapping local self-signed testing in a cached HSTS policy

Suggested commit:

```text
feat: add basic security response headers
```

## Part B — Disable Static Directory Listings

- keep valid `/static/...` files public
- keep uploaded post images visible to guests
- return a safe response for `/static/` and `/static/uploads/` directories
- do not break CSS or existing image URLs

Suggested commit:

```text
fix: disable static directory listings
```

## Part C — Optional HTTP Redirect

If implemented:

- run it on a separate configured HTTP address
- redirect only to the configured HTTPS origin
- avoid constructing redirect hosts from an untrusted `Host` header
- shut both servers down cleanly
- test query strings and paths

Suggested commit:

```text
feat: redirect configured HTTP traffic to HTTPS
```

---

# Phase 12 — Automated Verification Gate

## Required Commands

Run and fix every relevant failure:

```bash
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...
go build ./...
docker compose build
```

Do not format unrelated user files. Review the final diff after formatting.

## Dependency Check

- inspect `go.mod` and `go.sum`
- confirm security work added no third-party dependency
- confirm imports remain standard library plus the exercise-allowed SQLite,
  bcrypt, and UUID packages already used by the project

## Regression Check

- registration and bcrypt login work
- logout works
- GitHub and Google OAuth regression tests pass
- UUID session replacement works
- public forum reading works
- posts, comments, reactions, and filters work
- text-only and image posts work
- guests can view uploaded images
- database and uploads remain persistent in Docker

## Suggested Commit

```text
test: complete security regression coverage
```

---

# Phase 13 — Documentation And Manual Audit Rehearsal

## README Requirements

Document:

- prerequisites, including OpenSSL and Docker if used
- certificate generation
- every new environment variable
- the exact local HTTPS startup command
- the exact HTTPS URL and expected self-signed warning
- Docker HTTPS startup
- TLS version and cipher-suite policy
- server timeout policy
- rate-limit algorithm, client key, limits, and `429` behavior
- secure UUID/server-side session behavior
- bcrypt password storage
- how to run all verification commands
- which database-encryption work is bonus and not implemented

Documentation must describe the tested implementation, not an intended future
state.

## Manual Audit Walkthrough

### HTTPS and TLS

- start the forum with the documented certificate configuration
- open the documented `https://` URL
- verify HTTPS is negotiated
- verify obsolete TLS versions are rejected
- explain the TLS 1.2 cipher list and Go-managed TLS 1.3 suites
- point to the configured server timeouts

### Rate limiting

- send normal requests successfully
- exceed a configured limit
- observe `429 Too Many Requests`
- wait/refill as documented and observe requests succeed again
- explain per-IP state, synchronization, cleanup, and shutdown

### Passwords

- register a test user
- inspect SQLite:

```sql
SELECT id, email, username, password_hash FROM users;
```

- confirm the plaintext password is absent
- confirm the stored value is a bcrypt hash beginning with a bcrypt prefix such
  as `$2a$` or `$2b$`

### Sessions and cookies

- log in and inspect the session cookie in browser developer tools
- confirm the value is a UUID
- confirm `Secure`, `HttpOnly`, and `SameSite=Lax`
- confirm no user data exists in the cookie
- confirm the matching session state is stored in SQLite
- log out and confirm the session no longer authenticates

### Configuration and failures

- show how certificate and key paths are configured
- demonstrate that a missing or invalid pair fails safely
- confirm browser-facing errors do not expose internal details
- show that only allowed packages are used

### General regression

- browse as a guest
- register and log in
- create a text post and an image post
- comment and react
- verify public images still load
- restart the container and verify persistent content remains

## Suggested Commit

```text
docs: add reproducible security audit guide
```

---

# Final Audit Traceability Checklist

| Audit question | Required proof |
|---|---|
| Does the URL contain HTTPS? | Running documented `https://` URL |
| Are cipher suites implemented? | Explicit TLS 1.2 policy plus documented Go-managed TLS 1.3 behavior |
| Is Go TLS configured well? | Tested `tls.Config`, minimum version, and valid certificate loading |
| Are server timeouts reduced? | Non-zero read-header, read, write, and idle timeout tests |
| Is rate limiting implemented? | Per-IP limiter and reproducible `429` response |
| Are passwords protected? | bcrypt code/tests and SQLite inspection showing no plaintext |
| Is the session cookie a UUID? | Cookie inspection and UUID tests |
| Is session state server-side? | SQLite session row and opaque cookie value |
| Are certificate settings configurable? | Environment configuration and startup validation |
| Are packages allowed? | Reviewed `go.mod`, imports, and no new security dependency |
| Are errors handled? | Startup, middleware, recovery, and safe-error tests |
| Does the project avoid crashes/leaks? | Full tests, race tests, clean goroutine shutdown, and manual run |
| Does existing forum behavior remain? | Full regression suite and manual forum walkthrough |

## Definition Of Done

- every mandatory audit row has implementation, automated proof, and a manual
  demonstration
- all focused tests and the full regression suite pass
- the race detector passes
- the application builds and runs locally over HTTPS
- the Docker deployment runs over HTTPS
- secure session cookies are enforced in HTTPS mode
- rate limiting returns `429` and cleans up safely
- private keys, environment secrets, databases, logs, and uploads are not
  committed
- documentation matches the commands actually tested
- each commit is small enough to explain without combining unrelated controls
- optional hardening is clearly separated from mandatory passing work
- database encryption remains deferred unless all mandatory work is complete
