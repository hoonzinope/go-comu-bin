package port

type AccountUseCase interface {
	DeleteMyAccount(userID int64, password string) error
}
