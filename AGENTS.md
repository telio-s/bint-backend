# Bint Backend

This directory contains the Go backend for Bint. The module path is
`github.com/telio-s/bint-backend.git` and the project targets Go 1.27.1.

## Architecture

The project follows a ports-and-adapters (hexagonal) structure. Business rules
belong in the domain layer, interfaces that define external dependencies belong
in the port layer, and concrete HTTP or database implementations belong in the
adapter layer. Keep dependencies pointing inward: adapters may depend on ports
and domain types, while the domain must not depend on adapters.

## Project Structure

```text
bint-backend/
|-- cmd/
|   `-- api/                     # API executable entry point and dependency wiring
|-- db/
|   |-- schema.sql               # Canonical current database schema
|   |-- migrations/              # Ordered database schema migrations
|   `-- query/                   # SQL query definitions used for data access/code generation
|-- internal/
|   |-- adapter/
|   |   |-- http/
|   |   |   |-- dto/             # HTTP request and response data-transfer objects
|   |   |   |-- handler/         # HTTP endpoint handlers and validation
|   |   |   `-- middleware/      # Gin request logging and cross-cutting HTTP behavior
|   |   |-- oauth/                # External identity-provider adapters
|   |   |-- postgres/             # PostgreSQL repositories and sqlc-generated code
|   |   `-- security/             # Password hashing and Bint JWT implementation
|   |-- docs/                    # API documentation and OpenAPI specifications
|   |-- domain/
|   |   |-- apperror/            # Domain-specific errors
|   |   |-- model/               # Core business entities and value objects
|   |   `-- service/             # Use-case subpackages, such as service/auth
|   |-- infra/
|   |   `-- config/              # Environment and application configuration
|   `-- port/                    # Interfaces implemented or consumed by the application
|-- pkg/
|   `-- httpjson/                # Public, framework-neutral JSON response envelopes
|-- AGENTS.md                    # Contributor and coding-agent guidance
`-- go.mod                       # Go module definition
```

## Placement Guidelines

- Put executable startup code in `cmd/api`; keep business logic out of it.
- Put transport-specific parsing, validation, and response mapping in
  `internal/adapter/http`.
- Put reusable public libraries that do not depend on Bint domain or adapters
  in `pkg`. Bint JSON APIs use `pkg/httpjson` to return `{code, message, data}`
  on success and `{code, message, error}` on failure.
- Inject `*httpjson.Responder` through Fx into HTTP routers and handlers. All
  JSON responses, including health, errors, 404/405 responses, and panic
  recovery, must use the responder; do not bypass it with `gin.Context.JSON`.
  Redirect, cookie, OpenAPI YAML, and Scalar HTML responses are not JSON and
  remain transport-specific.
- Put PostgreSQL-specific persistence code in `internal/adapter/postgres`.
- Put business entities and rules in `internal/domain` without importing HTTP,
  database, or configuration packages.
- Define repository, service, and transaction boundaries as interfaces in
  `internal/port`.
- Add schema changes as migrations in `db/migrations` and reusable SQL in
  `db/query`. Keep `db/schema.sql` synchronized with the resulting current
  schema; sqlc reads this canonical schema file.
- Keep API specifications and generated documentation assets in `internal/docs`.

## API Convention

- Use plural nouns for REST resources.
- Use kebab-case for multi-word paths.
- Do not use verbs in endpoint names unless the endpoint represents a domain command.
- Example:
  - `GET /orders`
  - `GET /orders/{orderId}`
  - `POST /orders`
  - `PATCH /orders/{orderId}`

## Development Conventions

- Format Go code with `gofmt`.
- Prefer small packages with explicit dependencies passed through constructors.
- Wrap errors with useful context and preserve the underlying error where
  callers may need `errors.Is` or `errors.As`.
- Keep tests beside the code they cover and favor table-driven tests when
  several cases share the same setup.
- Run `go test ./...` before submitting changes once the project contains Go
  source files.

## Authentication Architecture

Bint supports two ways for a user to prove their identity:

1. Google authentication uses OAuth 2.0 Authorization Code flow with OpenID
   Connect, PKCE, `state`, and `nonce`.
2. Email authentication verifies an email address and a bcrypt password stored
   by Bint.

Google authentication and JWT authentication solve different problems. Google
proves who the user is during the Google callback. A Bint JWT authorizes later
requests to the Bint API. OAuth does not require an application to use JWTs for
its own session, but Bint deliberately issues its own token pair after both
Google and email sign-in so every authenticated user has the same Bint session
behavior.

The three token types must not be confused:

- A Google ID token proves a Google identity. Verify it during the callback and
  do not use it as the Bint session token.
- A Google access token authorizes Google APIs. Bint currently does not retain
  it because login only needs the verified ID token.
- A Bint access token is a short-lived JWT issued by Bint and accepted by Bint
  API middleware.
- A Bint refresh token is an opaque random value. Only its SHA-256 hash is
  stored in PostgreSQL. The raw value is kept in an HttpOnly cookie and rotated
  by the refresh endpoint.

### Shared Bint Session Flow

Both login methods end with the same session flow:

1. The domain service obtains a trusted Bint user from either the email or
   Google identity flow.
2. Bint creates a short-lived access JWT.
3. Bint creates an opaque refresh token, stores only its hash, and sends the raw
   value in an HttpOnly cookie.
4. The client uses the access JWT, either as `Authorization: Bearer <token>` or
   through the HttpOnly access cookie, for authenticated requests.
5. When the access JWT expires, `POST /api/v1/auth/refresh` rotates the refresh
   token and issues a new access JWT. This refreshes Bint authentication; it
   does not refresh or call Google.

### Google Authentication Paths and Flow

- `GET /api/v1/auth/google` starts authentication. It creates temporary
  HttpOnly cookies for `state`, `nonce`, and the PKCE verifier, then redirects
  the browser to Google.
- `GET /api/v1/auth/google/callback` is the exact redirect URI registered in
  Google Cloud. It validates `state`, exchanges the one-time code using the
  PKCE verifier, verifies the Google ID-token signature, issuer, audience,
  expiration, and `nonce`, then identifies the account using Google's stable
  `sub` claim. A verified Google email may link to an existing Bint user; the
  Google `sub`, not the email, remains the provider identity key. The callback
  then issues the normal Bint token pair and redirects to the frontend.

Google login therefore uses Google only for the identity proof. After the
callback, ordinary Bint requests use Bint tokens and do not call Google.

### Email Authentication Paths and Flow

- `POST /api/v1/auth/email/register` validates the request, hashes the password
  with bcrypt, creates the user, and issues the Bint token pair.
- `POST /api/v1/auth/email/login` compares the submitted password with the
  stored bcrypt hash and issues the same Bint token pair.

Email authentication never contacts Google. It relies entirely on credentials
owned by Bint, but its authenticated requests and refresh behavior are the same
as for a Google-authenticated user.

### Shared Authentication Paths

- `POST /api/v1/auth/refresh` consumes and rotates the Bint refresh-token
  cookie, then returns a new Bint access JWT. It is needed because Bint uses
  short-lived access JWTs.
- `POST /api/v1/auth/logout` revokes the current Bint refresh token and clears
  the Bint cookies. It does not log the user out of Google or revoke Google
  consent.
- `GET /api/v1/auth/session` verifies the Bint access JWT and returns the
  current Bint user.

Password reset is intentionally out of scope for the first auth version.
TODO: add password-reset request and password-reset confirmation endpoints
without exposing whether an email exists.

## Authentication Data and Generation

- The canonical current schema lives in `db/schema.sql`. Auth migrations live
  in `db/migrations`. Keep both representations synchronized, but do not
  automatically apply migrations as part of code generation or startup.
- SQL query definitions live in `db/query`.
- Run `make sqlc` to generate PostgreSQL query code in
  `internal/adapter/postgres/sqlc`. The Makefile pins the sqlc version so local
  and CI output remain reproducible.
- Never edit files in `internal/adapter/postgres/sqlc` by hand.
- API documentation is embedded from `internal/docs/openapi.yaml` and rendered
  with Scalar at `GET /docs`.

## Configuration and Logging

- Environment variables are grouped in `internal/infra/config.Config` under
  application, server, database, and authentication configuration structs.
- Keep `.env.example` complete and safe to commit. `.env` is local-only and
  must remain ignored because it may contain secrets.
- Use `*slog.Logger` for application logging. Constructor functions must accept
  the shared logger and derive a component logger, for example
  `logger.With("component", "auth_service")`.
- Pass loggers inward through Fx dependency injection. Do not create package
  global loggers, and never log passwords, raw JWTs, OAuth codes, refresh
  tokens, client secrets, or database credentials.
