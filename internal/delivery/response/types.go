package response

import "time"

type Board struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type BoardList struct {
	Boards     []Board `json:"boards"`
	Limit      int     `json:"limit"`
	LastID     int64   `json:"last_id"`
	HasMore    bool    `json:"has_more"`
	NextLastID *int64  `json:"next_last_id,omitempty"`
}

type Post struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	AuthorUUID string    `json:"author_uuid"`
	BoardID    int64     `json:"board_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PostList struct {
	Posts      []Post `json:"posts"`
	Limit      int    `json:"limit"`
	LastID     int64  `json:"last_id"`
	HasMore    bool   `json:"has_more"`
	NextLastID *int64 `json:"next_last_id,omitempty"`
}

type PostDetail struct {
	Post            *Post           `json:"post"`
	Tags            []Tag           `json:"tags"`
	Attachments     []Attachment    `json:"attachments"`
	Comments        []CommentDetail `json:"comments"`
	CommentsHasMore bool            `json:"comments_has_more"`
	Reactions       []Reaction      `json:"reactions"`
}

type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Comment struct {
	ID         int64     `json:"id"`
	Content    string    `json:"content"`
	AuthorUUID string    `json:"author_uuid"`
	PostID     int64     `json:"post_id"`
	ParentID   *int64    `json:"parent_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type CommentList struct {
	Comments   []Comment `json:"comments"`
	Limit      int       `json:"limit"`
	LastID     int64     `json:"last_id"`
	HasMore    bool      `json:"has_more"`
	NextLastID *int64    `json:"next_last_id,omitempty"`
}

type CommentDetail struct {
	Comment   *Comment   `json:"comment"`
	Reactions []Reaction `json:"reactions"`
}

type Reaction struct {
	ID         int64     `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   int64     `json:"target_id"`
	Type       string    `json:"type"`
	UserUUID   string    `json:"user_uuid"`
	CreatedAt  time.Time `json:"created_at"`
}

type Attachment struct {
	ID          int64     `json:"id"`
	PostID      int64     `json:"post_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	FileURL     string    `json:"file_url"`
	PreviewURL  string    `json:"preview_url"`
	CreatedAt   time.Time `json:"created_at"`
}
