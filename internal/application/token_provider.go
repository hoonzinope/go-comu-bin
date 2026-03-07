package application

type TokenProvider interface {
	IdToToken(userID int64) (string, error)
	ValidateTokenToId(token string) (int64, error)
}
