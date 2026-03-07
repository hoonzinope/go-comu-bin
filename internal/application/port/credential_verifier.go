package port

type CredentialVerifier interface {
	VerifyCredentials(username, password string) (int64, error)
}
