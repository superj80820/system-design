package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/pkg/errors"

	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
	utilKit "github.com/superj80820/system-design/kit/util"
)

func Reserve(
	ctx context.Context,
	ticketPlusService domain.TicketPlusService,
	logger loggerKit.Logger,
	countryCode,
	mobile,
	password,
	eventID string,
	priority map[string]int,
	reserveDuration,
	captchaDuration time.Duration,
	captchaCount int,
	ReserveExecTime int64,
	ReserveGetErrorThenContinue bool,
) {
	_, err := ticketPlusService.Reserve(ctx, &domain.TicketPlusReserveSchedule{
		CountryCode:                 countryCode,
		Mobile:                      mobile,
		Password:                    password,
		EventID:                     eventID,
		Priority:                    priority,
		CaptchaDuration:             captchaDuration,
		CaptchaCount:                captchaCount,
		ReserveExecTime:             ReserveExecTime,
		ReserveGetErrorThenContinue: ReserveGetErrorThenContinue,
	})
	if err != nil {
		logger.Info(fmt.Sprintf("reserve error: %+v", err))
	}
}

func DrawTerminal(ctx context.Context, key, eventKey string, readEventService domain.ReadEventService, logger loggerKit.Logger) (func(), error) {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}

	getTicketPlusStatus, err := readEventService.GetTicketPlusStatus(key, eventKey)
	if err != nil {
		return nil, errors.Wrap(err, "get ticket plus status failed")
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for { // TODO: return
			<-ticker.C

			p := widgets.NewParagraph()
			p.Text = time.Now().String()
			p.SetRect(5, 0, 25, 5)

			ui.Render(p)
		}
	}()

	go func() { // TODO: async problem
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			<-ticker.C

			ticketPlusStatus := getTicketPlusStatus()
			ticketPlusStatusLog, _ := json.Marshal(ticketPlusStatus)
			logger.Info("ticket plus " + string(ticketPlusStatusLog))

			data := make([]float64, len(ticketPlusStatus)*3)
			labels := make([]string, len(ticketPlusStatus)*3)

			for idx := range labels {
				division := idx % 3
				ticketPlusStatusIdx := idx / 3
				if division == 0 {
					labels[idx] = strconv.Itoa(ticketPlusStatus[ticketPlusStatusIdx].SortedIndex)
					if ticketPlusStatus[ticketPlusStatusIdx].Done {
						data[idx] = 1
					}
				} else if division == 1 {
					labels[idx] = "_"
					if len(ticketPlusStatus[ticketPlusStatusIdx].Count) > int(domain.TicketPlusResponseStatusPending) {
						data[idx] = float64(ticketPlusStatus[ticketPlusStatusIdx].Count[domain.TicketPlusResponseStatusPending])
					}
				} else {
					var otherCount int
					for idx, val := range ticketPlusStatus[ticketPlusStatusIdx].Count {
						if idx == int(domain.TicketPlusResponseStatusPending) ||
							idx == int(domain.TicketPlusResponseStatusOK) {
							continue
						}
						otherCount += val
					}
					labels[idx] = "_"
					data[idx] = float64(otherCount)
				}
			}

			bc := widgets.NewBarChart()
			bc.Data = data
			bc.Labels = labels
			bc.Title = "Bar Chart"
			bc.SetRect(5, 5, 200, 25)
			bc.BarWidth = 1
			bc.BarColors = []ui.Color{ui.ColorMagenta, ui.ColorGreen, ui.ColorRed}
			bc.LabelStyles = []ui.Style{ui.NewStyle(ui.ColorBlue)}
			bc.NumStyles = []ui.Style{ui.NewStyle(ui.ColorYellow)}

			ui.Render(bc)

			for idx, ticketPlusStatusChunk := range utilKit.ChunkBy[*domain.ReadEventStatus](ticketPlusStatus, 20) {
				p := widgets.NewParagraph()
				statusTextSlice := make([]string, 0, 20)
				for _, val := range ticketPlusStatusChunk {
					var valCountSlice []string
					for idx, valCount := range val.Count {
						if valCount == 0 {
							continue
						}
						valCountSlice = append(valCountSlice, domain.TicketPlusResponseStatus(idx).String()+": "+strconv.Itoa(valCount))
					}
					statusTextSlice = append(statusTextSlice, val.Name+"("+strconv.Itoa(val.SortedIndex)+")"+": "+
						strings.Join(valCountSlice, " || "))
				}
				p.Text = strings.Join(statusTextSlice, "\n")
				p.SetRect(5+idx*60, 25, 65+idx*60, 50)

				ui.Render(p)
			}

		}
	}()

	return func() {
		defer ui.Close()

		uiEvents := ui.PollEvents()
		for {
			e := <-uiEvents
			switch e.ID {
			case "q", "<C-c>":
				return
			}
		}
	}, nil
}
