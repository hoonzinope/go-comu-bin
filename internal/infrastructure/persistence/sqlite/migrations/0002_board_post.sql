CREATE TABLE IF NOT EXISTS boards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    hidden INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_boards_hidden_id ON boards(hidden, id);

CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    author_id INTEGER NOT NULL,
    board_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    published_at INTEGER NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER NULL
);

CREATE INDEX IF NOT EXISTS idx_posts_board_status_id ON posts(board_id, status, id);
CREATE INDEX IF NOT EXISTS idx_posts_author_status_id ON posts(author_id, status, id);
CREATE INDEX IF NOT EXISTS idx_posts_status_id ON posts(status, id);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_post_tags_tag_status_post ON post_tags(tag_id, status, post_id);
CREATE INDEX IF NOT EXISTS idx_post_tags_post_status_tag ON post_tags(post_id, status, tag_id);
