-- +goose Up
SELECT 'up SQL query';

CREATE TABLE auth_users (
    id                  VARCHAR(255) NOT NULL PRIMARY KEY,
    email               VARCHAR(255) NOT NULL UNIQUE,
    hashed_password     CHAR(60) NOT NULL,
    created_at          DATETIME NOT NULL
);

CREATE UNIQUE INDEX idx_auth_users_email ON auth_users (email);

CREATE TABLE auth_sessions (
    id          VARCHAR(255) NOT NULL PRIMARY KEY,
	token       VARCHAR(255) NOT NULL,
	user_id     VARCHAR(255) NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at  DATETIME,
	last_access DATETIME,

	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE UNIQUE INDEX idx_auth_sessions_token ON auth_sessions (token);
CREATE INDEX idx_auth_sessions_user_id ON auth_sessions (user_id);

CREATE TABLE auth_verifications (
    token_hash  VARCHAR(255) NOT NULL PRIMARY KEY,
    expires_at  DATETIME NOT NULL,
    max_uses    INTEGER NOT NULL,
    uses_count  INTEGER NOT NULL DEFAULT 0,
    author_id   VARCHAR(255),

    FOREIGN KEY (author_id) REFERENCES auth_users(id) ON DELETE SET NULL
);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE auth_verifications;
DROP TABLE auth_sessions;
DROP TABLE auth_users;
