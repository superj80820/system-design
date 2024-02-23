package util

import (
	"context"
	"time"
)

type tokenBucket struct {
	count     int
	maxCount  int
	duration  time.Duration
	updatedAt time.Time
}

func createTokenBucket(maxCount int, duration time.Duration) *tokenBucket {
	return &tokenBucket{
		maxCount:  maxCount,
		duration:  duration,
		updatedAt: time.Now(),
	}
}

func (t *tokenBucket) pass() (bool, int, time.Duration) {
	expiry := t.duration - (time.Now().Sub(t.updatedAt))
	if expiry < 0 {
		t.count = 1
		t.updatedAt = time.Now()
		expiry := t.duration - (time.Now().Sub(t.updatedAt))
		return true, t.count, expiry
	}
	if t.count < t.maxCount {
		t.count++
		return true, t.count, expiry
	}
	return false, t.count, expiry
}

type RateLimitTokenBucket interface {
	Get() chan struct{}
	Reset(tokenBucketMaxCount int, tokenBucketDuration time.Duration)
	Stop()
}

type updateTypeEnum int

const (
	updateTypeConfig updateTypeEnum = iota + 1
	updateTypeStop
)

type updateConfig struct {
	updateType          updateTypeEnum
	tokenBucketMaxCount int
	tokenBucketDuration time.Duration
}

type rateLimitTokenBucket struct {
	tokenBucketCh chan struct{}
	updateCh      chan *updateConfig
}

func CreateRateLimitTokenBucket(ctx context.Context, maxCount int, duration time.Duration) RateLimitTokenBucket {
	waitCh := make(chan struct{})
	updateCh := make(chan *updateConfig)
	go func() {
		fn := func(ctx context.Context, maxCount int, duration time.Duration) (chan struct{}, context.CancelFunc) {
			done := make(chan struct{})
			ctx, cancel := context.WithCancel(ctx)
			tokenBucket := createTokenBucket(maxCount, duration)
			go func() {
				defer close(done)

				timer := time.NewTimer(0)
				defer timer.Stop()

				for {
					if ctx.Err() != nil {
						return
					}
					pass, _, expiry := tokenBucket.pass()
					if pass {
						select {
						case waitCh <- struct{}{}:
						case <-ctx.Done():
							return
						}
					} else {
						timer.Reset(expiry)
						select {
						case <-timer.C:
						case <-ctx.Done():
							return
						}
					}
				}
			}()
			return done, cancel
		}

		done, cancel := fn(ctx, maxCount, duration)
		for {
			select {
			case <-ctx.Done():
				return
			case updateVal := <-updateCh:
				cancel()
				<-done
				done, cancel = fn(ctx, updateVal.tokenBucketMaxCount, updateVal.tokenBucketDuration)
			}
		}
	}()
	return &rateLimitTokenBucket{
		tokenBucketCh: waitCh,
		updateCh:      updateCh,
	}
}

func (t *rateLimitTokenBucket) Get() chan struct{} {
	return t.tokenBucketCh
}

func (t *rateLimitTokenBucket) Reset(tokenBucketCount int, tokenBucketDuration time.Duration) {
	t.updateCh <- &updateConfig{
		updateType:          updateTypeConfig,
		tokenBucketMaxCount: tokenBucketCount,
		tokenBucketDuration: tokenBucketDuration,
	}
}

func (t *rateLimitTokenBucket) Stop() {
	t.updateCh <- &updateConfig{
		updateType: updateTypeStop,
	}
}
