package delivery

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

func CreateAsyncDBBackup(ctx context.Context, s3UseCase domain.S3UseCase, backupDBScript, filePath, bucketKey string, backupDuration time.Duration) error {
	ticker := time.NewTicker(backupDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return errors.Wrap(err, "context done get error")
			}
			return nil
		case <-ticker.C:
			if err := s3UseCase.BackupThenUpload(ctx, backupDBScript, filePath, bucketKey); err != nil {
				return errors.Wrap(err, "backup db then upload failed")
			}
		}
	}
}
