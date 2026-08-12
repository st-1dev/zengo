package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zengo/user-service/gen/db"
)

// Record is the example service user model returned by Repository.
type Record struct {
	// ID is the stable user identifier.
	ID string
	// Email is the persisted user email.
	Email string
	// Name is the persisted user display name.
	Name string
}

// Repository wraps generated SQL queries for the example user service.
type Repository struct {
	queries *db.Queries
}

// NewRepository builds a Repository from a pgxpool-backed query set.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queries: db.New(pool)}
}

// GetByID loads one user by identifier.
func (r *Repository) GetByID(ctx context.Context, id string) (*Record, error) {
	row, err := r.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &Record{ID: row.ID, Email: row.Email, Name: row.Name}, nil
}

// List loads all users known to the example service.
func (r *Repository) List(ctx context.Context) ([]*Record, error) {
	rows, err := r.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, &Record{ID: row.ID, Email: row.Email, Name: row.Name})
	}
	return out, nil
}

// Create inserts a new user with a generated identifier.
func (r *Repository) Create(ctx context.Context, email, name string) (*Record, error) {
	row, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		ID:    uuid.NewString(),
		Email: email,
		Name:  name,
	})
	if err != nil {
		return nil, err
	}
	return &Record{ID: row.ID, Email: row.Email, Name: row.Name}, nil
}
