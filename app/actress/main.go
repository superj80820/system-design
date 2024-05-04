package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "net/http/pprof"
	"syscall"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	accountMySQLRepo "github.com/superj80820/system-design/auth/repository/account/mysql"
	authMySQLRepo "github.com/superj80820/system-design/auth/repository/auth/mysql"
	"github.com/superj80820/system-design/auth/usecase/auth"
	"gopkg.in/yaml.v3"

	actressDelivery "github.com/superj80820/system-design/actress/delivery"
	actressBackgroundDelivery "github.com/superj80820/system-design/actress/delivery/background"
	facePlusPlusRepo "github.com/superj80820/system-design/actress/repository/facepp"
	actressCrawlerMinnanoRepo "github.com/superj80820/system-design/actress/repository/minnano"
	actressPostgresRepo "github.com/superj80820/system-design/actress/repository/postgres"
	actressUseCase "github.com/superj80820/system-design/actress/usecase/actress"
	actressCrawlerUseCase "github.com/superj80820/system-design/actress/usecase/crawler"
	facePlusPlusUseCase "github.com/superj80820/system-design/actress/usecase/facepp"
	actressLineUseCase "github.com/superj80820/system-design/actress/usecase/line"
	authLineUseCase "github.com/superj80820/system-design/auth/usecase/line"
	awsDelivery "github.com/superj80820/system-design/aws/delivery"
	awsRepo "github.com/superj80820/system-design/aws/repository"
	awsUseCase "github.com/superj80820/system-design/aws/usecase"
	httpMiddlewareKit "github.com/superj80820/system-design/kit/http/middleware"
	redisContainer "github.com/superj80820/system-design/kit/testing/redis/container"
	traceKit "github.com/superj80820/system-design/kit/trace"
	lineDelivery "github.com/superj80820/system-design/line/delivery"
	lineRepo "github.com/superj80820/system-design/line/repository"

	ormKit "github.com/superj80820/system-design/kit/orm"

	redisKit "github.com/superj80820/system-design/kit/cache/redis"
	httpKit "github.com/superj80820/system-design/kit/http"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	redisRateLimitKit "github.com/superj80820/system-design/kit/ratelimit/redis"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func main() {
	utilKit.LoadEnvFile(".env")
	env := utilKit.GetEnvString("ENV", "development")
	lineMaxReplyCount := utilKit.GetEnvInt("LINE_MAX_REPLY_COUNT", 3)
	enableHTTPS := utilKit.GetEnvBool("ENABLE_HTTPS", false)
	httpsCertFilePath := utilKit.GetEnvString("HTTPS_CERT_FILE_PATH", "./fullchain.pem")
	httpsPrivateKeyFilePath := utilKit.GetEnvString("HTTPS_PRIVATE_KEY_FILE_PATH", "./privkey.pem")
	redisURI := utilKit.GetEnvString("REDIS_URI", "")
	enableUserRateLimit := utilKit.GetEnvBool("ENABLE_USER_RATE_LIMIT", false)
	enablePprofServer := utilKit.GetEnvBool("ENABLE_PPROF_SERVER", false)
	lineAPIURL := utilKit.GetEnvString("LINE_API_URL", "https://api.line.me/v2")
	lineDataAPIURL := utilKit.GetEnvString("LINE_API_URL", "https://api-data.line.me/v2")
	lineLoginClientID := utilKit.GetRequireEnvString("LINE_LOGIN_CLIENT_ID")
	lineLoginClientSecret := utilKit.GetRequireEnvString("LINE_LOGIN_CLIENT_SECRET")
	lineMessageAPIToken := utilKit.GetRequireEnvString("LINE_API_TOKEN")
	liffURI := utilKit.GetRequireEnvString("LIFF_URI")
	facePlusPlusAPIURL := utilKit.GetEnvString("FACE_PLUS_PLUS_API_URL", "https://api-us.faceplusplus.com")
	facePlusPlusKey := utilKit.GetRequireEnvString("FACE_PLUS_PLUS_KEY")
	facePlusPlusSecret := utilKit.GetRequireEnvString("FACE_PLUS_PLUS_SECRET")
	facePlusPlusFaceSets := utilKit.GetRequireEnvStringSlice("FACE_PLUS_PLUS_FACESETS")
	enableAddFace := utilKit.GetEnvBool("ENABLE_ADD_FACE", true)
	enableBindFace := utilKit.GetEnvBool("ENABLE_BIND_FACE", true)
	crawlerStartPage := utilKit.GetEnvInt("CRAWLER_START_PAGE", 1)
	accessPrivateKey := utilKit.GetRequireEnvString("ACCESS_PRIVATE_KEY")
	refreshPrivateKey := utilKit.GetRequireEnvString("REFRESH_PRIVATE_KEY")
	postgresURI := utilKit.GetRequireEnvString("POSTGRES_URI")
	facePlusPlusUseAPIDuration := utilKit.GetEnvInt("FACE_PLUS_PLUS_USE_API_DURATION", 3)
	awsAccessKeyID := utilKit.GetEnvString("AWS_ACCESS_KEY_ID", "")
	awsSecretAccessKey := utilKit.GetEnvString("AWS_SECRET_ACCESS_KEY", "")
	dbBackupBucket := utilKit.GetEnvString("DB_BACKUP_BUCKET", "")
	dbBackupRegion := utilKit.GetEnvString("DB_BACKUP_REGION", "")
	dbBackupScript := utilKit.GetEnvString("DB_BACKUP_SCRIPT", "")
	dbBackupFilePath := utilKit.GetEnvString("DB_BACKUP_FILE_PATH", "")
	dbBackupKey := utilKit.GetEnvString("DB_BACKUP_KEY", "")
	dbBackupDuration := utilKit.GetEnvInt("DB_BACKUP_DURATION", 21600)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	templateYaml, err := os.ReadFile("template.yml")
	if err != nil {
		panic(err)
	}
	template := make(map[string]string)
	if err := yaml.Unmarshal(templateYaml, &template); err != nil {
		panic(err)
	}

	if redisURI == "" {
		redisContainer, err := redisContainer.CreateRedis(ctx)
		if err != nil {
			panic(err)
		}
		defer redisContainer.Terminate(ctx)
		redisURI = redisContainer.GetURI()

		log.Println("testcontainers redis uri: ", redisURI)
	}

	ormDB, err := ormKit.CreateDB(ormKit.UsePostgres(postgresURI))
	if err != nil {
		panic(err)
	}

	redisCache, err := redisKit.CreateCache(redisURI, "", 0)
	if err != nil {
		panic(err)
	}

	loggerLevel := loggerKit.DebugLevel
	if env == "production" {
		loggerLevel = loggerKit.InfoLevel
	}
	logger, err := loggerKit.NewLogger("./go.log", loggerLevel, loggerKit.WithRotateLog(1, 10, 10))
	if err != nil {
		panic(err)
	}
	tracer := traceKit.CreateNoOpTracer()

	s3Repo := awsRepo.CreateS3Repo(awsAccessKeyID, awsSecretAccessKey, dbBackupBucket, dbBackupRegion)
	s3UseCase := awsUseCase.CreateS3UseCase(s3Repo)
	actressRepoHandler, err := actressPostgresRepo.CreateActressRepo(ormDB)
	if err != nil {
		panic(err)
	}
	actressCrawlerMinnanoRepoHandler := actressCrawlerMinnanoRepo.CreateActressCrawlerRepo("https://www.minnano-av.com")
	actressLineRepoHandler := actressPostgresRepo.CreateActressLineRepo(redisCache)
	lineMessageRepo := lineRepo.CreateLineMessageRepo(lineAPIURL, lineDataAPIURL, lineMessageAPIToken)
	lineLoginAPIRepo := lineRepo.CreateLineLoginAPIRepo(lineLoginClientID, lineLoginClientSecret)
	lineUserRepo := lineRepo.CreateLineUserRepo(ormDB)
	accountRepo := accountMySQLRepo.CreateAccountRepo(ormDB)
	authRepo, err := authMySQLRepo.CreateAuthRepo(ormDB, accessPrivateKey, refreshPrivateKey)
	if err != nil {
		panic(err)
	}
	lineTemplateRepo, err := lineRepo.CreateLineTemplate(template)
	if err != nil {
		panic(err)
	}
	facePlusPlusRepoHandler := facePlusPlusRepo.CreateFacePlusPlusRepo(
		facePlusPlusAPIURL,
		facePlusPlusKey,
		facePlusPlusSecret,
	)
	facePlusPlusWorkPoolUseCase := facePlusPlusUseCase.CreateFacePlusPlusWorkPoolUseCase(logger, facePlusPlusRepoHandler, time.Duration(facePlusPlusUseAPIDuration)*time.Second)
	facePlusPlusUseCaseHandler := facePlusPlusUseCase.CreateFacePlusPlusUseCase(facePlusPlusWorkPoolUseCase, facePlusPlusFaceSets)

	actressLineUseCaseHandler := actressLineUseCase.CreateActressUseCase(actressRepoHandler, actressLineRepoHandler, lineMessageRepo, lineTemplateRepo, facePlusPlusUseCaseHandler, liffURI, logger, lineMaxReplyCount)
	actressReverseIndexUseCase, err := actressUseCase.CreateActressReverseIndexUseCase(ctx, actressRepoHandler)
	if err != nil {
		panic(err)
	}
	actressUseCaseHandler, err := actressUseCase.CreateActressUseCase(ctx, actressRepoHandler, actressReverseIndexUseCase, facePlusPlusUseCaseHandler)
	if err != nil {
		panic(err)
	}
	actressCrawlerUseCaseHandler := actressCrawlerUseCase.CreateActressCrawlerUseCase(ctx, logger, actressCrawlerMinnanoRepoHandler, actressRepoHandler, facePlusPlusUseCaseHandler, actressReverseIndexUseCase, 30, enableAddFace, enableBindFace, crawlerStartPage)
	authLineUseCaseHandler := authLineUseCase.CreateAuthLineUseCase(accountRepo, lineLoginAPIRepo, lineUserRepo, authRepo)
	authUseCase, err := auth.CreateAuthUseCase(authRepo, accountRepo, logger)
	if err != nil {
		panic(err)
	}

	authMiddleware := httpMiddlewareKit.CreateAuthMiddleware(func(ctx context.Context, token string) (userID int64, err error) {
		if env == "development" {
			return -1, nil
		}
		return authUseCase.Verify(token)
	})
	userRateLimitMiddleware := httpMiddlewareKit.CreateNoOpRateLimitMiddleware()
	if enableUserRateLimit {
		userRateLimitMiddleware = httpMiddlewareKit.CreateRateLimitMiddlewareWithSpecKey(false, true, true, redisRateLimitKit.CreateCacheRateLimit(redisCache, 500, 60).Pass)
	}
	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}
	r := mux.NewRouter()
	api := r.PathPrefix("/api/").Subrouter()
	api.Methods("GET").Path("/health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	api.Methods("GET").Path("/actress/{actressID}").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(actressDelivery.MakeGetActressEndpoint(actressUseCaseHandler)),
			actressDelivery.DecodeGetActressRequest,
			actressDelivery.EncodeGetActressResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/user/favorites").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(actressDelivery.MakeGetFavoritesEndpoint(actressUseCaseHandler))),
			actressDelivery.DecodeGetFavoritesRequest,
			actressDelivery.EncodeGetFavoritesResponse,
			options...,
		),
	)
	api.Methods("GET").Path("/user/searchActressByName/{actressName}").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(actressDelivery.MakeSearchActressesByNameEndpoint(actressUseCaseHandler))),
			actressDelivery.DecodeSearchActressByNameRequests,
			actressDelivery.EncodeSearchActressByNameResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/user/favorites").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(actressDelivery.MakeAddFavoriteEndpoint(actressUseCaseHandler))),
			actressDelivery.DecodeAddFavoriteRequest,
			actressDelivery.EncodeAddFavoritesResponse,
			options...,
		),
	)
	api.Methods("DELETE").Path("/user/favorites/{actressID}").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(actressDelivery.MakeRemoveFavoriteEndpoint(actressUseCaseHandler))),
			actressDelivery.DecodeRemoveFavoriteRequest,
			actressDelivery.EncodeRemoveFavoritesResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/user/searchActress").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(authMiddleware(actressDelivery.MakeUploadSearchImageEndpoint(actressUseCaseHandler))),
			actressDelivery.DecodeUploadSearchImageRequest,
			actressDelivery.EncodeUploadSearchImageResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/verifyLIFFToken").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(lineDelivery.MakeVerifyLIFFEndpoint(authLineUseCaseHandler)),
			lineDelivery.DecodeVerifyLIFFRequest,
			lineDelivery.EncodeVerifyLIFFResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/verifyLineCode").Handler(
		httptransport.NewServer(
			userRateLimitMiddleware(lineDelivery.MakeVerifyLineCodeEndpoint(authLineUseCaseHandler)),
			lineDelivery.DecodeVerifyLineCodeRequest,
			lineDelivery.EncodeVerifyLineCodeResponse,
			options...,
		),
	)
	api.Methods("POST").Path("/lineWebhook").Handler(
		httptransport.NewServer(
			actressDelivery.MakeLineWebhookEndpoint(actressLineUseCaseHandler, logger),
			actressDelivery.DecodeLineWebhookRequest,
			actressDelivery.EncodeLineWebhookResponse,
			options...,
		),
	)

	r.PathPrefix("/search").Handler(http.StripPrefix("/search", http.FileServer(http.Dir("./web/"))))
	r.PathPrefix("/favorite").Handler(http.StripPrefix("/favorite", http.FileServer(http.Dir("./web/"))))
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./web/"))))

	c := cors.New(cors.Options{
		AllowedMethods: []string{"HEAD", "GET", "POST", "DELETE"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	})

	httpSrv := http.Server{
		Addr:    ":9090",
		Handler: c.Handler(r),
	}

	if dbBackupBucket != "" {
		go func() {
			if err := awsDelivery.CreateAsyncDBBackup(ctx, s3UseCase, dbBackupScript, dbBackupFilePath, dbBackupKey, time.Duration(dbBackupDuration)*time.Second); err != nil {
				logger.Fatal(fmt.Sprintf("backup db get error, error: %+v", err))
			}
		}()
	}
	go func() {
		if err := actressBackgroundDelivery.CreateAsyncActressCrawler(ctx, actressCrawlerUseCaseHandler); err != nil {
			logger.Fatal(fmt.Sprintf("crawler get error, error: %+v", err))
		}
	}()
	go func() {
		if enableHTTPS {
			if err := httpSrv.ListenAndServeTLS(httpsCertFilePath, httpsPrivateKeyFilePath); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("https server get error, error: %+v", err))
			}
		} else {
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("http server get error, error: %+v", err))
			}
		}
	}()
	if enablePprofServer {
		go func() {
			if err := http.ListenAndServe(":9999", nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatal(fmt.Sprintf("pprof http server get error, error: %+v", err))
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	httpSrv.Shutdown(ctx)
	cancel()
}
