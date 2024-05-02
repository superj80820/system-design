package facepp

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type facePlusPlusWorkPoolUseCase struct {
	logger           loggerKit.Logger
	facePlusPlusRepo domain.FacePlusPlusRepo
	inputCh          chan func()
}

func CreateFacePlusPlusWorkPoolUseCase(logger loggerKit.Logger, facePlusPlusRepo domain.FacePlusPlusRepo, useAPIDuration time.Duration) domain.FacePlusPlusWorkPoolUseCase {
	inputCh := make(chan func(), 1000)

	go func() {
		for inputHandleFunc := range inputCh {
			logger.Debug("request face plus plus api", loggerKit.Time("time", time.Now()))
			inputHandleFunc()
			time.Sleep(useAPIDuration)
		}
	}()

	return &facePlusPlusWorkPoolUseCase{
		logger:           logger,
		facePlusPlusRepo: facePlusPlusRepo,
		inputCh:          inputCh,
	}
}

func (f *facePlusPlusWorkPoolUseCase) Add(faceSetID string, faceTokens []string) (*domain.FaceAdd, error) {
	outPutCh := make(chan *domain.FaceAdd)
	errCh := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.inputCh <- func() {
		faceAdd, err := f.facePlusPlusRepo.Add(faceSetID, faceTokens)
		if err != nil {
			select {
			case errCh <- errors.Wrap(err, "add face failed"):
			case <-ctx.Done():
			}
			return
		}
		select {
		case outPutCh <- faceAdd:
		case <-ctx.Done():
		}
	}
	select {
	case outPut := <-outPutCh:
		return outPut, nil
	case err := <-errCh:
		return nil, errors.Wrap(err, "async add face failed")
	case <-ctx.Done():
		return nil, errors.New("operation time out")
	}
}

func (f *facePlusPlusWorkPoolUseCase) Detect(image []byte) (*domain.FaceDetect, error) {
	outPutCh := make(chan *domain.FaceDetect)
	errCh := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.inputCh <- func() {
		detect, err := f.facePlusPlusRepo.Detect(image)
		if err != nil {
			select {
			case errCh <- errors.Wrap(err, "detect face failed"):
			case <-ctx.Done():
			}
			return
		}
		select {
		case outPutCh <- detect:
		case <-ctx.Done():
		}
	}
	select {
	case outPut := <-outPutCh:
		return outPut, nil
	case err := <-errCh:
		return nil, errors.Wrap(err, "async detect face failed")
	case <-ctx.Done():
		return nil, errors.New("operation time out")
	}
}

func (f *facePlusPlusWorkPoolUseCase) GetFaceSetDetail(faceSetID string) (*domain.FaceSetDetail, error) {
	outPutCh := make(chan *domain.FaceSetDetail)
	errCh := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.inputCh <- func() {
		faceSetDetail, err := f.facePlusPlusRepo.GetFaceSetDetail(faceSetID)
		if err != nil {
			select {
			case errCh <- errors.Wrap(err, "get face set detail failed"):
			case <-ctx.Done():
			}
			return
		}
		select {
		case outPutCh <- faceSetDetail:
		case <-ctx.Done():
		}
	}
	select {
	case outPut := <-outPutCh:
		return outPut, nil
	case err := <-errCh:
		return nil, errors.Wrap(err, "async get face set detail failed")
	case <-ctx.Done():
		return nil, errors.New("operation time out")
	}
}

func (f *facePlusPlusWorkPoolUseCase) IsFaceSetFull(faceSetID string) (bool, error) {
	outPutCh := make(chan bool)
	errCh := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.inputCh <- func() {
		faceSetIsFull, err := f.facePlusPlusRepo.IsFaceSetFull(faceSetID)
		if err != nil {
			select {
			case errCh <- errors.Wrap(err, "get face set full information failed"):
			case <-ctx.Done():
			}
			return
		}
		select {
		case outPutCh <- faceSetIsFull:
		case <-ctx.Done():
		}
	}
	select {
	case outPut := <-outPutCh:
		return outPut, nil
	case err := <-errCh:
		return false, errors.Wrap(err, "async get face set full information failed")
	case <-ctx.Done():
		return false, errors.New("operation time out")
	}
}

func (f *facePlusPlusWorkPoolUseCase) Search(faceSetID string, image []byte) (*domain.FaceSearch, error) {
	outPutCh := make(chan *domain.FaceSearch)
	errCh := make(chan error)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f.inputCh <- func() {
		faceSetIsFull, err := f.facePlusPlusRepo.Search(faceSetID, image)
		if err != nil {
			select {
			case errCh <- errors.Wrap(err, "search actress by face failed"):
			case <-ctx.Done():
			}
			return
		}
		select {
		case outPutCh <- faceSetIsFull:
		case <-ctx.Done():
		}
	}
	select {
	case outPut := <-outPutCh:
		return outPut, nil
	case err := <-errCh:
		return nil, errors.Wrap(err, "async search actress by face failed")
	case <-ctx.Done():
		return nil, errors.New("operation time out")
	}
}
