CREATE TABLE github_sync_state
(
    repo_full_name              TEXT PRIMARY KEY,
    last_repo_updated_at        TIMESTAMPTZ,
    last_processed_issue_number INTEGER NOT NULL DEFAULT 0
);