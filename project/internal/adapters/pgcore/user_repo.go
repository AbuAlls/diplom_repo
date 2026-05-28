package pgcore

import (
	"context"

	"diplom.com/m/internal/ports"
)

type UserRepo struct{ Store *Store }

func NewUserRepo(store *Store) *UserRepo { return &UserRepo{Store: store} }

func (r *UserRepo) Create(ctx context.Context, email, fullName, passwordHash string) (int64, error) {
	const q = `insert into users (email, full_name, password_hash) values ($1, $2, $3) returning id`
	var id int64
	if err := r.Store.Pool.QueryRow(ctx, q, email, fullName, passwordHash).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (ports.UserDTO, error) {
	const q = `select id, email, full_name, password_hash from users where email = $1`
	var u ports.UserDTO
	err := r.Store.Pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.FullName, &u.PasswordHash)
	return u, err
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (ports.UserDTO, error) {
	const q = `select id, email, full_name, password_hash from users where id = $1`
	var u ports.UserDTO
	err := r.Store.Pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.FullName, &u.PasswordHash)
	return u, err
}

var _ ports.UserRepo = (*UserRepo)(nil)
