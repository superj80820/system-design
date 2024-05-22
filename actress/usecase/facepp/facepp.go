package facepp

import (
	"time"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
)

type facePlusPlusUseCase struct {
	facePlusPlusWorkPoolUseCase domain.FacePlusPlusWorkPoolUseCase
	faceSetIDs                  []string
}

func CreateFacePlusPlusUseCase(facePlusPlusWorkPoolUseCase domain.FacePlusPlusWorkPoolUseCase, faceSetIDs []string) domain.FacePlusPlusUseCase {
	return &facePlusPlusUseCase{
		facePlusPlusWorkPoolUseCase: facePlusPlusWorkPoolUseCase,
		faceSetIDs:                  faceSetIDs,
	}
}

// SearchAllFaceSets use concurrency will get rate limit error, so use sync way
func (f *facePlusPlusUseCase) SearchAllFaceSets(image []byte) (*domain.FaceSearch, error) {
	allFaceSetsSearchInformation := make([]*domain.FaceSearch, 0, len(f.faceSetIDs))
	for _, faceSet := range f.faceSetIDs {
		if err := func() error {
			faceSearchInformation, err := f.facePlusPlusWorkPoolUseCase.Search(faceSet, image)
			if errors.Is(err, domain.ErrNoData) {
				return nil
			} else if err != nil {
				return errors.Wrap(err, "search failed")
			}
			allFaceSetsSearchInformation = append(allFaceSetsSearchInformation, faceSearchInformation)

			return nil
		}(); err != nil {
			return nil, errors.Wrap(err, "search all face set get error")
		}
	}

	sortedFaceSetsSearchInformation := mergeKSortedByDesc(allFaceSetsSearchInformation)

	return sortedFaceSetsSearchInformation, nil
}

func (f *facePlusPlusUseCase) Add(faceTokens []string) (string, error) {
	for _, faceSetID := range f.faceSetIDs {
		faceSetToken, err := func() (string, error) {
			var sleepDuration time.Duration
			defer func() {
				ifGreaterZeroThenSleep(sleepDuration)
			}()

			isFull, err := f.facePlusPlusWorkPoolUseCase.IsFaceSetFull(faceSetID)
			if err != nil {
				return "", errors.Wrap(err, "check face set full failed")
			}
			if isFull {
				return "", errors.Wrap(domain.ErrNormalContinue, "face set is full")
			}

			faceAdd, err := f.facePlusPlusWorkPoolUseCase.Add(faceSetID, faceTokens)
			if errors.Is(err, domain.ErrAlreadyDone) {
				return faceSetID, nil
			} else if err != nil {
				return "", errors.Wrap(err, "add face failed")
			}
			return faceAdd.FacesetToken, nil
		}()
		if errors.Is(err, domain.ErrNormalContinue) {
			continue
		} else if err != nil {
			return "", errors.Wrap(err, "add face to face set failed")
		}
		return faceSetToken, nil
	}
	return "", errors.New("face set is full, need more face set")
}

func (f *facePlusPlusUseCase) Detect(image []byte) (*domain.FaceDetect, error) {
	faceDetect, err := f.facePlusPlusWorkPoolUseCase.Detect(image)
	if err != nil {
		return nil, errors.Wrap(err, "detect face failed")
	}
	return faceDetect, nil
}

func ifGreaterZeroThenSleep(duration time.Duration) {
	if duration != 0 {
		time.Sleep(duration)
	}
}

func mergeKSortedByDesc(allFaceSetsSearchInformation []*domain.FaceSearch) *domain.FaceSearch {
	for len(allFaceSetsSearchInformation) > 1 {
		var mergeLists []*domain.FaceSearch
		for i := 0; i < len(allFaceSetsSearchInformation); i += 2 {
			var list1, list2 []*domain.FaceSearchResults
			list1 = allFaceSetsSearchInformation[i].SearchResults
			if i+1 < len(allFaceSetsSearchInformation) {
				list2 = allFaceSetsSearchInformation[i+1].SearchResults
			}
			mergeLists = append(mergeLists, &domain.FaceSearch{
				SearchResults: mergeByDesc(list1, list2),
			})
		}
		allFaceSetsSearchInformation = mergeLists
	}
	return allFaceSetsSearchInformation[0]
}

func mergeByDesc(left, right []*domain.FaceSearchResults) []*domain.FaceSearchResults {
	res := make([]*domain.FaceSearchResults, 0, len(left)+len(right))
	var i, j int

	for i < len(left) && j < len(right) {
		if left[i].Confidence >= right[j].Confidence {
			res = append(res, left[i])
			i++
		} else {
			res = append(res, right[j])
			j++
		}
	}

	if i < len(left) {
		res = append(res, left[i:]...)
	} else if j < len(right) {
		res = append(res, right[j:]...)
	}

	return res
}
