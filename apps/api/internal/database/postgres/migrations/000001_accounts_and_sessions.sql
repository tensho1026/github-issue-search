CREATE TABLE accounts (
    id uuid PRIMARY KEY,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'deleting')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (
        (status = 'active' AND deleted_at IS NULL)
        OR (status = 'deleting' AND deleted_at IS NOT NULL)
    )
);

CREATE TABLE github_identities (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    github_user_id bigint NOT NULL UNIQUE CHECK (github_user_id > 0),
    login varchar(39) NOT NULL CHECK (login = lower(login)),
    avatar_url text NOT NULL,
    profile_url text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id)
);

CREATE TABLE oauth_authorization_states (
    id uuid PRIMARY KEY,
    state_hash bytea NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
    return_path text NOT NULL CHECK (
        octet_length(return_path) BETWEEN 1 AND 2048
        AND return_path LIKE '/%'
        AND return_path NOT LIKE '//%'
    ),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at),
    CHECK (consumed_at IS NULL OR consumed_at >= created_at)
);

CREATE INDEX oauth_authorization_states_expiry_idx
    ON oauth_authorization_states (expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_secret_hash bytea NOT NULL CHECK (
        octet_length(csrf_secret_hash) = 32
    ),
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at),
    CHECK (last_seen_at >= created_at),
    CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX auth_sessions_account_active_idx
    ON auth_sessions (account_id, expires_at DESC)
    WHERE revoked_at IS NULL;

CREATE INDEX auth_sessions_expiry_idx
    ON auth_sessions (expires_at)
    WHERE revoked_at IS NULL;
