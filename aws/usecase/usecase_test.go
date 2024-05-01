package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	awsRepo "github.com/superj80820/system-design/aws/repository"
)

func TestUseCase(t *testing.T) {
	ctx := context.Background()

	s3RepoHandler := awsRepo.CreateS3Repo("", "", "", "")
	s3UseCase := CreateS3UseCase(s3RepoHandler)

	err := s3UseCase.BackupThenUpload(ctx, "", "", "")
	assert.Nil(t, err)
}
