# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project: MovieHat

MovieHat is a Go service that helps groups fairly choose a movie to watch together.

**Domain concepts:**
- **User**: A person with an auth account and a movie watchlist
- **Session**: A movie-selection round with participants. When ended, a winner is extracted.
- **Extractor**: An algorithm that selects a session winner fairly (weighted by past wins so everyone gets a turn)
- **Movie**: From TMDB; users add movies to their lists; winners set the watched movie
- **Authentication**: Session tokens + bcrypt passwords + invitation-only sign-up

**Tech stack:**
- **ConnectRPC** (HTTP/gRPC with protobuf, via `connectrpc.com/connect`)
- **Protobuf**: Definitions in `proto/`, generated bindings in `api/` (Buf build system)
- **SQLite** via `modernc.org/sqlite` (no CGo), migrations via `goose`
- **TMDB API**: OpenAPI spec in `third_party/tmdb/`, generated Go client in `gen/tmdb/` (via `make oas-generate`)

**Testing patterns:**
- Table-driven tests with shared assertion helpers
- Mock packages per domain via `go.uber.org/mock` (stored under each `persistence/mocks/`)
- Individual domain tests avoid database by mocking persistence interfaces
- Integration tests in `tests/` may use real SQLite

**Code generation:**
- `make proto-generate` — Buf-based protobuf + Connect code generation into `api/`
- `make oas-generate` — oapi-codegen from `third_party/tmdb/api.json` into `gen/tmdb/`

## Code modification policy

Do **not** modify business code unless the user explicitly includes the phrase:

```text
ALLOW CODE CHANGES
```

Do **not** modify test files unless the user explicitly includes either phrase:

```text
ALLOW TEST CHANGES
```

or:

```text
ALLOW CODE CHANGES
```

`ALLOW TEST CHANGES` permits changes to test files only. It does not permit changes to business code.

Business code includes, but is not limited to:

- application/domain logic
- handlers and services
- persistence/storage code
- migrations
- generated API code
- protobuf definitions
- tests that change expectations around business behavior
- configuration or startup code that affects runtime behavior

If the user asks for a review, brainstorming, diagnosis, test fixing, refactoring, or implementation but does **not** include the required allow phrase, you must not edit those files. Instead, explain what you would change and ask for explicit permission.

## Allowed without `ALLOW CODE CHANGES`

You may perform read-only actions, such as:

- inspect files
- run tests or linters
- run search commands
- provide code review feedback
- propose patches in prose

You may create or update documentation-only instruction files when explicitly requested, such as this `AGENTS.md`.

## When in doubt

Ask before editing.
