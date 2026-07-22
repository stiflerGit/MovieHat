-- +goose Up
SELECT 'up SQL query';

CREATE TABLE scores (
    user_id        TEXT NOT NULL,
    score          REAL NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (user_id)
);


-- +goose Down
SELECT 'down SQL query';
