package trading

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/domain"
	assetMemoryRepo "github.com/superj80820/system-design/exchange/repository/asset/memory"
	orderMysqlReop "github.com/superj80820/system-design/exchange/repository/order/mysql"
	tradingMySQLAndMongoRepo "github.com/superj80820/system-design/exchange/repository/trading/mysqlandmongo"
	memoryMQKit "github.com/superj80820/system-design/kit/mq/memory"
	ormKit "github.com/superj80820/system-design/kit/orm"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/superj80820/system-design/exchange/usecase/asset"
	"github.com/superj80820/system-design/exchange/usecase/clearing"
	"github.com/superj80820/system-design/exchange/usecase/matching"
	"github.com/superj80820/system-design/exchange/usecase/order"
	utilKit "github.com/superj80820/system-design/kit/util"
)

var (
	liabilityUser = 1
	userAID       = 2
	userBID       = 3
	currencyMap   = map[string]int{
		"BTC":  1,
		"USDT": 2,
	}
)

type testSetup struct {
	syncTradingUseCase domain.SyncTradingUseCase
	userAssetUseCase   domain.UserAssetUseCase
	matchingUseCase    domain.MatchingUseCase
	createTradingEvent func(userID, previousID, sequenceID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) *domain.TradingEvent
	teardownFn         func()
}

func testSetupFn() (*testSetup, error) {
	ctx := context.Background()

	quotationSchemaSQL, err := os.ReadFile("../../repository/quotation/mysqlandredis/schema.sql")
	if err != nil {
		panic(err)
	}
	tradingSchemaSQL, err := os.ReadFile("../../repository/trading/mysqlandmongo/schema.sql")
	if err != nil {
		panic(err)
	}
	orderSchemaSQL, err := os.ReadFile("../../repository/order/mysql/schema.sql")
	if err != nil {
		panic(err)
	}
	candleSchemaSQL, err := os.ReadFile("../../repository/candle/schema.sql")
	if err != nil {
		panic(err)
	}
	sequencerSchemaSQL, err := os.ReadFile("../../repository/sequencer/kafkaandmysql/schema.sql")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./schema.sql", []byte(string(quotationSchemaSQL)+"\n"+string(tradingSchemaSQL)+"\n"+string(candleSchemaSQL)+"\n"+string(orderSchemaSQL)+"\n"+string(sequencerSchemaSQL)), 0644)
	if err != nil {
		panic(err)
	}
	mysqlDBName := "db"
	mysqlDBUsername := "root"
	mysqlDBPassword := "password"
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8"),
		mysql.WithDatabase(mysqlDBName),
		mysql.WithUsername(mysqlDBUsername),
		mysql.WithPassword(mysqlDBPassword),
		mysql.WithScripts(filepath.Join(".", "schema.sql")),
	)
	if err != nil {
		panic(err)
	}
	err = os.Remove("./schema.sql")
	if err != nil {
		panic(err)
	}
	mysqlDBHost, err := mysqlContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	mysqlDBPort, err := mysqlContainer.MappedPort(ctx, "3306")
	if err != nil {
		panic(err)
	}
	mysqlDB, err := ormKit.CreateDB(
		ormKit.UseMySQL(
			fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				mysqlDBUsername,
				mysqlDBPassword,
				mysqlDBHost,
				mysqlDBPort.Port(),
				mysqlDBName,
			)))
	if err != nil {
		panic(err)
	}

	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		panic(err)
	}
	mongoHost, err := mongodbContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	mongoPort, err := mongodbContainer.MappedPort(ctx, "27017")
	if err != nil {
		panic(err)
	}
	mongoDB, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://"+mongoHost+":"+mongoPort.Port()))
	if err != nil {
		panic(err)
	}
	eventsCollection := mongoDB.Database("exchange").Collection("events")
	eventsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"sequence_id": -1,
		},
		Options: options.Index().SetUnique(true),
	})

	tradingEventMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)
	tradingResultMQTopic := memoryMQKit.CreateMemoryMQ(ctx, 100)

	orderRepo := orderMysqlReop.CreateOrderRepo(mysqlDB)
	assetRepo := assetMemoryRepo.CreateAssetRepo()
	tradingRepo := tradingMySQLAndMongoRepo.CreateTradingRepo(ctx, eventsCollection, mysqlDB, tradingEventMQTopic, tradingResultMQTopic)
	matchingUseCase := matching.CreateMatchingUseCase()
	userAssetUseCase := asset.CreateUserAssetUseCase(assetRepo)
	orderUserCase := order.CreateOrderUseCase(userAssetUseCase, tradingRepo, orderRepo, currencyMap["BTC"], currencyMap["USDT"])
	clearingUseCase := clearing.CreateClearingUseCase(userAssetUseCase, orderUserCase, currencyMap["BTC"], currencyMap["USDT"])

	uniqueIDGenerate, err := utilKit.GetUniqueIDGenerate()
	if err != nil {
		return nil, errors.Wrap(err, "get unique id generate failed")
	}

	syncTradingUseCase := CreateSyncTradingUseCase(context.Background(), matchingUseCase, userAssetUseCase, orderUserCase, clearingUseCase)
	createTradingEvent := func(userID, previousID, sequenceID int, direction domain.DirectionEnum, price, quantity decimal.Decimal) *domain.TradingEvent {
		return &domain.TradingEvent{
			EventType:  domain.TradingEventCreateOrderType,
			SequenceID: sequenceID,
			PreviousID: previousID,
			UniqueID:   int(uniqueIDGenerate.Generate().GetInt64()),

			OrderRequestEvent: &domain.OrderRequestEvent{
				UserID:    userID,
				Direction: direction,
				Price:     price,
				Quantity:  quantity,
			},

			CreatedAt: time.Now(),
		}
	}

	return &testSetup{
		syncTradingUseCase: syncTradingUseCase,
		userAssetUseCase:   userAssetUseCase,
		matchingUseCase:    matchingUseCase,
		createTradingEvent: createTradingEvent,
		teardownFn: func() {
			err := mysqlContainer.Terminate(ctx)
			if err != nil {
				panic(err)
			}
			err = mongodbContainer.Terminate(ctx)
			if err != nil {
				panic(err)
			}
		},
	}, nil
}

