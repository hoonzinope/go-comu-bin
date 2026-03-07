package port

type PasswordHasher interface {
	Hash(password string) (string, error)
	Matches(hashedPassword, password string) (bool, error)
}
