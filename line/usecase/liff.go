package usecase

import (
	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type lineLIFFUseCase struct{}

func CreateLIFFUseCase() domain.LineLIFFUseCase {
	return &lineLIFFUseCase{}
}

func (l *lineLIFFUseCase) VerifyLIFF(liffID, accessToken string) error {
	if err := l.VerifyLIFF(liffID, accessToken); err != nil {
		return errors.Wrap(err, "verify liff failed")
	}
	return nil
}
