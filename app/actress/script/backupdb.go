package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	awsRepo "github.com/superj80820/system-design/aws/repository"
	awsUseCase "github.com/superj80820/system-design/aws/usecase"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
	"golang.org/x/sync/errgroup"
)

func main() {
	utilKit.LoadEnvFile(".env")
	awsAccessKeyID := utilKit.GetEnvString("AWS_ACCESS_KEY_ID", "")
	awsSecretAccessKey := utilKit.GetEnvString("AWS_SECRET_ACCESS_KEY", "")
	dbBackupBucket := utilKit.GetEnvString("DB_BACKUP_BUCKET", "")
	dbBackupRegion := utilKit.GetEnvString("DB_BACKUP_REGION", "")
	dbBackupScript := utilKit.GetEnvString("DB_BACKUP_SCRIPT", "")
	dbBackupFilePath := utilKit.GetEnvString("DB_BACKUP_FILE_PATH", "")
	dbBackupKey := utilKit.GetEnvString("DB_BACKUP_KEY", "")
	dbBackupDuration := utilKit.GetEnvInt("DB_BACKUP_DURATION", 21600)

	ctx, cancel := context.WithCancel(context.Background())

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel, loggerKit.WithRotateLog(1, 10, 10))
	if err != nil {
		panic(err)
	}

	s3Repo := awsRepo.CreateS3Repo(awsAccessKeyID, awsSecretAccessKey, dbBackupBucket, dbBackupRegion)
	s3UseCase := awsUseCase.CreateS3UseCase(s3Repo)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	backupDBTicker := time.NewTicker(time.Duration(dbBackupDuration) * time.Second)
	defer backupDBTicker.Stop()
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for {
			select {
			case <-backupDBTicker.C:
				logger.Info("backup db start")
				if err := s3UseCase.BackupThenUpload(ctx, dbBackupScript, dbBackupFilePath, dbBackupKey); err != nil {
					logger.Error("backup db error", loggerKit.Error(err))
				}
				logger.Info("backup db done")
			case <-ctx.Done():
				return nil
			}
		}
	})

	<-quit
	cancel()

	if err := eg.Wait(); err != nil {
		panic(errors.Wrap(err, "error group get error"))
	}
}
