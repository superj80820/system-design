package domain

import (
	"context"
	"time"
)

type URL struct {
	ID        int64
	ShortURL  string
	LongURL   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type URLService interface {
	Save(ctx context.Context, url string) (string, error)
	Get(ctx context.Context, shortURL string) (string, error)
}

const SHORT_URL_BF_CACHE = "SHORT_URL_BF_CACHE"
