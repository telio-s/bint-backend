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
|   |-- migrations/              # Ordered database schema migrations
|   `-- query/                   # SQL query definitions used for data access/code generation
|-- internal/
|   |-- adapter/
|   |   |-- http/
|   |   |   |-- dto/             # HTTP request and response data-transfer objects
|   |   |   `-- handler/         # HTTP endpoint handlers and validation
|   |   `-- postgres/             # PostgreSQL repositories and transaction implementations
|   |-- docs/                    # API documentation and OpenAPI specifications
|   |-- domain/
|   |   |-- apperror/            # Domain-specific errors
|   |   |-- model/               # Core business entities and value objects
|   |   `-- service/             # Application and business use cases
|   |-- infra/
|   |   `-- config/              # Environment and application configuration
|   `-- port/                    # Interfaces implemented or consumed by the application
|-- AGENTS.md                    # Contributor and coding-agent guidance
`-- go.mod                       # Go module definition
```

## Placement Guidelines

- Put executable startup code in `cmd/api`; keep business logic out of it.
- Put transport-specific parsing, validation, and response mapping in
  `internal/adapter/http`.
- Put PostgreSQL-specific persistence code in `internal/adapter/postgres`.
- Put business entities and rules in `internal/domain` without importing HTTP,
  database, or configuration packages.
- Define repository, service, and transaction boundaries as interfaces in
  `internal/port`.
- Add schema changes as migrations in `db/migrations` and reusable SQL in
  `db/query`.
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
