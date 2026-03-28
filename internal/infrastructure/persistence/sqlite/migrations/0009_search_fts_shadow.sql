CREATE VIRTUAL TABLE IF NOT EXISTS post_search_fts_shadow USING fts5(
    title,
    content,
    tags,
    tokenize = 'unicode61 remove_diacritics 2'
);
