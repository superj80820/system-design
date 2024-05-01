package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestRepository(t *testing.T) {
	ctx := context.Background()

	s3Repo := CreateS3Repo("", "", "", "")

	f, err := os.Open("./test_image.jpg")
	if err != nil {
		fmt.Fprintf(os.Stderr, "無法打開檔案: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	s3Repo.Upload(ctx, f, "test")
}
