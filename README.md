# Bint Backend

Go backend for Bint using Gin, Fx, PostgreSQL, and sqlc.

## Local setup

1. Copy `.env.example` to `.env` and set a random `JWT_SECRET` plus Google OAuth
   web-client credentials.
2. In Google Cloud, register
   `http://localhost:8080/api/v1/auth/google/callback` as an authorized redirect
   URI, or use the value configured in `GOOGLE_REDIRECT_URL`.
3. Start PostgreSQL with `docker compose up -d`.
4. Apply the SQL files in `db/migrations` using your migration tool. The API
   does not run migrations automatically. `db/schema.sql` contains the
   canonical current schema used by sqlc and schema-inspection tools.
5. Run `go run ./cmd/api`.

The OpenAPI document is available at `http://localhost:8080/openapi.yaml` and
the Scalar UI at `http://localhost:8080/docs`.

## Development

```sh
make sqlc
make fmt
make test
```

`make sqlc` uses the pinned sqlc version from the Makefile and writes generated
code to `internal/adapter/postgres/sqlc`.
