package domain

import (
	"context"
	"io"
)

type S3Repo interface {
	Upload(ctx context.Context, fileReader io.Reader, key string) error
}

type S3UseCase interface {
	BackupThenUpload(ctx context.Context, backupDBScript, filePath, bucketKey string) error
}
