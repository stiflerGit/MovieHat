# MovieHat

MovieHat is a Go backend for group movie nights.
It helps a group manage users and movie lists, run movie-picking sessions, and fairly choose who gets to decide.

## Table of Contents

- [About The Project](#about-the-project)
- [Built With](#built-with)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [HTTPS and reverse proxies](#https-and-reverse-proxies)
- [Usage](#usage)
- [Project Structure](#project-structure)

## About The Project

MovieHat is currently an early MVP, but the main backend flows are already in place.

What is implemented in this repository:

- create invitation links and sign up with them
- sign in and sign out with token-based auth
- bootstrap the first user for local development
- list users, update the current user, and delete the current user
- add, list, and delete user movies (searched via TMDB API)
- create, list, get, and delete sessions
- add, remove, and list session participants
- end a session and extract a winner with a weighted fair-share algorithm
- set the watched movie for a closed session as that session's winner
- run embedded SQLite migrations at startup
- build and run the service in Docker
- expose the standard gRPC health service

Current caveats:

- sign-up is invite-only
- all authenticated users can currently manage shared session state
- user deletion is best-effort across MovieHat and auth storage
- list endpoints are not paginated yet

## Built With

- [Go](https://go.dev/)
- [ConnectRPC](https://connectrpc.com/)
- [SQLite](https://www.sqlite.org/) via `modernc.org/sqlite`
- [Goose](https://github.com/pressly/goose)
- [Buf](https://buf.build/) for protobuf management
- [TMDB API](https://developer.themoviedb.org/) for movie search

## Getting Started

### Prerequisites

- Go `1.26.4` or newer
- Docker (optional)
- A TMDB API Read Access Token (free, [sign up](https://www.themoviedb.org/signup))

### Installation

Set your TMDB token and run:

```bash
export TMDB_TOKEN="your-tmdb-read-access-token"
make run
```

That command starts the server with local-development defaults, including bootstrap of the first user:

| Variable | Default |
|---|---|
| `ADDR` | `0.0.0.0:8080` |
| `DB_PATH` | `./moviehat.db` |
| `AUTH_SECRET` | `dev-secret` |
| `BOOTSTRAP_ENABLED` | `true` |
| `BOOTSTRAP_EMAIL` | `admin@moviehat.com` |
| `BOOTSTRAP_PASSWORD` | `moviehat` |
| `TMDB_TOKEN` | *(from environment — must be set manually)* |

If you prefer running it directly:

```bash
TMDB_TOKEN=your-tmdb-token \
AUTH_SECRET=dev-secret \
BOOTSTRAP_ENABLED=true \
BOOTSTRAP_EMAIL=admin@moviehat.com \
BOOTSTRAP_PASSWORD=moviehat \
go run ./cmd/moviehat
```

### Configuration

The server reads configuration from environment variables.
`.env.example` is a reference file only; it is not loaded automatically.

Required:

- `AUTH_SECRET`
- `TMDB_TOKEN` — API Read Access Token from TMDB

Optional:

- `ADDR` — default `0.0.0.0:8080`
- `DB_PATH` — default `./moviehat.db`
- `FRONTEND_INVITATION_URL` — default `https://moviehat.app/invite`
- `BOOTSTRAP_ENABLED` — default `false`
- `BOOTSTRAP_EMAIL` — required when `BOOTSTRAP_ENABLED=true`
- `BOOTSTRAP_PASSWORD` — required when `BOOTSTRAP_ENABLED=true`

### HTTPS and reverse proxies

MovieHat listens on plain HTTP internally by default (`0.0.0.0:8080`).
That is convenient for local development, but for public exposure you should run MovieHat behind an HTTPS reverse proxy such as Traefik, Caddy, or Nginx.

Recommended setup:

- terminate TLS at the reverse proxy on port `443`
- optionally expose port `80` only to redirect HTTP to HTTPS and/or satisfy ACME certificate challenges
- proxy requests from the reverse proxy to MovieHat on a private/local address such as `127.0.0.1:8080`
- do **not** expose MovieHat's backend port `8080` directly to the internet
- preserve the original `Host` header and send `X-Forwarded-Proto: https`

Protocol note:

- MovieHat enables HTTP/1 and h2c internally
- if you use gRPC or other HTTP/2 clients through a reverse proxy, make sure the proxy supports forwarding to an h2c upstream

Local development can continue to use plain HTTP on `localhost:8080` without a reverse proxy.

## Usage

Run the test suite:

```bash
make test
```

Run `make help` to get a list of available commands (auto-generated from Makefile annotations).

Notes:
- local runs listen on `0.0.0.0:8080` by default
- local runs store data in `./moviehat.db` by default
- Docker image builds are supported through `make docker-build`
- `make docker-run` is best used with an already-seeded volume; for a fresh volume you should pass bootstrap variables explicitly

Example Docker run for a fresh volume:

```bash
docker run --rm \
  -p 8080:8080 \
  -e AUTH_SECRET=dev-secret \
  -e TMDB_TOKEN=your-tmdb-token \
  -e BOOTSTRAP_ENABLED=true \
  -e BOOTSTRAP_EMAIL=admin@moviehat.com \
  -e BOOTSTRAP_PASSWORD=moviehat \
  -v moviehat-data:/data \
  moviehat:dev
```

## Regenerating code

- **Protobuf**: `make proto-generate` — generates Go code from `proto/` into `api/`
- **TMDB client**: `make oas-generate` — generates Go client from `third_party/tmdb/api.json` into `gen/tmdb/`

Run `make help` for the full list.

## Project Structure

```text
cmd/moviehat/                  application entrypoint

proto/                         protobuf definitions (your service schema)
api/                           generated protobuf / ConnectRPC code

third_party/                   external specs and definitions you don't own
  tmdb/api.json                 TMDB OpenAPI specification

gen/                           generated Go code from external specs
  tmdb/client.gen.go            TMDB API client (from oapi-codegen)

pkg/                           hand-written, reusable Go packages
  math/                         normalization utilities
  sql/tx/                       generic DB transaction manager

internal/                      app-specific code, not importable outside the module
  core/                         domain logic (users, sessions, movies, probabilities)
  auth/                         authentication handler + persistence
  gateway/v1/                   transport adapter (ConnectRPC → domain)
  moviesearch/                  movie search abstraction layer
    handler.go                   maps domain types to protobuf types
    types.go                     provider interface and domain types
    provider/tmdb/               TMDB provider implementation
  extractor/weighted/           weighted fair-share extraction engine
  extractor/fair_share/         legacy standalone extractor
  migrations/                   embedded SQL migration files

build/docker/                  container build files
tools/                         tool-only Go module (buf, oapi-codegen, etc.)
tests/                         integration tests
```