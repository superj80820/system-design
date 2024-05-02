package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	redisKit "github.com/superj80820/system-design/kit/cache/redis"
)

const EnableGroupRedisKeyTemplate = "ACTRESS:ENABLE_GROUP:%s"

type actressLineRepository struct {
	redisCache *redisKit.Cache
}

func CreateActressLineRepo(redisCache *redisKit.Cache) domain.ActressLineRepository {
	return &actressLineRepository{
		redisCache: redisCache,
	}
}

func (a *actressLineRepository) DisableGroupRecognition(ctx context.Context, groupID string) error {
	if err := a.redisCache.Del(ctx, fmt.Sprintf(EnableGroupRedisKeyTemplate, groupID)); err != nil {
		return errors.Wrap(err, "delete redis key failed")
	}
	return nil
}

func (a *actressLineRepository) EnableGroupRecognition(ctx context.Context, groupID string) error {
	if err := a.redisCache.Set(ctx, fmt.Sprintf(EnableGroupRedisKeyTemplate, groupID), true, time.Hour*24); err != nil {
		return errors.Wrap(err, "set redis key failed")
	}
	return nil
}

func (a *actressLineRepository) GetUseInformation() (string, error) {
	return `你好!請對我們傳送圖片~\n我們來幫你找你的天使w\n\n對了 你可以拍照截圖做到以下幾點 天使會更容易找到：\n１．正面臉\n２．清晰照\n３．不截到其他人頭\n\n另外 你可以\n輸入\"許願\"：來找尋天使\n點選\"我心愛的女孩\"：來觀看我的最愛\n\n有任何問題都可以回報粉專喔~\nhttps://www.facebook.com/377946752672874`, nil
}

func (a *actressLineRepository) IsEnableGroupRecognition(ctx context.Context, groupID string) (bool, error) {
	exists := a.redisCache.Exists(ctx, fmt.Sprintf(EnableGroupRedisKeyTemplate, groupID))
	if err := exists.Err(); err != nil {
		return false, errors.Wrap(err, "check redis key exists failed")
	}
	if exists.Val() == 0 {
		return false, nil
	}
	return true, nil
}
