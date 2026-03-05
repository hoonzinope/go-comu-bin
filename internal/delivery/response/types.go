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
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	BoardID   int64     `json:"board_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostList struct {
	Posts      []Post `json:"posts"`
	Limit      int    `json:"limit"`
	LastID     int64  `json:"last_id"`
	HasMore    bool   `json:"has_more"`
	NextLastID *int64 `json:"next_last_id,omitempty"`
}

type PostDetail struct {
	Post      *Post           `json:"post"`
	Comments  []CommentDetail `json:"comments"`
	Reactions []Reaction      `json:"reactions"`
}

type Comment struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	PostID    int64     `json:"post_id"`
	ParentID  *int64    `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
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
	UserID     int64     `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
}
