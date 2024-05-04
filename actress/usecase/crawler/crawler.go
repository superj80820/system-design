package crawler

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type actressCrawlerUseCase struct {
	logger                     loggerKit.Logger
	actressCrawlerRepo         domain.ActressCrawlerRepo
	actressRepo                domain.ActressRepo
	facePlusPlusUseCase        domain.FacePlusPlusUseCase
	actressReverseIndexUseCase domain.ActressReverseIndexUseCase

	enableAddFace   bool
	enableBindFace  bool
	crawlerPage     int
	crawlerLimit    int
	faceAreaPercent float64
	doneCh          chan struct{}
	errLock         *sync.Mutex
	err             error
}

func CreateActressCrawlerUseCase(ctx context.Context, logger loggerKit.Logger, actressCrawlerRepo domain.ActressCrawlerRepo, actressRepo domain.ActressRepo, facePlusPlusUseCase domain.FacePlusPlusUseCase, actressReverseIndexUseCase domain.ActressReverseIndexUseCase, crawlerLimit int, enableAddFace, enableBindFace bool, crawlerStartPage int) domain.ActressCrawlerUseCase {
	a := &actressCrawlerUseCase{
		logger:                     logger,
		actressCrawlerRepo:         actressCrawlerRepo,
		actressRepo:                actressRepo,
		facePlusPlusUseCase:        facePlusPlusUseCase,
		actressReverseIndexUseCase: actressReverseIndexUseCase,
		crawlerPage:                crawlerStartPage,
		crawlerLimit:               crawlerLimit,
		faceAreaPercent:            0.1,
		errLock:                    new(sync.Mutex),
		doneCh:                     make(chan struct{}),
		enableAddFace:              enableAddFace,
		enableBindFace:             enableBindFace,
	}
	return a
}

func (a *actressCrawlerUseCase) Process(ctx context.Context) {
	setErrAndDone := func(err error) {
		defer close(a.doneCh)

		a.errLock.Lock()
		a.err = err
		a.errLock.Unlock()
	}

	tickerCreateActressAndFace := time.NewTicker(1 * time.Second)
	defer tickerCreateActressAndFace.Stop()
	tickerBindFaceToFaceSet := time.NewTicker(5 * time.Second)
	defer tickerBindFaceToFaceSet.Stop()

	for {
		select {
		case <-tickerCreateActressAndFace.C:
			if !a.enableAddFace {
				break
			}
			if err := a.createActressAndFace(); err != nil {
				setErrAndDone(errors.Wrap(err, "add face failed"))
				return
			}
		case <-tickerBindFaceToFaceSet.C:
			if !a.enableBindFace {
				break
			}
			if err := a.bindFaceToFaceSet(); err != nil {
				setErrAndDone(errors.Wrap(err, "bind face failed"))
				return
			}
		case <-ctx.Done():
			setErrAndDone(nil)
			return
		}
	}

}

