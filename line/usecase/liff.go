package usecase

import "github.com/superj80820/system-design/domain"

type lineLIFFUseCase struct{}

func CreateLIFFUseCase() domain.LineLIFFUseCase {
	return &lineLIFFUseCase{}
}

func (l *lineLIFFUseCase) VerifyLIFF(liffID, accessToken string) error {
	return nil
}
