CREATE TABLE bookmarks (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    target_type text NOT NULL CHECK (
        target_type IN ('issue', 'repository')
    ),
    repository_owner varchar(39) NOT NULL CHECK (
        repository_owner = lower(repository_owner)
    ),
    repository_name varchar(100) NOT NULL,
    issue_number integer CHECK (issue_number > 0),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (target_type = 'issue' AND issue_number IS NOT NULL)
        OR (target_type = 'repository' AND issue_number IS NULL)
    )
);

CREATE UNIQUE INDEX bookmarks_repository_unique_idx
    ON bookmarks (
        account_id,
        repository_owner,
        lower(repository_name)
    )
    WHERE target_type = 'repository';

CREATE UNIQUE INDEX bookmarks_issue_unique_idx
    ON bookmarks (
        account_id,
        repository_owner,
        lower(repository_name),
        issue_number
    )
    WHERE target_type = 'issue';

CREATE INDEX bookmarks_account_order_idx
    ON bookmarks (account_id, created_at DESC, id DESC);

CREATE TABLE saved_searches (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    search_type text NOT NULL CHECK (
        search_type IN ('issue', 'repository')
    ),
    name varchar(80) NOT NULL CHECK (btrim(name) <> ''),
    filters jsonb NOT NULL CHECK (
        jsonb_typeof(filters) = 'object'
        AND octet_length(filters::text) <= 8192
    ),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX saved_searches_account_name_unique_idx
    ON saved_searches (account_id, lower(name));

CREATE INDEX saved_searches_account_order_idx
    ON saved_searches (account_id, updated_at DESC, id DESC);

CREATE TABLE user_preferences (
    account_id uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    theme text NOT NULL DEFAULT 'system'
        CHECK (theme IN ('light', 'dark', 'system')),
    reduced_motion text NOT NULL DEFAULT 'system'
        CHECK (reduced_motion IN ('reduce', 'no-preference', 'system')),
    results_per_page smallint NOT NULL DEFAULT 20
        CHECK (results_per_page IN (10, 20, 50)),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE privacy_audit_events (
    id uuid PRIMARY KEY,
    account_id uuid REFERENCES accounts(id) ON DELETE SET NULL,
    event_type text NOT NULL CHECK (
        event_type IN (
            'account_created',
            'account_deleted',
            'identity_linked',
            'session_revoked'
        )
    ),
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX privacy_audit_events_account_idx
    ON privacy_audit_events (account_id, occurred_at DESC)
    WHERE account_id IS NOT NULL;
