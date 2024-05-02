package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	if err := exec.Command("bash", "-c", "docker-compose up -d").Run(); err != nil {
		panic(errors.Wrap(err, "exec script failed"))
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	restartTicker := time.NewTicker(1 * time.Hour)
	defer restartTicker.Stop()
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for {
			select {
			case <-restartTicker.C:
				log.Println("---restart service start")
				if err := exec.Command("bash", "-c", "docker-compose down").Run(); err != nil {
					return errors.Wrap(err, "exec script failed")
				}
				log.Println("closed all service")
				if err := exec.Command("bash", "-c", "docker-compose up -d").Run(); err != nil {
					return errors.Wrap(err, "exec script failed")
				}
				log.Println("start all service")
				log.Println("---restart service done")
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

	if err := exec.Command("bash", "-c", "docker-compose down").Run(); err != nil {
		panic(errors.Wrap(err, "exec script failed"))
	}
}
