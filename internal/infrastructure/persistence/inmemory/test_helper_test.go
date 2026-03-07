package inmemory

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

func testUser(name, password string, admin bool) *entity.User {
	if admin {
		return entity.NewAdmin(name, password)
	}
	return entity.NewUser(name, password)
}

func testBoard(name, description string) *entity.Board {
	return entity.NewBoard(name, description)
}

func testPost(title, content string, authorID, boardID int64) *entity.Post {
	return entity.NewPost(title, content, authorID, boardID)
}

func testComment(content string, authorID, postID int64) *entity.Comment {
	return entity.NewComment(content, authorID, postID, nil)
}

func testReaction(targetType entity.ReactionTargetType, targetID int64, reactionType entity.ReactionType, userID int64) *entity.Reaction {
	return entity.NewReaction(targetType, targetID, reactionType, userID)
}
