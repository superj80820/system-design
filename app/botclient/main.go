package main

import (
	"context"
	"flag"

	"github.com/superj80820/system-design/bot/delivery"
	"github.com/superj80820/system-design/bot/repository/eventsourcebotservice"

	readEventUseCase "github.com/superj80820/system-design/bot/usecase/readevent"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func main() {
	websocketURL := utilKit.GetEnvString("WEBSOCKET_URL", "ws://0.0.0.0:8080/ws")
	eventKey := utilKit.GetRequireEnvString("EVENT_KEY")

	flag.Parse()

	ctx := context.Background()

	logger, err := loggerKit.NewLogger("./go.log", loggerKit.InfoLevel, loggerKit.NoStdout)
	if err != nil {
		panic(err)
	}

	eventSourceRepo, err := eventsourcebotservice.CreateEventSource[*domain.TicketPlusEvent](websocketURL)
	if err != nil {
		panic(err)
	}

	readEventUseCaseInstance := readEventUseCase.CreateReadEvent(eventSourceRepo)

	waitKeyboard, err := delivery.DrawTerminal(ctx, "terminal", eventKey, readEventUseCaseInstance, logger)
	if err != nil {
		panic(err)
	}

	waitKeyboard()
}
