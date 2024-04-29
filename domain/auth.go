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

type AuthRepo interface {
	GenerateToken(sub string, iat, exp time.Time) (string, error)
	VerifyToken(token string, accountTokenEnum AccountTokenEnum) (userID int64, err error)
	CreateToken(userID int64, token string, expireAt time.Time, tokenType AccountTokenEnum) (*AccountToken, error)
	UpdateStatusToken(token string, status AccountTokenStatusEnum) error
	GetLastRefreshTokenByUserID(userID int64) (*AccountToken, error)
}

type AuthUseCase interface {
	Login(email, password string) (*Account, error)
	Logout(accessToken string) error
	RefreshAccessToken(refreshToken string) (string, error)
	Verify(accessToken string) (int64, error)
}

type AuthLineUseCase interface {
	VerifyCode(code, redirectURI string) (*Account, *LineUserProfile, error)
	VerifyLIFFToken(token string) (*Account, *LineUserProfile, error)
}

type AuthServiceRepo interface {
	Verify(accessToken string) (int64, error)
}
