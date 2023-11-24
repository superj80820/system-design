package domain

import "time"

type Auth struct {
	Sub string // TODO: user id, maybe use another way for security
	EXP string
	IAT string
}

type AccountTokenEnum int

const (
	UNKNOWN_TOKEN AccountTokenEnum = iota
	ACCESS_TOKEN
	REFRESH_TOKEN
)

type AccountTokenStatusEnum int

const (
	NOT_ACTIVE AccountTokenStatusEnum = iota
	ACTIVE
	REVOKE
)

type AccountToken struct {
	ID        int64
	Token     string
	Status    AccountTokenStatusEnum // TODO: use bool, or int?
	Type      AccountTokenEnum       // TODO: use enum, or another type?
	UserID    int64
	ExpireAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AuthService interface {
	Login(email, password string) (*Account, error)
	Logout(accessToken string) error
	RefreshAccessToken(refreshToken string) (string, error)
	Verify(accessToken string) error
}
