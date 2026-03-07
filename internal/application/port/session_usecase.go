package port

type SessionUseCase interface {
	Login(username, password string) (string, error)
	Logout(token string) error
	ValidateTokenToId(token string) (int64, error)
}
