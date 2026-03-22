CREATE TABLE chats
(
    id         BIGINT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE links
(
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT        NOT NULL UNIQUE,
    last_checked_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_links
(
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT      NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    link_id    BIGINT      NOT NULL REFERENCES links (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (chat_id, link_id)
);

CREATE TABLE tags
(
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_link_tags
(
    id           BIGSERIAL PRIMARY KEY,
    chat_link_id BIGINT NOT NULL REFERENCES chat_links (id) ON DELETE CASCADE,
    tag_id       BIGINT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    UNIQUE (chat_link_id, tag_id)
);