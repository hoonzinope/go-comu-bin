package delivery

type userCredentialRequest struct {
	Username string `json:"username" example:"alice"`
	Password string `json:"password" example:"pw"`
}

type passwordOnlyRequest struct {
	Password string `json:"password" example:"pw"`
}

type boardRequest struct {
	Name        string `json:"name" example:"free"`
	Description string `json:"description" example:"free board"`
}

type postRequest struct {
	Title   string `json:"title" example:"hello"`
	Content string `json:"content" example:"first post"`
}

type commentRequest struct {
	Content string `json:"content" example:"nice post"`
}

type reactionRequest struct {
	ReactionType string `json:"reaction_type" example:"like"`
}
