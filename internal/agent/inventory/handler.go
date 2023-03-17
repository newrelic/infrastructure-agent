package inventory

import (
	context2 "context"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"net/http"
	"time"
)

var (
	ilog = log.WithComponent("Inventory")
)

type HandlerConfig struct {
	ReapInterval time.Duration
	SendInterval time.Duration
}

// Handler maintains the infrastructure inventory in an updated state.
// It will receive inventory data from integrations/plugins for processing deltas (differences between versions)
type Handler struct {
	cfg HandlerConfig

	ctx      context2.Context
	cancelFn context2.CancelFunc

	patcher Patcher

	dataCh chan types.PluginOutput

	reapTimer *time.Ticker
	sendTimer *time.Ticker

	sendErrorCount uint32
}

// NewInventoryHandler returns a new instances of an inventory.Handler.
func NewInventoryHandler(cfg HandlerConfig, patcher Patcher) *Handler {
	ctx, cancelFn := context2.WithCancel(context2.Background())

	return &Handler{
		cfg:       cfg,
		reapTimer: time.NewTicker(cfg.ReapInterval),
		sendTimer: time.NewTicker(cfg.SendInterval),
		dataCh:    make(chan types.PluginOutput, 0),
		ctx:       ctx,
		cancelFn:  cancelFn,
		patcher:   patcher,
	}
}

// Handle the inventory data from a plugin/integration.
func (h *Handler) Handle(data types.PluginOutput) {
	h.dataCh <- data
}

// Start will run the routines that periodically checks for deltas and submit them.
func (h *Handler) Start() {
	go h.listenForData()
	h.doProcess()
}

// Stop will gracefully stop the inventory.Handler.
func (h *Handler) Stop() {
	h.cancelFn()
}

// listenForData from plugins/integrations.
func (h *Handler) listenForData() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case data := <-h.dataCh:
			err := h.patcher.Save(data)
			if err != nil {
				ilog.WithError(err).Error("problem storing plugin output")
			}
		}
	}
}

// doProcess does the inventory processing.
func (h *Handler) doProcess() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.reapTimer.C:
			h.patcher.Reap()
		case <-h.sendTimer.C:
			h.send()
		}
	}
}

// send will submit the deltas.
func (h *Handler) send() {
	backoffMax := config.MAX_BACKOFF

	err := h.patcher.Send()
	if err != nil {
		if ingestError, ok := err.(*inventoryapi.IngestError); ok &&
			ingestError.StatusCode == http.StatusTooManyRequests {

			ilog.Warn("server is rate limiting inventory submission")

			backoffMax = config.RATE_LIMITED_BACKOFF
			h.sendErrorCount = helpers.MaxBackoffErrorCount
		} else {
			h.sendErrorCount++
		}

		ilog.WithError(err).WithField("errorCount", h.sendErrorCount).
			Debug("Inventory sender can't process data.")
	} else {
		h.sendErrorCount = 0
	}

	sendTimerVal := helpers.ExpBackoff(h.cfg.SendInterval,
		time.Duration(backoffMax)*time.Second,
		h.sendErrorCount)
	h.sendTimer.Reset(sendTimerVal)
}
