-- +goose Up
SELECT 'up SQL query';

CREATE TABLE users(
    id          VARCHAR(255) PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    deleted_at  DATETIME
);

CREATE TABLE movies (
    id         VARCHAR(255) PRIMARY KEY,
    owner_id   VARCHAR(255) NOT NULL,
    title      TEXT NOT NULL,
    status     VARCHAR(255) NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME,

    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE TABLE sessions(
    id                  VARCHAR(255) PRIMARY KEY,
    created_at          DATETIME NOT NULL,
    updated_at          DATETIME NOT NULL,
    closed_at           DATETIME,
    deleted_at          DATETIME,
    winner_id           VARCHAR(255),
    watched_movie_id    VARCHAR(255),

    FOREIGN KEY (winner_id) REFERENCES users(id) ON DELETE RESTRICT
    FOREIGN KEY (watched_movie_id) REFERENCES movies(id) ON DELETE RESTRICT
);

CREATE TABLE participants(
    id          VARCHAR(255) PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL,
    session_id  VARCHAR(255) NOT NULL,
    created_at  DATETIME NOT NULL,

    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,

    UNIQUE(session_id, user_id)
);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE participants;
DROP TABLE sessions;
DROP TABLE movies;
DROP TABLE users;