func TestTrading(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test no money should not create order",
			fn: func(t *testing.T) {
				testSetup, err := testSetupFn()
				assert.Nil(t, err)
				defer testSetup.teardownFn()

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)))
				assert.ErrorIs(t, err, domain.LessAmountErr)
			},
		},
		{
			scenario: "test buy",
			fn: func(t *testing.T) {
				testSetup, err := testSetupFn()
				assert.Nil(t, err)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["USDT"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userBID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userBID, currencyMap["USDT"], decimal.NewFromInt(1000000)))

				asset, err := testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "997917.66", asset.Available.String())
				assert.Equal(t, "2082.34", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 2, domain.DirectionBuy, decimal.NewFromFloat(2087.6), decimal.NewFromInt(5)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "987479.66", asset.Available.String())
				assert.Equal(t, "12520.34", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userBID, 2, 3, domain.DirectionSell, decimal.NewFromFloat(2080.9), decimal.NewFromInt(10)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1012520.34", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "999990", asset.Available.String())
				assert.Equal(t, "4", asset.Frozen.String())
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "987479.66", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000006", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
			},
		},
		{
			scenario: "test sell",
			fn: func(t *testing.T) {
				testSetup, err := testSetupFn()
				assert.Nil(t, err)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["USDT"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userBID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userBID, currencyMap["USDT"], decimal.NewFromInt(1000000)))

				asset, err := testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 0, 1, domain.DirectionSell, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "999999", asset.Available.String())
				assert.Equal(t, "1", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2085.34), decimal.NewFromInt(3)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1000000", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "999996", asset.Available.String())
				assert.Equal(t, "4", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())

				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userBID, 2, 3, domain.DirectionBuy, decimal.NewFromFloat(2090.34), decimal.NewFromInt(5)))
				assert.Nil(t, err)
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "989571.3", asset.Available.String())
				assert.Equal(t, "2090.34", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userBID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "1000004", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "1008338.36", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(userAID, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "999996", asset.Available.String())
				assert.Equal(t, "0", asset.Frozen.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["BTC"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
				asset, err = testSetup.userAssetUseCase.GetAsset(liabilityUser, currencyMap["USDT"])
				assert.Nil(t, err)
				assert.Equal(t, "-2000000", asset.Available.String())
			},
		},
		{
			scenario: "test order book",
			fn: func(t *testing.T) {
				testSetup, err := testSetupFn()
				assert.Nil(t, err)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["USDT"], decimal.NewFromInt(1000000)))

				// buy  2082.34 1
				// sell 2087.6  2
				// buy  2087.8  1
				// buy  2085.01 5
				// sell 2088.02 3
				// sell 2087.60 6
				// buy  2081.11 7
				// buy  2086.0  3
				// buy  2088.33 1
				// sell 2086.54 2
				// sell 2086.55 5
				// buy  2086.55 3

				// 2088.02 3
				// 2087.60 6
				// 2086.55 4
				// ---------
				// 2086.55
				// ---------
				// 2086.00 3
				// 2085.01 5
				// 2082.34 1
				// 2081.11 7
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 2, 3, domain.DirectionBuy, decimal.NewFromFloat(2087.8), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 3, 4, domain.DirectionBuy, decimal.NewFromFloat(2085.01), decimal.NewFromInt(5)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 4, 5, domain.DirectionSell, decimal.NewFromFloat(2088.02), decimal.NewFromInt(3)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 5, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 6, 7, domain.DirectionBuy, decimal.NewFromFloat(2081.11), decimal.NewFromInt(7)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 7, 8, domain.DirectionBuy, decimal.NewFromFloat(2086.0), decimal.NewFromInt(3)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 8, 9, domain.DirectionBuy, decimal.NewFromFloat(2088.33), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 9, 10, domain.DirectionSell, decimal.NewFromFloat(2086.54), decimal.NewFromInt(2)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 10, 11, domain.DirectionSell, decimal.NewFromFloat(2086.55), decimal.NewFromInt(5)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 11, 12, domain.DirectionBuy, decimal.NewFromFloat(2086.55), decimal.NewFromInt(3)))
				assert.Nil(t, err)

				// test all
				{
					orderBook := testSetup.matchingUseCase.GetOrderBook(10)
					buyExpected := []struct {
						price    string
						quantity string
					}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}, {price: "2082.34", quantity: "1"}, {price: "2081.11", quantity: "7"}}
					for idx, val := range orderBook.Buy {
						assert.Equal(t, buyExpected[idx].price, val.Price.String())
						assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
					}
					sellExpected := []struct {
						price    string
						quantity string
					}{{price: "2086.55", quantity: "4"}, {price: "2087.6", quantity: "6"}, {price: "2088.02", quantity: "3"}}
					for idx, val := range orderBook.Sell {
						assert.Equal(t, sellExpected[idx].price, val.Price.String())
						assert.Equal(t, sellExpected[idx].quantity, val.Quantity.String())
					}
				}

				// test with max depth
				{
					orderBook := testSetup.matchingUseCase.GetOrderBook(2)
					assert.Equal(t, 2, len(orderBook.Buy))
					assert.Equal(t, 2, len(orderBook.Sell))
					buyExpected := []struct {
						price    string
						quantity string
					}{{price: "2086", quantity: "3"}, {price: "2085.01", quantity: "5"}}
					for idx, val := range orderBook.Buy {
						assert.Equal(t, buyExpected[idx].price, val.Price.String())
						assert.Equal(t, buyExpected[idx].quantity, val.Quantity.String())
					}
				}
			},
		},
		{
			scenario: "test get duplicate event",
			fn: func(t *testing.T) {
				testSetup, err := testSetupFn()
				assert.Nil(t, err)
				defer testSetup.teardownFn()

				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["BTC"], decimal.NewFromInt(1000000)))
				assert.Nil(t, testSetup.userAssetUseCase.LiabilityUserTransfer(userAID, currencyMap["USDT"], decimal.NewFromInt(1000000)))
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 0, 1, domain.DirectionBuy, decimal.NewFromFloat(2082.34), decimal.NewFromInt(1)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)))
				assert.Nil(t, err)
				_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 2, 3, domain.DirectionBuy, decimal.NewFromFloat(2087.8), decimal.NewFromInt(1)))
				assert.Nil(t, err)

				// test get duplicate event
				{
					_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 2, domain.DirectionSell, decimal.NewFromFloat(2087.6), decimal.NewFromInt(2)))
					assert.ErrorIs(t, err, domain.ErrGetDuplicateEvent)
				}

				// test miss event
				{
					_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 5, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)))
					assert.ErrorIs(t, err, domain.ErrMissEvent)
				}

				// TODO: test think maybe no need previous
				// test previous id not correct
				{
					_, err = testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, 1, 6, domain.DirectionSell, decimal.NewFromFloat(2087.60), decimal.NewFromInt(6)))
					assert.ErrorIs(t, err, domain.ErrPreviousIDNotCorrect)
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}

func BenchmarkTrading(b *testing.B) {
	ctx := context.Background()

	testSetup, _ := testSetupFn()
	defer testSetup.teardownFn()

	testSetup.userAssetUseCase.LiabilityUserTransfer(ctx, userAID, currencyMap["BTC"], decimal.NewFromInt(1000000))
	testSetup.userAssetUseCase.LiabilityUserTransfer(ctx, userAID, currencyMap["USDT"], decimal.NewFromInt(1000000))

	direction := []domain.DirectionEnum{domain.DirectionBuy, domain.DirectionSell}
	for i := 0; i < b.N; i++ {
		randNum := rand.Float64() * 10
		testSetup.syncTradingUseCase.CreateOrder(testSetup.createTradingEvent(userAID, i, i+1, direction[i%2], decimal.NewFromFloat(randNum), decimal.NewFromInt(3)))
	}
}
