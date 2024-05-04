package actress

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	"github.com/superj80820/system-design/kit/code"
)

type actressUseCase struct {
	actressReverseIndexUseCase domain.ActressReverseIndexUseCase
	actressRepository          domain.ActressRepo
	facePlusPlusUseCase        domain.FacePlusPlusUseCase
}

func (a *actressUseCase) SearchActressByName(ctx context.Context, name string) ([]*domain.Actress, error) {
	searchResult, err := a.actressReverseIndexUseCase.Search(name)
	if err != nil {
		return nil, errors.Wrap(err, "search by reverse index failed")
	}

	actresses, err := a.actressRepository.GetActresses(ctx, searchResult...)
	if err != nil {
		return nil, errors.Wrap(err, "get actresses failed")
	}
	return actresses, nil
}

func CreateActressUseCase(ctx context.Context, actressRepository domain.ActressRepo, actressReverseIndexUseCase domain.ActressReverseIndexUseCase, facePlusPlusUseCase domain.FacePlusPlusUseCase) (domain.ActressUseCase, error) {
	return &actressUseCase{
		actressRepository:          actressRepository,
		actressReverseIndexUseCase: actressReverseIndexUseCase,
		facePlusPlusUseCase:        facePlusPlusUseCase,
	}, nil
}

func (a *actressUseCase) AddFavorite(userID, actressID string) error {
	err := a.actressRepository.AddFavorite(userID, actressID)
	if errors.Is(err, domain.ErrAlreadyDone) {
		return code.CreateErrorCode(http.StatusConflict).AddErrorMetaData(err)
	} else if err != nil {
		return errors.Wrap(err, "add favorite failed")
	}
	return nil
}

func (a *actressUseCase) GetActress(id string) (*domain.Actress, error) {
	actress, err := a.actressRepository.GetActress(id)
	if err != nil {
		return nil, errors.Wrap(err, "add favorite failed")
	}
	return actress, nil
}

func (a *actressUseCase) GetFavorites(userID string) ([]*domain.Actress, error) {
	favorites, err := a.actressRepository.GetFavorites(userID)
	if err != nil {
		return nil, errors.Wrap(err, "add favorite failed")
	}
	return favorites, nil
}

func (a *actressUseCase) RemoveFavorite(userID string, actressID string) error {
	if err := a.actressRepository.RemoveFavorite(userID, actressID); err != nil {
		return errors.Wrap(err, "remove favorite failed")
	}
	return nil
}

func (a *actressUseCase) SearchActressByFace(faceImage []byte) ([]*domain.Actress, error) {
	result, err := a.facePlusPlusUseCase.SearchAllFaceSets(faceImage)
	if err != nil {
		return nil, errors.Wrap(err, "search all face sets failed")
	}
	actresses := make([]*domain.Actress, len(result.SearchResults))
	for idx, searchResult := range result.SearchResults { // TODO: york optimize to one sql
		actress, err := a.actressRepository.GetActressByFaceToken(searchResult.FaceToken)
		if err != nil {
			return nil, errors.Wrap(err, "get actress by face token failed")
		}
		actresses[idx] = actress
	}
	return actresses, nil
}
