package ticketplus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	utilKit "github.com/superj80820/system-design/kit/util"
	"golang.org/x/sync/errgroup"
)

const eventKeyFormat = "%s-%s:%s" // CountryCode-Mobile:EventID

var (
	ctxDoneErr = errors.New("done by context")

	cancelTimeoutDuration = time.Second * 20
)

type ticketPlus struct {
	ticketPlusOrderListURL string

	repoTicketPlus domain.TicketPlusRepo
	repoCaptcha    domain.OCRService
	repoEvenSource domain.EventSourceRepo[*domain.TicketPlusEvent]
	lineRepo       domain.LineRepo

	logger loggerKit.Logger

	tokenExpireDuration time.Duration
	reservesScheduleMap utilKit.GenericSyncMap[string, *reservesSchedule]

	tokenBucketCh chan struct{}
}

type reservesSchedule struct {
	ticketPlusReserveSchedule *domain.TicketPlusReserveSchedule
	cancelFunc                context.CancelFunc
	done                      chan struct{}
}

func CreateTicketPlus(
	ctx context.Context,
	ticketPlusOrderListURL string,
	ticketPlusRepo domain.TicketPlusRepo,
	captchaRepo domain.OCRService,
	evenSourceRepo domain.EventSourceRepo[*domain.TicketPlusEvent],
	lineRepo domain.LineRepo,
	logger loggerKit.Logger,
	tokenExpireDuration time.Duration,
	ticketPlusReserveRequests []*domain.TicketPlusReserveSchedule,
	tokenBucketDuration time.Duration,
	tokenBucketCount int,
) (domain.TicketPlusService, error) {
	tokenBucketCh := make(chan struct{}, tokenBucketCount)

	go func() {
		ticker := time.NewTicker(tokenBucketDuration)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C: // TODO: check
				for i := 0; i < tokenBucketCount; i++ {
					select {
					case tokenBucketCh <- struct{}{}:
					default:
					}
				}
			case <-ctx.Done():
			}
		}
	}()

	t := &ticketPlus{
		ticketPlusOrderListURL: ticketPlusOrderListURL,

		repoTicketPlus: ticketPlusRepo,
		repoCaptcha:    captchaRepo,
		repoEvenSource: evenSourceRepo,
		lineRepo:       lineRepo,

		logger: logger,

		tokenExpireDuration: tokenExpireDuration,

		tokenBucketCh: tokenBucketCh,
	}

	t.formatReserveRequests(ticketPlusReserveRequests)

	_, err := t.Reserves(ctx, ticketPlusReserveRequests)
	if err != nil {
		return nil, errors.Wrap(err, "default reserves failed")
	}

	return t, nil
}

func (t *ticketPlus) formatReserveRequests(ticketPlusReserveRequests []*domain.TicketPlusReserveSchedule) error {
	for _, ticketPlusReserveRequest := range ticketPlusReserveRequests {
		if ticketPlusReserveRequest.Mobile == "" {
			return errors.New("mobile not set")
		}
		if ticketPlusReserveRequest.Password == "" {
			return errors.New("password not set")
		}
		if ticketPlusReserveRequest.CountryCode == "" {
			return errors.New("country code not set")
		}
		if ticketPlusReserveRequest.EventID == "" {
			return errors.New("event id not set")
		}
		if ticketPlusReserveRequest.CaptchaDuration == 0 {
			ticketPlusReserveRequest.CaptchaDuration = 1000
		}
		if ticketPlusReserveRequest.CaptchaCount == 0 {
			ticketPlusReserveRequest.CaptchaCount = 1
		}
		if ticketPlusReserveRequest.ReserveExecTime == 0 {
			ticketPlusReserveRequest.ReserveExecTime = time.Now().Add(5 * time.Second).Unix()
		}
		if ticketPlusReserveRequest.ReserveCount == 0 {
			ticketPlusReserveRequest.ReserveCount = 1
		}
	}
	return nil
}

