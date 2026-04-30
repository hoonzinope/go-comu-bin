CREATE TABLE IF NOT EXISTS boards (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    hidden TINYINT(1) NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL
);

CREATE INDEX idx_boards_hidden_id ON boards(hidden, id);

CREATE TABLE IF NOT EXISTS tags (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    created_at BIGINT NOT NULL
);

CREATE INDEX idx_tags_name ON tags(name);

CREATE TABLE IF NOT EXISTS posts (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    content LONGTEXT NOT NULL,
    author_id BIGINT NOT NULL,
    board_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at BIGINT NOT NULL,
    published_at BIGINT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT NULL
);

CREATE INDEX idx_posts_board_status_id ON posts(board_id, status, id);
CREATE INDEX idx_posts_author_status_id ON posts(author_id, status, id);
CREATE INDEX idx_posts_status_id ON posts(status, id);

CREATE TABLE IF NOT EXISTS post_tags (
    post_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    PRIMARY KEY (post_id, tag_id)
);

CREATE INDEX idx_post_tags_tag_status_post ON post_tags(tag_id, status, post_id);
CREATE INDEX idx_post_tags_post_status_tag ON post_tags(post_id, status, tag_id);
