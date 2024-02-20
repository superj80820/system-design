package domain

import "time"

type Account struct {
	ID       int64
	Email    string
	Password string

	AccessToken  string `gorm:"-"`
	RefreshToken string `gorm:"-"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type AccountRepo interface {
	Create(email, password string) (*Account, error)
	Get(userID int) (*Account, error)
	GetEmail(email string) (*Account, error)
}

type AccountService interface {
	Register(email, password string) (*Account, error)
	Get(userID int) (*Account, error)
}
