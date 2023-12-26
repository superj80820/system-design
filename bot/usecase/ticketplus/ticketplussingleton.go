package ticketplus

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type singletonUserInformation struct {
	lock            *sync.RWMutex
	userInformation *domain.TicketPlusDBUserToken
	updateFn        func(*domain.TicketPlusDBUserToken) (*domain.TicketPlusDBUserToken, error)
	asyncErr        error
}

func createSingletonUserInformation(ctx context.Context, updateFn func(*domain.TicketPlusDBUserToken) (*domain.TicketPlusDBUserToken, error), userInformation *domain.TicketPlusDBUserToken, expireDuration time.Duration) (*singletonUserInformation, error) {
	s := &singletonUserInformation{
		lock:            new(sync.RWMutex),
		updateFn:        updateFn,
		userInformation: userInformation,
	}
	if s.userInformation.AccessTokenExpiresIn.Sub(time.Now()) < expireDuration/2 {
		userInformation, err := s.updateFn(s.userInformation)
		if err != nil {
			return nil, errors.Wrap(err, "update token function failed")
		}
		s.userInformation = userInformation
	}
	go func() {
		ticker := time.NewTicker(expireDuration / 4)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				userInformation, err := s.updateFn(s.userInformation)
				if err != nil {
					s.lock.Lock()
					s.asyncErr = errors.Wrap(err, "update token function failed")
					s.lock.Unlock()
					return
				}
				s.lock.Lock()
				s.userInformation = userInformation
				s.lock.Unlock()
			}
		}
	}()

	return s, nil
}

func (s *singletonUserInformation) Get() (*domain.TicketPlusDBUserToken, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.asyncErr != nil {
		return nil, s.asyncErr
	}

	return s.userInformation, nil
}

type singletonCaptcha struct {
	captchaLock    *sync.RWMutex
	captchaRawData string
	ocrCaptchaFn   func(sessionID string) (string, error)

	rateLimitLock *sync.Mutex
	setDuration   time.Duration
	setMaxCount   int
	setNextTime   time.Time
	setCount      int
}

func createSingletonCaptcha(
	getCaptchaFn func(sessionID string) (string, error),
	setDuration time.Duration,
	setMaxCount int,
) (*singletonCaptcha, error) {
	return &singletonCaptcha{
		captchaLock:  new(sync.RWMutex),
		ocrCaptchaFn: getCaptchaFn,

		rateLimitLock: new(sync.Mutex),
		setDuration:   setDuration,
		setMaxCount:   setMaxCount,
	}, nil
}

func (s *singletonCaptcha) IsEmpty() bool {
	s.captchaLock.RLock()
	defer s.captchaLock.RUnlock()

	return len(s.captchaRawData) == 0
}

func (s *singletonCaptcha) waitRateLimit() {
	s.rateLimitLock.Lock()
	defer s.rateLimitLock.Unlock()

	if s.setCount < s.setMaxCount {
		s.setNextTime = time.Now().Add(s.setDuration)
		s.setCount++
	} else {
		if time.Now().Before(s.setNextTime) {
			time.Sleep(s.setNextTime.Sub(time.Now()))
		}
		s.setNextTime = time.Now().Add(s.setDuration)
		s.setCount = 1
	}
}

func (s *singletonCaptcha) Set(sessionID string) error {
	s.captchaLock.Lock()
	defer s.captchaLock.Unlock()

	s.waitRateLimit()

	captchaRawData, err := s.ocrCaptchaFn(sessionID)
	if err != nil {
		return errors.Wrap(err, "get captcha failed")
	}
	s.captchaRawData = captchaRawData

	return nil
}

func (s *singletonCaptcha) Get() string {
	s.captchaLock.RLock()
	defer s.captchaLock.RUnlock()

	return s.captchaRawData
}
