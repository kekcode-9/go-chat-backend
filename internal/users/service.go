package users

import "github.com/jackc/pgx/v5/pgxpool"

type UserService struct {
	Repo *Repository
}

func NewUserService(db *pgxpool.Pool) *UserService {
	repo := NewRepository(db)

	return &UserService{
		Repo: repo,
	}
}
