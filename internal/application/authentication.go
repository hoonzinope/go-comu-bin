package application

type AuthUseCase interface {
	IdToToken(userID int64) (string, error)
	ValidateTokenToId(token string) (int64, error)
}
