package port

type TokenProvider interface {
	IdToToken(userID int64) (string, error)
	TTLSeconds() int
	ValidateTokenToId(token string) (int64, error)
}
