package actress

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	utilKit "github.com/superj80820/system-design/kit/util"
)

type actressReverseIndexUseCase struct {
	reverseIndex      *utilKit.ReverseIndex
	actressRepository domain.ActressRepo
}

func (a *actressReverseIndexUseCase) AddData(actressName string, actressID string) {
	a.reverseIndex.AddData(actressName, actressID)
}

func (a *actressReverseIndexUseCase) Search(actressName string) ([]int32, error) {
	searchResult := a.reverseIndex.Search(actressName)
	actressIDs := make([]int32, len(searchResult))
	for idx, val := range searchResult {
		id, err := strconv.ParseInt(*val, 10, 32)
		if err != nil {
			return nil, errors.Wrap(err, "string covert to int32 failed")
		}
		actressIDs[idx] = int32(id)
	}
	return actressIDs, nil
}

func CreateActressReverseIndexUseCase(ctx context.Context, actressRepository domain.ActressRepo) (domain.ActressReverseIndexUseCase, error) {
	a := &actressReverseIndexUseCase{
		reverseIndex: utilKit.CreateReverseIndex(),
	}

	for page := 1; ; page++ {
		actresses, _, isEnd, err := actressRepository.GetActressesByPagination(ctx, page, 1000)
		if err != nil {
			return nil, errors.Wrap(err, "get actresses by pagination failed")
		}
		for _, val := range actresses {
			a.AddData(val.Name, val.ID)
		}
		if isEnd {
			break
		}
	}

	return a, nil
}
