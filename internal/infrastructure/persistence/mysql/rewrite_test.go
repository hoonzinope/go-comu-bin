package mysql

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteQuery_RewritesSQLiteConflictAndIgnoreSyntax(t *testing.T) {
	t.Parallel()

	query, args := rewriteQuery(`INSERT OR IGNORE INTO tags (name, created_at)
VALUES (?, ?)
`)
	require.Empty(t, args)
	assert.Contains(t, query, "INSERT IGNORE INTO tags")

	query, args = rewriteQuery(`INSERT INTO post_tags (post_id, tag_id, created_at, status)
VALUES (?, ?, ?, 'active')
ON CONFLICT(post_id, tag_id) DO UPDATE SET status = 'active'
`)
	require.Empty(t, args)
	assert.Contains(t, query, "ON DUPLICATE KEY UPDATE")
	assert.NotContains(t, query, "ON CONFLICT")

	query, args = rewriteQuery(`INSERT INTO email_verification_tokens (
    token_hash, user_id, created_at, expires_at, consumed_at, delivered_at
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(token_hash) DO UPDATE SET
    user_id = excluded.user_id,
    created_at = excluded.created_at,
    expires_at = excluded.expires_at,
    consumed_at = excluded.consumed_at,
    delivered_at = excluded.delivered_at
`)
	require.Empty(t, args)
	assert.Contains(t, query, "ON DUPLICATE KEY UPDATE")
	assert.NotContains(t, query, "excluded.")
}

func TestRewriteQuery_RewritesPostSearchStatements(t *testing.T) {
	t.Parallel()

	query, args := rewriteQuery(`SELECT rowid
FROM post_search_fts
WHERE post_search_fts MATCH ?
ORDER BY rowid ASC
`, "search")
	require.Empty(t, args)
	assert.NotContains(t, strings.ToLower(query), "match")
	assert.Contains(t, strings.Join(strings.Fields(query), " "), "SELECT post_id FROM post_search_fts")

	query, args = rewriteQuery(`INSERT INTO post_search_fts (rowid, title, content, tags)
VALUES (?, ?, ?, ?)
`)
	require.Empty(t, args)
	assert.Contains(t, query, "post_id")
	assert.NotContains(t, query, "rowid")

	query, args = rewriteQuery(`DELETE FROM post_search_fts
WHERE rowid = ?
`, int64(123))
	require.Equal(t, []any{int64(123)}, args)
	assert.Contains(t, query, "post_id")
	assert.NotContains(t, query, "rowid")
}
