package users

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	Repo *Repository
}

func NewUserService(db *pgxpool.Pool) *UserService {
	repo := NewRepository(db)

	return &UserService{
		Repo: repo,
	}
}

func (u *UserService) LookupUser(
	email string,
	userName string,
) (*UserLookupResponse, error) {

	ctx := context.Background()

	tx, err := u.Repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	// For now only email lookup exists.
	// Username lookup can be added later with another repo method.
	if email == "" {
		return nil, ErrMissingQuery
	}

	user, err := u.Repo.GetUserByEmail(
		ctx,
		tx,
		email,
	)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	resp := &UserLookupResponse{
		ID:       user.ID,
		UserName: user.UserName,
		Email:    user.Email,
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return resp, nil
}
