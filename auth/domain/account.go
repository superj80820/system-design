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

type AccountService interface {
	Register(email, password string) error
}
