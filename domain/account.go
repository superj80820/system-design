package domain

import "time"

type Account struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`

	AccessToken  string `gorm:"-" json:"access_token"`
	RefreshToken string `gorm:"-" json:"refresh_token"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

type AccountRepo interface {
	Create(email, password string) (*Account, error)
	Get(userID int) (*Account, error)
	GetEmail(email string) (*Account, error)
}

type AccountUseCase interface {
	Register(email, password string) (*Account, error)
	Get(userID int) (*Account, error)
}
