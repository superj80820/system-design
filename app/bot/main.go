package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	ticketPlusHTTPDelivery "github.com/superj80820/system-design/bot/delivery/http"
	wsDelivery "github.com/superj80820/system-design/bot/delivery/http/websocket"
	"github.com/superj80820/system-design/bot/repository/eventsource"
	"github.com/superj80820/system-design/bot/repository/ocr"
	"github.com/superj80820/system-design/bot/repository/ticketplus"
	eventSourceUseCase "github.com/superj80820/system-design/bot/usecase/eventsource"
	ticketPlusUseCase "github.com/superj80820/system-design/bot/usecase/ticketplus"
	"github.com/superj80820/system-design/domain"
	wsTransport "github.com/superj80820/system-design/kit/core/transport/http/websocket"
	httpKit "github.com/superj80820/system-design/kit/http"
	wsKit "github.com/superj80820/system-design/kit/http/websocket"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	ormKit "github.com/superj80820/system-design/kit/orm"
	traceKit "github.com/superj80820/system-design/kit/trace"
	utilKit "github.com/superj80820/system-design/kit/util"
	lineRepo "github.com/superj80820/system-design/line/repository"
)

func main() {
	ticketPlusURL := utilKit.GetEnvString("TICKET_PLUS_URL", "https://apis.ticketplus.com.tw")
	ticketPlusOrderListURL := utilKit.GetEnvString("TICKET_PLUS_ORDER_LIST_URL", "https://ticketplus.com.tw/orderList")
	priorityString := utilKit.GetEnvString("PRIORITY", "{}")
	addr := utilKit.GetEnvString("ADDR", "0.0.0.0:8080")
	serviceName := utilKit.GetEnvString("BOT_SERVICE", "bot-service")
	tokenBucketDuration := utilKit.GetEnvInt("TOKEN_BUCKET_DURATION", 1)
	tokenBucketCount := utilKit.GetEnvInt("TOKEN_BUCKET_COUNT", 3)
	ocrServiceURL := utilKit.GetEnvString("OCR_SERVICE_URL", "http://ocr-service:5001")
	lineAPIURL := utilKit.GetEnvString("LINE_API_URL", "https://notify-api.line.me")
	lineAPIToken := utilKit.GetRequireEnvString("LINE_API_TOKEN")
	lineMonitorAPIToken := utilKit.GetRequireEnvString("LINE_MONITOR_API_TOKEN")
	tokenExpireDuration := utilKit.GetEnvInt("TOKEN_EXPIRE_DURATION", 3600)
	reserveSchedulesString := utilKit.GetEnvString("RESERVE_SCHEDULES", "[]")
	var reserveSchedules []*domain.TicketPlusReserveSchedule
	if err := json.Unmarshal([]byte(reserveSchedulesString), &reserveSchedules); err != nil {
		panic(err)
	}

	flag.Parse()

	priority := make(map[string]int)
	err := json.Unmarshal([]byte(priorityString), &priority)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	tracer := traceKit.CreateNoOpTracer()
	ormDB, err := ormKit.CreateDB(ormKit.UseSQLite("sqlite.db"))
	if err != nil {
		panic(err)
	}
	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel)
	if err != nil {
		panic(err)
	}

	ocrRepo := ocr.CreateOCR(ocrServiceURL)

	ticketPlusRepo, err := ticketplus.CreateTicketPlus(ticketPlusURL, ormDB)
	if err != nil {
		panic(err)
	}
	eventSourceRepo := eventsource.CreateEventSource[*domain.TicketPlusEvent]()

	readEventUseCaseInstance := eventSourceUseCase.CreateReadEvent(eventSourceRepo)
	ticketPlusUseCaseInstance, err := ticketPlusUseCase.CreateTicketPlus(
		ctx,
		ticketPlusOrderListURL,
		ticketPlusRepo,
		ocrRepo,
		eventSourceRepo,
		lineRepo.CreateLineRepo(lineAPIURL, lineAPIToken),
		lineRepo.CreateLineRepo(lineAPIURL, lineMonitorAPIToken),
		logger,
		time.Duration(tokenExpireDuration)*time.Second,
		reserveSchedules,
		time.Duration(tokenBucketDuration)*time.Second,
		tokenBucketCount,
	)
	if err != nil {
		panic(err)
	}

	r := mux.NewRouter()
	options := []httptransport.ServerOption{
		httptransport.ServerBefore(httpKit.CustomBeforeCtx(tracer)),
		httptransport.ServerAfter(httpKit.CustomAfterCtx),
		httptransport.ServerErrorEncoder(httpKit.EncodeHTTPErrorResponse()),
	}
	r.Handle("/ws",
		wsTransport.NewServer(
			wsDelivery.MakeReadEventEndpoint(serviceName, readEventUseCaseInstance),
			wsKit.NoRequest,
			wsKit.JsonEncodeResponse[*domain.TicketPlusEvent],
			wsTransport.AddHTTPResponseHeader(wsKit.CustomHeaderFromCtx(ctx)),
			wsTransport.ServerErrorEncoder(wsKit.EncodeWSErrorResponse()),
		))

	r.Methods("GET").Path("/api/v1/reserve/schedule").Handler(
		httptransport.NewServer(
			ticketPlusHTTPDelivery.MakeGetReservesScheduleEndpoint(ticketPlusUseCaseInstance),
			ticketPlusHTTPDelivery.DecodeGetReservesScheduleRequest,
			ticketPlusHTTPDelivery.EncodeGetReservesScheduleResponse,
			options...,
		))
	r.Methods("DELETE").Path("/api/v1/reserve/schedule").Handler(
		httptransport.NewServer(
			ticketPlusHTTPDelivery.MakeDeleteReservesScheduleEndpoint(ticketPlusUseCaseInstance),
			ticketPlusHTTPDelivery.DecodeDeleteReservesScheduleRequest,
			ticketPlusHTTPDelivery.EncodeDeleteReservesScheduleResponse,
			options...,
		))
	r.Methods("POST").Path("/api/v1/reserve/schedule").Handler(
		httptransport.NewServer(
			ticketPlusHTTPDelivery.MakeReservesScheduleEndpoint(ticketPlusUseCaseInstance),
			ticketPlusHTTPDelivery.DecodeReservesScheduleRequest,
			ticketPlusHTTPDelivery.EncodeReservesScheduleResponse,
			options...,
		))
	r.Methods("PUT").Path("/api/v1/reserve/frequency").Handler(
		httptransport.NewServer(
			ticketPlusHTTPDelivery.MakeUpdateReserveFrequencyEndpoint(ticketPlusUseCaseInstance),
			ticketPlusHTTPDelivery.DecodeUpdateReserveFrequencyRequest,
			ticketPlusHTTPDelivery.EncodeUpdateReserveFrequencyResponse,
			options...,
		),
	)

	srv := http.Server{
		Addr:    addr,
		Handler: r,
	}
	go func() {
		logger.Info("start server")
		if err := srv.ListenAndServe(); err != nil {
			logger.Error(fmt.Sprintf("close service failed, error: %+v", err))
		}
	}()

	<-quit
	srv.Shutdown(ctx)
}