func (t *ticketPlus) getSingleton(ctx context.Context, countryCode, mobile, password string, expireDuration, captchaDuration time.Duration, captchaCount int) (*singletonUserInformation, *singletonCaptcha, error) {
	userTokenInformation, err := t.repoTicketPlus.GetToken(countryCode, mobile)
	if errors.Is(err, ormKit.ErrRecordNotFound) {
		t.logger.Info("step: login, mobile: " + mobile)
		loginRes, err := t.repoTicketPlus.Login(countryCode, mobile, password)
		if errors.Is(err, domain.ErrCodeLocked) {
			return nil, nil, errors.New("login user locked")
		} else if err != nil {
			return nil, nil, errors.Wrap(err, "login failed")
		}
		accessTokenExpiresIn := time.Now().Add(time.Duration(loginRes.UserInfo.AccessTokenExpiresIn) * time.Second)
		_, err = t.repoTicketPlus.SaveOrUpdateToken(
			countryCode,
			mobile,
			loginRes.UserInfo.ID,
			loginRes.UserInfo.AccessToken,
			loginRes.UserInfo.RefreshToken,
			accessTokenExpiresIn,
		)
		if err != nil {
			return nil, nil, errors.Wrap(err, "save or update token faileed")
		}
		userTokenInformation, err = t.repoTicketPlus.GetToken(countryCode, mobile)
		if err != nil {
			return nil, nil, errors.Wrap(err, "get user token information failed")
		}
	} else if err != nil {
		return nil, nil, errors.Wrap(err, "get user token information failed")
	}

	singletonUserInformation, err := createSingletonUserInformation(ctx, func(userInformation *domain.TicketPlusDBUserToken) (*domain.TicketPlusDBUserToken, error) {
		t.logger.Info("step: login, mobile: " + mobile)
		loginRes, err := t.repoTicketPlus.Login(countryCode, mobile, password)
		if errors.Is(err, domain.ErrCodeLocked) {
			return nil, errors.New("login user locked")
		} else if err != nil {
			return nil, errors.Wrap(err, "login failed")
		}
		accessTokenExpiresIn := time.Now().Add(time.Duration(loginRes.UserInfo.AccessTokenExpiresIn) * time.Second)
		_, err = t.repoTicketPlus.SaveOrUpdateToken(
			countryCode,
			mobile,
			loginRes.UserInfo.ID,
			loginRes.UserInfo.AccessToken,
			loginRes.UserInfo.RefreshToken,
			accessTokenExpiresIn,
		)
		if err != nil {
			return nil, errors.Wrap(err, "save or update token faileed")
		}

		userInformation.AccessToken = loginRes.UserInfo.AccessToken
		userInformation.RefreshToken = loginRes.UserInfo.RefreshToken
		userInformation.AccessTokenExpiresIn = accessTokenExpiresIn

		return userInformation, nil
	}, userTokenInformation, expireDuration)

	singletonCaptcha, err := createSingletonCaptcha(func(sessionID string) (string, error) {
		t.logger.Info("step: set captcha, mobile: " + mobile)
		userInformation, err := singletonUserInformation.Get()
		if err != nil {
			return "", errors.Wrap(err, "get token failed")
		}
		captchaInformation, err := t.repoTicketPlus.GetCaptcha(userInformation.AccessToken, sessionID, true)
		if err != nil {
			t.logger.Info(fmt.Sprintf("get captcha information failed, error: %+v", err))
			return "", errors.Wrap(err, "get captcha information failed")
		}

		ocrInformation, err := t.repoCaptcha.OCR(captchaInformation.Data)
		if err != nil {
			t.logger.Info(fmt.Sprintf("get captcha answer failed, error: %+v", err))
			return "", errors.Wrap(err, "get captcha answer failed")
		}

		return ocrInformation.OCR, nil
	}, captchaDuration, captchaCount)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create singleton captcha failed")
	}

	return singletonUserInformation, singletonCaptcha, nil
}

func (t *ticketPlus) toReservesScheduleKey(countryCode, mobile, eventID string) string {
	return fmt.Sprintf(eventKeyFormat, countryCode, mobile, eventID)
}

