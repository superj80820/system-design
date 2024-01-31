package asset

import (
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
)

func TestAssetUseCase(t *testing.T) {
	// test race condition
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	userAssetUseCase := CreateUserAssetUseCase(assetRepo)

	assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(2, 1, decimal.NewFromInt(2000)))
	assert.Nil(t, userAssetUseCase.LiabilityUserTransfer(3, 1, decimal.NewFromInt(2000)))

	wg := new(sync.WaitGroup)
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()

			userAssetUseCase.Transfer(domain.AssetTransferAvailableToAvailable, 2, 3, 1, decimal.NewFromInt(1))
		}()
	}
	wg.Add(1500)
	for i := 0; i < 1500; i++ {
		go func() {
			defer wg.Done()

			userAssetUseCase.Transfer(domain.AssetTransferAvailableToAvailable, 3, 2, 1, decimal.NewFromInt(1))
		}()
	}
	wg.Wait()
	userAAssert, err := userAssetUseCase.GetAsset(2, 1)
	assert.Nil(t, err)
	userBAssert, err := userAssetUseCase.GetAsset(3, 1)
	assert.Nil(t, err)
	assert.Equal(t, decimal.NewFromInt(2500), userAAssert.Available)
	assert.Equal(t, decimal.NewFromInt(1500), userBAssert.Available)
}
