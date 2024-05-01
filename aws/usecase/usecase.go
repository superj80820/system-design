package usecase

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type s3UseCase struct {
	s3Repo domain.S3Repo
}

func CreateS3UseCase(s3Repo domain.S3Repo) domain.S3UseCase {
	return s3UseCase{
		s3Repo: s3Repo,
	}
}

func (s s3UseCase) BackupThenUpload(ctx context.Context, backupScript, filePath, bucketKey string) error {
	file, err := utilKit.ExecScriptAndGetFile(backupScript, filePath)
	if err != nil {
		return errors.Wrap(err, "exec script and get file failed")
	}

	if err := s.s3Repo.Upload(ctx, file, bucketKey); err != nil {
		return errors.Wrap(err, "update to s3 failed")
	}

	file.Close()

	if err := os.Remove(filePath); err != nil {
		return errors.Wrap(err, "remove file failed")
	}

	return nil
}
