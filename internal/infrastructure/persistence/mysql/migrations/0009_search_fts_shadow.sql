CREATE TABLE IF NOT EXISTS post_search_fts_shadow (
    post_id BIGINT NOT NULL PRIMARY KEY,
    title LONGTEXT NOT NULL,
    content LONGTEXT NOT NULL,
    tags LONGTEXT NOT NULL
);
