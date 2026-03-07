package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type BcryptPasswordHasher struct {
	cost int
}

func NewBcryptPasswordHasher(cost int) *BcryptPasswordHasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptPasswordHasher{cost: cost}
}

func (h *BcryptPasswordHasher) Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func (h *BcryptPasswordHasher) Matches(hashedPassword, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	return false, err
}
