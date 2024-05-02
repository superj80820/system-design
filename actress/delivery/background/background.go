package background

import (
	"context"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

func CreateAsyncActressCrawler(ctx context.Context, actressCrawlerUseCase domain.ActressCrawlerUseCase) error {
	for {
		select {
		case <-actressCrawlerUseCase.Done():
			if err := actressCrawlerUseCase.Err(); err != nil {
				return errors.Wrap(err, "crawler failed")
			}
			return nil
		default:
			actressCrawlerUseCase.Process(ctx)
		}
	}
}
