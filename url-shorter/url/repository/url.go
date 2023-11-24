package repository

import "github.com/superj80820/system-design/url-shorter/domain"

type URLEntity domain.URL

func (URLEntity) TableName() string {
	return "url"
}
