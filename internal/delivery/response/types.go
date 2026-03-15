package response

import "time"

type Board struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type BoardList struct {
	Boards     []Board `json:"boards"`
	Limit      int     `json:"limit"`
	Cursor     string  `json:"cursor"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

type Post struct {
	UUID       string    `json:"uuid"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	AuthorUUID string    `json:"author_uuid"`
	BoardUUID  string    `json:"board_uuid"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PostList struct {
	Posts      []Post `json:"posts"`
	Limit      int    `json:"limit"`
	Cursor     string `json:"cursor"`
	HasMore    bool   `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
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
	UUID       string    `json:"uuid"`
	Content    string    `json:"content"`
	AuthorUUID string    `json:"author_uuid"`
	PostUUID   string    `json:"post_uuid"`
	ParentUUID *string   `json:"parent_uuid,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type CommentList struct {
	Comments   []Comment `json:"comments"`
	Limit      int       `json:"limit"`
	Cursor     string    `json:"cursor"`
	HasMore    bool      `json:"has_more"`
	NextCursor *string   `json:"next_cursor,omitempty"`
}

type CommentDetail struct {
	Comment   *Comment   `json:"comment"`
	Reactions []Reaction `json:"reactions"`
}

type Reaction struct {
	ID         int64     `json:"id"`
	TargetType string    `json:"target_type"`
	TargetUUID string    `json:"target_uuid"`
	Type       string    `json:"type"`
	UserUUID   string    `json:"user_uuid"`
	CreatedAt  time.Time `json:"created_at"`
}

type Attachment struct {
	UUID        string    `json:"uuid"`
	PostUUID    string    `json:"post_uuid"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	FileURL     string    `json:"file_url"`
	PreviewURL  string    `json:"preview_url"`
	CreatedAt   time.Time `json:"created_at"`
}
