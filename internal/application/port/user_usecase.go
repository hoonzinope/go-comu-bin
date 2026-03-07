package port

type UserUseCase interface {
	SignUp(username, password string) (string, error)
	DeleteMe(userID int64, password string) error
}
