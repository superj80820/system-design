package usecase

import (
	"context"
	"net/http"
	"time"

	"github.com/jxskiss/base62"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	"github.com/superj80820/system-design/kit/code"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
	"github.com/superj80820/system-design/urlshortener/repository"
)

type urlService struct {
	db     *ormKit.DB
	cache  *redisKit.Cache
	logger *loggerKit.Logger
}

func CreateURLService(db *ormKit.DB, cache *redisKit.Cache, logger *loggerKit.Logger) (*urlService, error) {
	if db == nil || cache == nil || logger == nil {
		return nil, errors.New("create service failed")
	}
	return &urlService{db: db, cache: cache, logger: logger}, nil
}

func checkPrefixURL(url, prefix string) bool {
	for i := 0; i < len(prefix); i++ {
		if url[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (u urlService) Save(ctx context.Context, url string) (string, error) {
	if !checkPrefixURL(url, "https://") && !checkPrefixURL(url, "http://") {
		return "", errors.New("error url format")
	}

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return "", errors.Wrap(err, "generate unique id failed")
	}

	uniqueID := uniqueIDGenerate.Generate()
	shortURLID := uniqueID.GetInt64()
	shortURL := uniqueID.GetBase62()

	if err := u.cache.SetBF(ctx, domain.SHORT_URL_BF_CACHE, shortURL); err != nil {
		return "", errors.Wrap(err, "set bloom filter failed")
	}

	err = u.db.Create(&repository.URLEntity{
		ID:       shortURLID,
		LongURL:  url,
		ShortURL: shortURL,
	}).Error
	if err != nil {
		return "", errors.Wrap(err, "save url data failed")
	}

	return shortURL, nil
}

func (u urlService) Get(ctx context.Context, shortURL string) (string, error) {
	shortURLID, err := base62.ParseInt([]byte(shortURL))
	if err != nil {
		return "", errors.Wrap(err, "parse short url to number failed")
	}

	maybeExists, err := u.cache.MaybeExistsBF(ctx, domain.SHORT_URL_BF_CACHE, shortURL)
	if err != nil {
		return "", errors.Wrap(err, "check exists failed")
	}
	if !maybeExists {
		return "", code.CreateErrorCode(http.StatusNotFound)
	}

	val, exist, err := u.cache.Get(ctx, shortURL)
	if err != nil {
		return "", errors.Wrap(err, "get cache failed")
	}
	if exist {
		return val, nil
	}

	var url repository.URLEntity
	err = u.db.First(&url, shortURLID)
	if err != nil {
		return "", errors.New("get from db failed")
	}

	if err := u.cache.Set(ctx, shortURL, url.LongURL, time.Second*30); err != nil {
		return "", errors.Wrap(err, "set cache failed")
	}

	return url.LongURL, nil
}
