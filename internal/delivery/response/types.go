package response

import "time"

type User struct {
	ID              int64      `json:"id"`
	UUID            string     `json:"uuid"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Guest           bool       `json:"guest"`
	GuestStatus     string     `json:"guest_status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

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
	Posts      []Post  `json:"posts"`
	Limit      int     `json:"limit"`
	Cursor     string  `json:"cursor"`
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
}

type PostDetail struct {
	Post            *Post           `json:"post"`
	Tags            []Tag           `json:"tags"`
	Attachments     []Attachment    `json:"attachments"`
	Comments        []CommentDetail `json:"comments"`
	CommentsHasMore bool            `json:"comments_has_more"`
	Reactions       []Reaction      `json:"reactions"`
	MyReactionType  *string         `json:"my_reaction_type"`
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
	Comment        *Comment   `json:"comment"`
	Reactions      []Reaction `json:"reactions"`
	MyReactionType *string    `json:"my_reaction_type"`
}

type Notification struct {
	UUID           string      `json:"uuid"`
	Type           string      `json:"type"`
	ActorUUID      string      `json:"actor_uuid"`
	PostUUID       string      `json:"post_uuid"`
	CommentUUID    *string     `json:"comment_uuid,omitempty"`
	ActorName      string      `json:"actor_name"`
	PostTitle      string      `json:"post_title"`
	CommentPreview string      `json:"comment_preview"`
	IsRead         bool        `json:"is_read"`
	TargetKind     string      `json:"target_kind"`
	MessageKey     string      `json:"message_key"`
	MessageArgs    MessageArgs `json:"message_args"`
	ReadAt         *time.Time  `json:"read_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

type MessageArgs struct {
	ActorName      string `json:"actor_name"`
	PostTitle      string `json:"post_title"`
	CommentPreview string `json:"comment_preview"`
}

type NotificationList struct {
	Notifications []Notification `json:"notifications"`
	Limit         int            `json:"limit"`
	Cursor        string         `json:"cursor"`
	HasMore       bool           `json:"has_more"`
	NextCursor    *string        `json:"next_cursor,omitempty"`
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
