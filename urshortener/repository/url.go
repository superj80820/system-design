package repository

import "github.com/superj80820/system-design/domain"

type URLEntity domain.URL

func (URLEntity) TableName() string {
	return "url"
}
