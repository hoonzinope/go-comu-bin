package inmemory

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

func testUser(name, password string, admin bool) *entity.User {
	u := &entity.User{}
	if admin {
		u.NewAdmin(name, password)
		return u
	}
	u.NewUser(name, password)
	return u
}

func testBoard(name, description string) *entity.Board {
	b := &entity.Board{}
	b.NewBoard(name, description)
	return b
}

func testPost(title, content string, authorID, boardID int64) *entity.Post {
	p := &entity.Post{}
	p.NewPost(title, content, authorID, boardID)
	return p
}

func testComment(content string, authorID, postID int64) *entity.Comment {
	c := &entity.Comment{}
	c.NewComment(content, authorID, postID, nil)
	return c
}

func testReaction(targetType string, targetID int64, reactionType string, userID int64) *entity.Reaction {
	r := &entity.Reaction{}
	r.NewReaction(targetType, targetID, reactionType, userID)
	return r
}