func (t *ticketPlus) createEventReporter(countryCode, mobile, eventID string) func(ticketAreaName string, sortedIndex int, ticketPlusResponseStatus domain.TicketPlusResponseStatus, reportTime time.Time) {
	return func(ticketAreaName string, sortedIndex int, ticketPlusResponseStatus domain.TicketPlusResponseStatus, reportTime time.Time) {
		t.repoEvenSource.AddEvent(&domain.TicketPlusEvent{
			Key:         t.toReservesScheduleKey(countryCode, mobile, eventID),
			Name:        ticketAreaName,
			SortedIndex: sortedIndex,
			Status:      ticketPlusResponseStatus,
			Time:        reportTime,
		})
	}
}

func (t *ticketPlus) Reserve(
	ctx context.Context,
	ticketPlusReserveRequest *domain.TicketPlusReserveSchedule,
) (*domain.TicketReserve, error) {
	singletonUserInformation, singletonCaptcha, err := t.getSingleton(ctx, ticketPlusReserveRequest.CountryCode, ticketPlusReserveRequest.Mobile, ticketPlusReserveRequest.Password, t.tokenExpireDuration, ticketPlusReserveRequest.CaptchaDuration, ticketPlusReserveRequest.CaptchaCount)
	if err != nil {
		return nil, errors.Wrap(err, "get singleton failed")
	}
	eventReporter := t.createEventReporter(ticketPlusReserveRequest.CountryCode, ticketPlusReserveRequest.Mobile, ticketPlusReserveRequest.EventID)

	t.logger.Info("step: get ticket information, mobile: " + ticketPlusReserveRequest.Mobile + " event id: " + ticketPlusReserveRequest.EventID)
	ticketInformation, err := t.repoTicketPlus.GetTicketInformation(ticketPlusReserveRequest.EventID)
	ticketAreaIDs := make([]string, len(ticketInformation.Products))
	productIDs := make([]string, len(ticketInformation.Products))
	for idx, product := range ticketInformation.Products {
		ticketAreaIDs[idx] = product.TicketAreaID
		productIDs[idx] = product.ProductID
	}

	t.logger.Info("step: get ticket status, mobile: " + ticketPlusReserveRequest.Mobile + " event id: " + ticketPlusReserveRequest.EventID)
	ticketStatus, err := t.repoTicketPlus.GetTicketStatusOrderByCount(ticketAreaIDs, productIDs)
	if err != nil {
		return nil, errors.Wrap(err, "get ticket status failed")
	}
	ticketStatusForLog := make([]string, len(ticketStatus.Result.TicketArea))
	for idx, val := range ticketStatus.Result.TicketArea {
		ticketStatusForLog[idx] = val.TicketAreaName + " count: " + strconv.Itoa(val.Count)
	}
	t.logger.Info("ticket status: " + strings.Join(ticketStatusForLog, ", "))

	sessionIDMap := make(map[string]string)
	productIDsMap := make(map[string]string)
	for _, product := range ticketStatus.Result.Product {
		sessionIDMap[product.TicketAreaID] = product.SessionID
		productIDsMap[product.TicketAreaID] = product.ID
	}

	reserveCh := make(chan *domain.TicketReserve, len(ticketStatus.Result.TicketArea))
	errorCh := make(chan error)
	errGroupCtx, errGroupCancel := context.WithCancel(ctx)
	defer errGroupCancel()
	e, errGroupCtx := errgroup.WithContext(errGroupCtx)
	continueErr := errors.New("get error but continue")
	for _, val := range ticketStatus.Result.TicketArea {
		status := val

		fn := func() error {
			for {
				err = func() error {
					defer func() {
						timer := time.NewTimer(time.Duration(ticketPlusReserveRequest.ReserveDuration) * time.Millisecond)
						defer timer.Stop()

						select {
						case <-errGroupCtx.Done():
						case <-timer.C:
						}
					}()

					select {
					case <-errGroupCtx.Done():
						return ctxDoneErr
					default:
						<-t.tokenBucketCh
						timeNow := time.Now()
						if singletonCaptcha.IsEmpty() {
							if err := singletonCaptcha.Set(sessionIDMap[status.ID]); err != nil {
								return errors.Wrap(err, "set captcha failed")
							}
						}

						t.logger.Info("step: reserve, mobile: " + ticketPlusReserveRequest.Mobile + " event id: " + ticketPlusReserveRequest.EventID)
						userInformation, err := singletonUserInformation.Get()
						if err != nil {
							return errors.Wrap(err, "get user token failed")
						}
						reserveInformation, err := t.repoTicketPlus.Reserve(errGroupCtx, userInformation.AccessToken, []*domain.TicketReserveRequestProduct{
							{
								ProductID: productIDsMap[status.ID],
								Count:     ticketPlusReserveRequest.ReserveCount,
							},
						},
							userInformation.TicketPlusUserID,
							singletonCaptcha.Get(),
						)
						if errors.Is(err, domain.ReserveErrCaptchaNotFound) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusCaptchaNotFound,
								timeNow,
							)
							return errors.Wrap(continueErr, domain.TicketPlusResponseStatusCaptchaNotFound.String()+" "+status.TicketAreaName)
						} else if errors.Is(err, domain.ErrCodePending) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusPending,
								timeNow,
							)
							return errors.Wrap(continueErr, domain.TicketPlusResponseStatusPending.String()+" "+status.TicketAreaName)
						} else if errors.Is(err, domain.ReserveErrCaptchaFailed) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusCaptchaFailed,
								timeNow,
							)
							if err := singletonCaptcha.Set(sessionIDMap[status.ID]); err != nil {
								return errors.Wrap(err, "set ocr failed")
							}
							return errors.Wrap(continueErr, domain.TicketPlusResponseStatusCaptchaFailed.String()+" "+status.TicketAreaName)
						} else if errors.Is(err, domain.ReserveErrCodeReserved) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusReserved,
								timeNow,
							)
							return errors.Wrap(err, "already reserved")
						} else if errors.Is(err, domain.ReserveErrSoldOut) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusSoldOut,
								timeNow,
							)
							return errors.Wrap(continueErr, domain.TicketPlusResponseStatusSoldOut.String()+" "+status.TicketAreaName)
						} else if errors.Is(err, domain.ErrCodeUnknown) {
							eventReporter(
								status.TicketAreaName,
								status.SortedIndex,
								domain.TicketPlusResponseStatusUnknown,
								timeNow,
							)
							return errors.Wrap(continueErr, fmt.Sprintf("%s %s, other case, error: %+v", domain.TicketPlusResponseStatusUnknown.String(), status.TicketAreaName, err))
						} else if err != nil {
							return errors.Wrap(err, userInformation.TicketPlusUserID+" reserve failed, error: "+err.Error())
						}

						eventReporter(
							status.TicketAreaName,
							status.SortedIndex,
							domain.TicketPlusResponseStatusOK,
							timeNow,
						)
						reserveCh <- reserveInformation
						notifyMessage := make([]string, len(reserveInformation.Products))
						for idx, val := range reserveInformation.Products {
							notifyMessage[idx] = val.TicketAreaName + " count: " + strconv.Itoa(val.Count)
						}
						if err := t.lineRepo.Notify("get ticket, user: " + maskMobile(ticketPlusReserveRequest.Mobile) + ", information: " + strings.Join(notifyMessage, ", ") + "\n url: " + t.ticketPlusOrderListURL); err != nil {
							return errors.Wrap(err, "send line notify failed")
						}
						return nil
					}
				}()
				if errors.Is(err, continueErr) {
					t.logger.Info(err.Error())
					continue
				} else if err != nil {
					return err
				}
				return nil
			}
		}

		if priorityCount, ok := ticketPlusReserveRequest.Priority[status.TicketAreaName]; ok {
			for i := 0; i < priorityCount; i++ {
				e.Go(fn)
			}
		} else {
			for i := 0; i < 1; i++ {
				e.Go(fn)
			}
		}
	}
	go func() {
		errorCh <- e.Wait()
		close(errorCh)
	}()

	select {
	case reserveInformation := <-reserveCh:
		return reserveInformation, nil
	case err := <-errorCh:
		if err != nil {
			return nil, errors.Wrap(err, "reserve failed")
		}
	}

	return nil, errors.New("reserve failed, no get reserve information or error")
}

