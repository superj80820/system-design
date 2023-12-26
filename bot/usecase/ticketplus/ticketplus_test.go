package ticketplus

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSingletonCaptchaRateLimit(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test timer",
			fn: func(t *testing.T) {
				singletonCaptcha, err := createSingletonCaptcha(
					func(sessionID string) (string, error) {
						return "", nil
					},
					2*time.Second,
					5,
				)
				assert.Nil(t, err)

				{
					done := make(chan struct{})
					go func() {
						for i := 0; i < 5; i++ {
							singletonCaptcha.waitRateLimit()
						}
						close(done)
					}()
					select {
					case <-done:
					case <-time.NewTimer(1 * time.Second).C:
						assert.Fail(t, "rate limit timeout")
					}
				}

				{
					done := make(chan struct{})
					go func() {
						singletonCaptcha.waitRateLimit()
						close(done)
					}()
					select {
					case <-done:
					case <-time.NewTimer(3 * time.Second).C:
						assert.Fail(t, "rate limit timeout")
					}
				}
			},
		},
		{
			scenario: "test race condition",
			fn: func(t *testing.T) {
				singletonCaptcha, err := createSingletonCaptcha(
					func(sessionID string) (string, error) {
						return "", nil
					},
					1*time.Millisecond,
					5,
				)
				assert.Nil(t, err)

				wg := new(sync.WaitGroup)
				wg.Add(10000)
				for i := 0; i < 10000; i++ {
					go func() {
						defer wg.Done()

						singletonCaptcha.waitRateLimit()
					}()
				}

				wg.Add(10000)
				for i := 0; i < 10000; i++ {
					go func() {
						defer wg.Done()
						singletonCaptcha.rateLimitLock.Lock()
						defer singletonCaptcha.rateLimitLock.Unlock()

						assert.LessOrEqual(t, singletonCaptcha.setCount, singletonCaptcha.setMaxCount)
					}()
				}

				wg.Wait()
			},
		},
		{
			scenario: "test race condition duration",
			fn: func(t *testing.T) {
				singletonCaptcha, err := createSingletonCaptcha(
					func(sessionID string) (string, error) {
						return "", nil
					},
					1*time.Second,
					1,
				)
				assert.Nil(t, err)

				timeNow := time.Now()

				wg := new(sync.WaitGroup)
				wg.Add(3)
				for i := 0; i < 3; i++ {
					go func() {
						defer wg.Done()

						singletonCaptcha.Set("")
					}()
				}
				wg.Wait()

				assert.Less(t, 2.0, time.Since(timeNow).Seconds())
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}

func TestMaskMobile(t *testing.T) {
	assert.Equal(t, "98****733", maskMobile("985698733"))
}
