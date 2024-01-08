package repository

import "github.com/superj80820/system-design/domain"

type AccountEntity domain.Account

func (AccountEntity) TableName() string {
	return "account"
}