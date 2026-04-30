package mysql

import "strings"

func rewriteQuery(query string, args ...any) (string, []any) {
	query = strings.TrimSpace(query)
	if query == "" {
		return query, args
	}
	if strings.Contains(query, "FROM post_search_fts") && strings.Contains(query, "MATCH") {
		return "SELECT post_id\nFROM post_search_fts\nORDER BY post_id ASC", nil
	}
	if strings.Contains(query, "post_search_fts") {
		query = strings.ReplaceAll(query, "rowid", "post_id")
	}
	query = strings.ReplaceAll(query, "INSERT OR IGNORE INTO tags", "INSERT IGNORE INTO tags")
	query = strings.ReplaceAll(query, "INSERT OR IGNORE INTO outbox_messages", "INSERT IGNORE INTO outbox_messages")
	query = strings.ReplaceAll(query, "ON CONFLICT(post_id, tag_id) DO UPDATE SET status = 'active'", "ON DUPLICATE KEY UPDATE status = 'active'")
	query = strings.ReplaceAll(query, "ON CONFLICT(token_hash) DO UPDATE SET\n    user_id = excluded.user_id,\n    created_at = excluded.created_at,\n    expires_at = excluded.expires_at,\n    consumed_at = excluded.consumed_at,\n    delivered_at = excluded.delivered_at", "ON DUPLICATE KEY UPDATE user_id = VALUES(user_id), created_at = VALUES(created_at), expires_at = VALUES(expires_at), consumed_at = VALUES(consumed_at), delivered_at = VALUES(delivered_at)")
	return query, args
}
