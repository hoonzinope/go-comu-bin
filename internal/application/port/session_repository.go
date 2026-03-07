package port

type SessionRepository interface {
	Save(userID int64, token string, ttlSeconds int) error
	Delete(userID int64, token string) error
	DeleteByUser(userID int64) error
	Exists(userID int64, token string) (bool, error)
}