func (t *ticketPlus) Reserves(ctx context.Context, ticketPlusReserveRequests []*domain.TicketPlusReserveSchedule) ([]string, error) {
	t.formatReserveRequests(ticketPlusReserveRequests)
	processReservesStatusResult := make([]string, len(ticketPlusReserveRequests))
	timeoutDuration := cancelTimeoutDuration
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()

	for idx, val := range ticketPlusReserveRequests {
		ticketPlusReserveRequest := val

		reservesScheduleKey := t.toReservesScheduleKey(ticketPlusReserveRequest.CountryCode, ticketPlusReserveRequest.Mobile, ticketPlusReserveRequest.EventID)

		if reservesScheduleInstance, ok := t.reservesScheduleMap.Load(reservesScheduleKey); ok {
			reservesScheduleInstance.cancelFunc()
			timer.Reset(timeoutDuration)
			select {
			case <-timer.C:
				processReservesStatusResult[idx] = domain.ProcessReserveStatusStopTimeout.String()
			case <-reservesScheduleInstance.done:
				processReservesStatusResult[idx] = domain.ProcessReserveStatusUpdate.String()
			}
		} else {
			processReservesStatusResult[idx] = domain.ProcessReserveStatusAdd.String()
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func(ctx context.Context, ticketPlusReserveRequest *domain.TicketPlusReserveSchedule) {
			defer close(done)

			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				<-ticker.C

				if time.Now().Before(time.Unix(ticketPlusReserveRequest.ReserveExecTime, 0)) {
					continue
				}
				ticketReserveInformation, err := t.Reserve(
					ctx,
					ticketPlusReserveRequest,
				)
				if errors.Is(err, ctxDoneErr) {
					t.logger.Info(err.Error())
					return
				} else if err != nil {
					t.logger.Info(fmt.Sprintf("reserve error, mobile: %s, event id: %s, reserve result: %+v", ticketPlusReserveRequest.Mobile, ticketPlusReserveRequest.EventID, err))
					if ticketPlusReserveRequest.ReserveGetErrorThenContinue {
						continue
					}
					return
				}

				if reservesSchedule, ok := t.reservesScheduleMap.Load(reservesScheduleKey); ok {
					reservesSchedule.ticketPlusReserveSchedule.Done = true
					t.reservesScheduleMap.Store(reservesScheduleKey, reservesSchedule)
				}

				reservedInformationForLog := make([]string, len(ticketReserveInformation.Products))
				for idx, val := range ticketReserveInformation.Products {
					reservedInformationForLog[idx] = val.TicketAreaName
				}
				t.logger.Info(fmt.Sprintf("step: get status ok: %s, mobile: %s, event id: %s", strings.Join(reservedInformationForLog, ","), ticketPlusReserveRequest.Mobile, ticketPlusReserveRequest.EventID))
				return
			}
		}(ctx, ticketPlusReserveRequest)

		t.reservesScheduleMap.Store(reservesScheduleKey, &reservesSchedule{
			ticketPlusReserveSchedule: ticketPlusReserveRequest,
			cancelFunc:                cancel,
			done:                      done,
		})

	}

	return processReservesStatusResult, nil
}

func (t *ticketPlus) GetReservesSchedule() map[string]*domain.TicketPlusReserveSchedule {
	reservesScheduleMap := make(map[string]*domain.TicketPlusReserveSchedule)
	t.reservesScheduleMap.Range(func(key string, value *reservesSchedule) bool {
		reservesScheduleMap[key] = value.ticketPlusReserveSchedule
		return true
	})
	return reservesScheduleMap
}

func (t *ticketPlus) DeleteReservesSchedule(scheduleID string) error {
	timeoutDuration := cancelTimeoutDuration
	timer := time.NewTimer(timeoutDuration)
	defer timer.Stop()

	if reservesScheduleInstance, ok := t.reservesScheduleMap.Load(scheduleID); ok {
		reservesScheduleInstance.cancelFunc()
		timer.Reset(timeoutDuration)
		select {
		case <-timer.C:
			return errors.New("process timeout")
		case <-reservesScheduleInstance.done:
		}
		t.reservesScheduleMap.Delete(scheduleID)
		return nil
	} else {
		return errors.New("not exist")
	}
}

func maskMobile(mobile string) string {
	center := len(mobile) / 2

	return mobile[:center-2] + "****" + mobile[center+2:]
}
