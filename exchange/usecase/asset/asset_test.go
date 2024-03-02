package asset

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
)

func TestAssetUseCase(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		scenario string
		fn       func(*testing.T)
	}{
		{
			scenario: "test race condition",
			fn: func(t *testing.T) {
				assetMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100, 100*time.Millisecond)
				assetRepo := assetMemoryRepo.CreateAssetRepo(assetMQTopic)
				userAssetUseCase := CreateUserAssetUseCase(assetRepo)

				_, err := userAssetUseCase.LiabilityUserTransfer(ctx, 2, 1, decimal.NewFromInt(2000))
				assert.Nil(t, err)
				_, err = userAssetUseCase.LiabilityUserTransfer(ctx, 3, 1, decimal.NewFromInt(2000))
				assert.Nil(t, err)

				wg := new(sync.WaitGroup)
				wg.Add(1000)
				for i := 0; i < 1000; i++ {
					go func() {
						defer wg.Done()

						userAssetUseCase.Transfer(ctx, domain.AssetTransferAvailableToAvailable, 2, 3, 1, decimal.NewFromInt(1))
					}()
				}
				wg.Add(1500)
				for i := 0; i < 1500; i++ {
					go func() {
						defer wg.Done()

						userAssetUseCase.Transfer(ctx, domain.AssetTransferAvailableToAvailable, 3, 2, 1, decimal.NewFromInt(1))
					}()
				}
				wg.Wait()
				userAAssert, err := userAssetUseCase.GetAsset(2, 1)
				assert.Nil(t, err)
				userBAssert, err := userAssetUseCase.GetAsset(3, 1)
				assert.Nil(t, err)
				assert.Equal(t, decimal.NewFromInt(2500), userAAssert.Available)
				assert.Equal(t, decimal.NewFromInt(1500), userBAssert.Available)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
