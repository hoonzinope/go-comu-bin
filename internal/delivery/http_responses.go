package delivery

type signUpResponse struct {
	Result string `json:"result" example:"ok"`
}

type loginResponse struct {
	Login string `json:"login" example:"ok"`
}

type logoutResponse struct {
	Logout string `json:"logout" example:"ok"`
}

type errorResponse struct {
	Error string `json:"error" example:"invalid credential"`
}

type idResponse struct {
	ID int64 `json:"id" example:"1"`
}
