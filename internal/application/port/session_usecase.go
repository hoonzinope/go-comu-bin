package port

type SessionUseCase interface {
	Login(username, password string) (string, error)
	Logout(token string) error
	InvalidateUserSessions(userID int64) error
	ValidateTokenToId(token string) (int64, error)
}
