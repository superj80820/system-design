package repository

import "github.com/superj80820/system-design/auth/domain"

type AccountTokenEntity domain.AccountToken

func (AccountTokenEntity) TableName() string {
	return "account_token"
}
