package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	httpKit "github.com/superj80820/system-design/kit/http"

	"github.com/superj80820/system-design/url-shorter/domain"
)

type RateLimitMiddleware struct {
	passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)
	next     domain.URLService
}

func CreateRateLimitMiddleware(next domain.URLService, passFunc func(ctx context.Context, key string) (pass bool, lastRequests, curExpiry int, err error)) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		passFunc: passFunc,
		next:     next,
	}
}

func (mw RateLimitMiddleware) Get(ctx context.Context, shortURL string) (string, error) {
	pass, lastRequests, expiry, err := mw.passFunc(ctx, httpKit.GetIP(ctx))
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprint("get rate limit failed"))
	}
	if !pass {
		return "", httpKit.CreateErrorHTTPCodeWithCode(http.StatusTooManyRequests, 1, lastRequests, expiry)
	}
	longURL, err := mw.next.Get(ctx, shortURL)
	return longURL, err
}

func (mw RateLimitMiddleware) Save(ctx context.Context, url string) (string, error) {
	pass, lastRequests, expiry, err := mw.passFunc(ctx, httpKit.GetIP(ctx))
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprint("get rate limit failed"))
	}
	if !pass {
		return "", httpKit.CreateErrorHTTPCodeWithCode(http.StatusTooManyRequests, 1, lastRequests, expiry)
	}
	shortURL, err := mw.next.Save(ctx, url)
	return shortURL, err
}