func (a *actressCrawlerUseCase) createActressAndFace() error {
	actressCrawlerDataPagination, err := a.actressCrawlerRepo.GetActresses(a.crawlerPage, a.crawlerLimit)
	if errors.Is(err, domain.ErrNoData) {
		a.crawlerPage = 1
		return nil
	} else if err != nil {
		return errors.Wrap(err, "get actresses from crawler failed")
	}
	a.logger.Debug("actress crawler data pagination",
		loggerKit.Int("count", actressCrawlerDataPagination.Count),
		loggerKit.Int("current-page", actressCrawlerDataPagination.CurrentPage),
		loggerKit.Int("total-page", actressCrawlerDataPagination.TotalPages),
	)

	for _, actressCrawlerDataProvider := range actressCrawlerDataPagination.Items {
		if err := func(actressCrawlerDataProvider domain.ActressCrawlerProvider) error {
			actressCrawlerData, err := actressCrawlerDataProvider.GetWithValid()
			if err != nil {
				return errors.Wrap(domain.ErrNormalContinue, fmt.Sprintf("get actress crawler data provider failed, err: %v", err))
			}
			actressCrawlerImage, err := actressCrawlerDataProvider.GetImage()
			if err != nil {
				return errors.Wrap(domain.ErrNormalContinue, fmt.Sprintf("get actress crawler image failed, err: %v", err))
			}

			a.logger.Debug("actress crawler data",
				loggerKit.String("name", actressCrawlerData.ActressName),
				loggerKit.String("preview-url", actressCrawlerData.ActressPreviewURL),
				loggerKit.String("preview-image-type", string(actressCrawlerData.PreviewImageType)),
			)

			var actressID string
			actress, err := a.actressRepo.GetActressByName(actressCrawlerData.ActressName)
			if errors.Is(err, domain.ErrNoData) {
				actressID, err = a.actressRepo.AddActress(actressCrawlerData.ActressName, actressCrawlerData.ActressPreviewURL)
				if err != nil {
					return errors.Wrap(err, "add actress failed")
				}
				a.actressReverseIndexUseCase.AddData(actressCrawlerData.ActressName, actressID)
			} else if err != nil {
				return errors.Wrap(err, "get actress failed")
			} else {
				actressID = actress.ID

				if err := a.actressRepo.SetActressPreview(actressID, actressCrawlerData.ActressPreviewURL); err != nil {
					return errors.Wrap(err, "update actress preview url failed")
				}

				faces, err := a.actressRepo.GetFacesByActressID(actressID)
				if err != nil {
					return errors.Wrap(err, "get faces by actress id failed")
				}

				if len(faces) > 0 {
					return errors.Wrap(domain.ErrNormalContinue, "already has faces")
				}
			}

			faceDetect, err := a.facePlusPlusUseCase.Detect(actressCrawlerImage.GetRawData())
			if errors.Is(err, domain.ErrRateLimit) {
				sleepDuration := 10 * time.Second
				a.logger.Warn("get rate limit error", loggerKit.String("reason", err.Error()), loggerKit.Duration("sleep-duration", sleepDuration))
				time.Sleep(sleepDuration)
				return errors.Wrap(domain.ErrWarningContinue, "get rate limit error")
			} else if err != nil {
				a.logger.Warn("detect face error",
					loggerKit.String("reason", err.Error()),
					loggerKit.String("image-type", string(actressCrawlerData.PreviewImageType)),
					loggerKit.String("image-url", actressCrawlerData.ActressPreviewURL),
					loggerKit.String("name", actressCrawlerData.ActressName),
				)
				return errors.Wrap(domain.ErrWarningContinue, fmt.Sprintf("detect failed, err: %v", err))
			}

			if len(faceDetect.Faces) != 1 {
				return errors.Wrap(domain.ErrNormalContinue, "unexpected detect faces count")
			}

			imageArea, err := actressCrawlerImage.GetImageArea()
			if err != nil {
				return errors.Wrap(err, "get crawler image area failed")
			}
			detectImageArea, err := utilKit.GetImageArea(float64(faceDetect.Faces[0].FaceRectangle.Width), float64(faceDetect.Faces[0].FaceRectangle.Height))
			if err != nil {
				return errors.Wrap(err, "get detect image area failed")
			}
			if detectImageArea/imageArea < a.faceAreaPercent {
				a.logger.Warn("detect area too small", loggerKit.String("image-url", actressCrawlerData.ActressPreviewURL), loggerKit.Float64("image-area", detectImageArea/imageArea), loggerKit.Float64("detect-image-area", detectImageArea), loggerKit.Float64("image-area", imageArea))
				return errors.Wrap(domain.ErrWarningContinue, "detect image too small")
			}

			faceDetectCount := len(faceDetect.Faces)
			if faceDetectCount != 1 {
				if err := a.actressRepo.SetActressPreview(actressID, ""); err != nil {
					return errors.Wrap(err, "update actress preview url failed")
				}

				return errors.Wrap(domain.ErrWarningContinue, "face detect count error, count: "+strconv.Itoa(faceDetectCount))
			}

			_, err = a.actressRepo.AddFace(actressID, faceDetect.Faces[0].FaceToken, actressCrawlerData.ActressPreviewURL)
			if err != nil {
				return errors.Wrap(err, "add actress with face failed")
			}

			return nil
		}(actressCrawlerDataProvider); errors.Is(err, domain.ErrNormalContinue) {
			a.logger.Debug("continue add face", loggerKit.String("reason", err.Error()))
		} else if errors.Is(err, domain.ErrWarningContinue) {
			a.logger.Warn("continue add face", loggerKit.String("reason", err.Error()))
		} else if err != nil {
			return errors.Wrap(err, "add face failed")
		}
	}

	a.crawlerPage += 1
	return nil
}

func (a *actressCrawlerUseCase) bindFaceToFaceSet() error {
	faces, err := a.actressRepo.GetFacesByStatus(domain.FaceStatusNotInFaceSet)
	if err != nil {
		errors.Wrap(err, "get actresses by status failed")
	}

	a.logger.Debug("bind face", loggerKit.Int("count", len(faces)))

	for _, face := range faces {
		if err := func() error {
			faceSetTokenByAdd, err := a.facePlusPlusUseCase.Add([]string{face.Token}) // TODO: optimize concurrency
			if errors.Is(err, domain.ErrRateLimit) {
				sleepDuration := 10 * time.Second
				a.logger.Warn("get rate limit error", loggerKit.String("reason", err.Error()), loggerKit.Duration("sleep-duration", sleepDuration))
				time.Sleep(sleepDuration)
				return errors.Wrap(domain.ErrWarningContinue, "get rate limit error")
			} else if errors.Is(err, domain.ErrInvalidData) {
				a.logger.Warn("get invalid data", loggerKit.String("reason", err.Error()))
				if err := a.actressRepo.RemoveFace(face.ID); err != nil {
					return errors.Wrap(err, "delete face failed")
				}
				return nil
			} else if err != nil {
				return errors.Wrap(err, "face plus plus face set add face failed")
			}

			if err := a.actressRepo.SetFaceStatus(face.ID, faceSetTokenByAdd, domain.FaceStatusAlreadyInFaceSet); err != nil {
				return errors.Wrap(err, "set face status failed")
			}

			return nil
		}(); errors.Is(err, domain.ErrNormalContinue) {
			a.logger.Debug("continue bind face", loggerKit.String("reason", err.Error()))
		} else if errors.Is(err, domain.ErrWarningContinue) {
			a.logger.Warn("continue bind face", loggerKit.String("reason", err.Error()))
		} else if err != nil {
			return errors.Wrap(err, "bind face failed")
		}
	}

	return nil
}

func (a *actressCrawlerUseCase) Done() <-chan struct{} {
	return a.doneCh
}

func (a *actressCrawlerUseCase) Err() error {
	a.errLock.Lock()
	defer a.errLock.Unlock()

	return a.err
}
