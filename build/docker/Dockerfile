FROM golang:1.26 AS build

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/moviehat ./cmd/moviehat

FROM debian:bookworm-slim

RUN useradd --system --create-home --home-dir /app appuser \
    && mkdir -p /data \
    && chown -R appuser:appuser /app /data

WORKDIR /app
ENV ADDR=0.0.0.0:8080
ENV DB_PATH=/data/moviehat.db
VOLUME ["/data"]

COPY --from=build /out/moviehat /usr/local/bin/moviehat

USER appuser
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/moviehat"]
