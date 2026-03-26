package key

import "fmt"

func BoardList(limit int, lastID int64) string {
	return fmt.Sprintf("boards:list:limit:%d:last:%d", limit, lastID)
}

func BoardListPrefix() string {
	return "boards:list:"
}

func PostList(boardID int64, limit int, lastID int64) string {
	return fmt.Sprintf("posts:list:board:%d:limit:%d:last:%d", boardID, limit, lastID)
}

func PostListPrefix(boardID int64) string {
	return fmt.Sprintf("posts:list:board:%d:", boardID)
}

func PostDetail(postID int64) string {
	return fmt.Sprintf("posts:detail:%d", postID)
}

func PostDetailPrefix() string {
	return "posts:detail:"
}

func TagPostList(tagName string, limit int, lastID int64) string {
	return fmt.Sprintf("tags:posts:name:%s:limit:%d:last:%d", tagName, limit, lastID)
}

func TagPostListPrefix(tagName string) string {
	return fmt.Sprintf("tags:posts:name:%s:", tagName)
}

func TagPostListGlobalPrefix() string {
	return "tags:posts:name:"
}

func PostSearchList(query string, limit int, cursor string) string {
	return fmt.Sprintf("posts:search:q:%s:limit:%d:cursor:%s", query, limit, cursor)
}

func PostSearchSortedList(query, sortBy, window string, limit int, cursor string) string {
	return fmt.Sprintf("posts:search:q:%s:sort:%s:window:%s:limit:%d:cursor:%s", query, sortBy, window, limit, cursor)
}

func PostSearchListPrefix() string {
	return "posts:search:"
}

func PostFeedList(sortBy, window string, limit int, cursor string) string {
	return fmt.Sprintf("posts:feed:sort:%s:window:%s:limit:%d:cursor:%s", sortBy, window, limit, cursor)
}

func PostFeedListPrefix() string {
	return "posts:feed:"
}

func RankedPostList(boardID int64, sortBy, window string, limit int, cursor string) string {
	return fmt.Sprintf("posts:ranked:list:board:%d:sort:%s:window:%s:limit:%d:cursor:%s", boardID, sortBy, window, limit, cursor)
}

func RankedTagPostList(tagName, sortBy, window string, limit int, cursor string) string {
	return fmt.Sprintf("tags:ranked:posts:name:%s:sort:%s:window:%s:limit:%d:cursor:%s", tagName, sortBy, window, limit, cursor)
}

func RankedPostListPrefix(boardID int64) string {
	return fmt.Sprintf("posts:ranked:list:board:%d:", boardID)
}

func RankedPostListGlobalPrefix() string {
	return "posts:ranked:list:"
}

func RankedTagPostListPrefix(tagName string) string {
	return fmt.Sprintf("tags:ranked:posts:name:%s:", tagName)
}

func RankedTagPostListGlobalPrefix() string {
	return "tags:ranked:posts:name:"
}

func CommentList(postID int64, limit int, lastID int64) string {
	return fmt.Sprintf("comments:list:post:%d:limit:%d:last:%d", postID, limit, lastID)
}

func CommentListPrefix(postID int64) string {
	return fmt.Sprintf("comments:list:post:%d:", postID)
}

func CommentListGlobalPrefix() string {
	return "comments:list:post:"
}

func ReactionList(targetType string, targetID int64) string {
	return fmt.Sprintf("reactions:list:%s:%d", targetType, targetID)
}

func ReactionListPrefix() string {
	return "reactions:list:"
}
