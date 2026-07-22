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
- add, list, and delete user movies
- create, list, get, and delete sessions
- add, remove, and list session participants
- end a session and extract a winner with the fair-share algorithm
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

## Getting Started

### Prerequisites

- Go `1.26.4` or newer
- Docker (optional)

### Installation

The quickest way to run the project locally is:

```bash
make run
```

That command starts the server with local-development defaults, including bootstrap of the first user:

- `ADDR=0.0.0.0:8080`
- `DB_PATH=./moviehat.db`
- `AUTH_SECRET=dev-secret`
- `BOOTSTRAP_ENABLED=true`
- `BOOTSTRAP_EMAIL=admin@moviehat.com`
- `BOOTSTRAP_PASSWORD=moviehat`

If you prefer running it directly:

```bash
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

Run `make help` to get a list of available commands.

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
  -e BOOTSTRAP_ENABLED=true \
  -e BOOTSTRAP_EMAIL=admin@moviehat.com \
  -e BOOTSTRAP_PASSWORD=moviehat \
  -v moviehat-data:/data \
  moviehat:dev
```

## Project Structure

```text
cmd/moviehat/          application entrypoint
internal/auth/         authentication logic and auth persistence
internal/moviehat/     user, movie, and session logic
internal/extractor/    fair-share extraction and score persistence
internal/migrations/   embedded SQL migrations
proto/                 protobuf definitions and Buf config
api/                   generated protobuf / Connect code
build/docker/          container build files
tools/                 tool-only Go module
```
