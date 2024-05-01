package repository

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type s3Repo struct {
	bucket string
	region string
}

func CreateS3Repo(awsAccessKeyID, awsSecretAccessKey, bucket, region string) domain.S3Repo {
	os.Setenv("AWS_ACCESS_KEY_ID", awsAccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", awsSecretAccessKey)

	return s3Repo{
		bucket: bucket,
		region: region,
	}
}

func (s s3Repo) Upload(ctx context.Context, fileReader io.Reader, key string) error {
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(s.region))
	if err != nil {
		return errors.Wrap(err, "load default config failed")
	}

	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   fileReader,
	})
	if err != nil {
		return errors.Wrap(err, "put object failed")
	}

	return nil
}
