package main

import (
	"errors"
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubUserRepository struct {
	save func(user *entity.User) (int64, error)
}

func (r *stubUserRepository) Save(user *entity.User) (int64, error) {
	if r.save != nil {
		return r.save(user)
	}
	return 1, nil
}

func (r *stubUserRepository) SelectUserByUsername(username string) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) SelectUserByID(id int64) (*entity.User, error) {
	return nil, nil
}

func (r *stubUserRepository) Delete(id int64) error {
	return nil
}

func TestSeedAdmin_ReturnsError_WhenSaveFails(t *testing.T) {
	expected := errors.New("save failed")

	err := seedAdmin(&stubUserRepository{
		save: func(user *entity.User) (int64, error) {
			require.Equal(t, "admin", user.Name)
			require.NotEmpty(t, user.Password)
			return 0, expected
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, expected)
}
