package infrastructure

type InMemory struct {
	// In-memory storage for users, boards, posts, comments, reactions
	User     map[int64]interface{}
	Board    map[int64]interface{}
	Post     map[int64]interface{}
	Comment  map[int64]interface{}
	Reaction map[int64]interface{}
}

func NewInMemory() *InMemory {
	return &InMemory{
		User:     make(map[int64]interface{}),
		Board:    make(map[int64]interface{}),
		Post:     make(map[int64]interface{}),
		Comment:  make(map[int64]interface{}),
		Reaction: make(map[int64]interface{}),
	}
}
