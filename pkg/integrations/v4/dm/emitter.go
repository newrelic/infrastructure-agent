// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/backend/backoff"
	"time"

	"github.com/tevino/abool"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/entity/register"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

var (
	// Errors
	ProtocolV4NotEnabledErr = errors.New("integration protocol version 4 is not enabled")
	NoContentToParseErr     = errors.New("no content to parse")

	// internal
	elog = log.WithComponent("DimensionalMetricsEmitter")
)

const (
	defaultRegisterWorkersAmnt        = 4
	defaultRegisterBatchSize          = 100
	defaultRegisterBatchBytesSize     = 1000 * 1000 // Size limit for a batch call payload (1MB)
	defaultRegisterBatchSecs          = 1
	defaultRequestsQueueLen           = 1000
	defaultRequestsToRegisterQueueLen = 1000
	defaultRequestsRegisteredQueueLen = 1000
)

type Agent interface {
	GetContext() agent.AgentContext
}

type emitter struct {
	isProcessing              abool.AtomicBool
	reqsQueue                 chan fwrequest.FwRequest
	reqsToRegisterQueue       chan fwrequest.EntityFwRequest
	reqsRegisteredQueue       chan fwrequest.EntityFwRequest
	retryBo                   *backoff.Backoff
	maxRetryBo                time.Duration
	idCache                   entity.KnownIDs
	metricsSender             MetricsSender
	agentContext              agent.AgentContext
	registerClient            identityapi.RegisterClient
	registerWorkers           int
	registerMaxBatchSize      int
	registerMaxBatchBytesSize int
	registerMaxBatchTime      time.Duration
	verboseLogLevel           int
}

type Emitter interface {
	Send(fwrequest.FwRequest)
}

func NewEmitter(
	agentContext agent.AgentContext,
	dmSender MetricsSender,
	registerClient identityapi.RegisterClient) Emitter {

	return &emitter{
		retryBo:                   backoff.NewDefaultBackoff(),
		maxRetryBo:                time.Duration(agentContext.Config().RegisterMaxRetryBoSecs) * time.Second,
		reqsQueue:                 make(chan fwrequest.FwRequest, defaultRequestsQueueLen),
		reqsToRegisterQueue:       make(chan fwrequest.EntityFwRequest, defaultRequestsToRegisterQueueLen),
		reqsRegisteredQueue:       make(chan fwrequest.EntityFwRequest, defaultRequestsRegisteredQueueLen),
		registerWorkers:           defaultRegisterWorkersAmnt,
		idCache:                   entity.NewKnownIDs(),
		agentContext:              agentContext,
		metricsSender:             dmSender,
		registerClient:            registerClient,
		registerMaxBatchSize:      defaultRegisterBatchSize,
		registerMaxBatchBytesSize: defaultRegisterBatchBytesSize,
		registerMaxBatchTime:      defaultRegisterBatchSecs * time.Second,
		verboseLogLevel:           agentContext.Config().Verbose,
	}
}

// Send receives data forward requests and queues them while processing them on different goroutine.
// Processor is automatically being lazy run at first data received.
func (e *emitter) Send(req fwrequest.FwRequest) {
	e.reqsQueue <- req
	e.lazyLoadProcessor()
}

func (e *emitter) lazyLoadProcessor() {
	if e.isProcessing.IsNotSet() {
		e.isProcessing.Set()
		ctx := e.agentContext.Context()

		go e.runFwReqConsumer(ctx)
		go e.runReqsRegisteredConsumer(ctx)
		for w := 0; w < e.registerWorkers; w++ {
			config := register.WorkerConfig{
				MaxBatchSize:      e.registerMaxBatchSize,
				MaxBatchSizeBytes: e.registerMaxBatchBytesSize,
				MaxBatchDuration:  e.registerMaxBatchTime,
				MaxRetryBo:        e.maxRetryBo,
				VerboseLogLevel:   e.verboseLogLevel,
			}
			regWorker := register.NewWorker(
				e.agentContext.Identity,
				e.registerClient,
				e.retryBo,
				e.reqsToRegisterQueue,
				e.reqsRegisteredQueue,
				config)
			go regWorker.Run(ctx)
		}
	}
}

// runFwReqConsumer consumes forward reqs and dispatches them to registered or non-registered queues
// based on local entity Key to ID cache.
func (e *emitter) runFwReqConsumer(ctx context.Context) {
	defer e.isProcessing.UnSet()

	agentVersion := e.agentContext.Version()
	for {
		select {
		case _ = <-ctx.Done():
			return

		case req := <-e.reqsQueue:
			for _, ds := range req.Data.DataSets {

				eKey, err := ds.Entity.ResolveUniqueEntityKey(e.agentContext.EntityKey(), e.agentContext.IDLookup(), req.FwRequestMeta.EntityRewrite, 4)
				if err != nil {
					elog.
						WithError(err).
						WithField("integration", req.Definition.Name).
						Errorf("couldn't determine a unique entity Key")
					continue
				}

				var eID entity.ID
				var found bool

				if ds.Entity.IsAgent() {
					eID, found = e.agentContext.Identity().ID, true
				} else {
					eID, found = e.idCache.Get(eKey)
				}

				if found {
					select {
					case <-ctx.Done():
						return

					case e.reqsRegisteredQueue <- fwrequest.NewEntityFwRequest(ds, eID, req.FwRequestMeta, req.Data.Integration, agentVersion):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return

				case e.reqsToRegisterQueue <- fwrequest.NewEntityFwRequest(ds, entity.EmptyID, req.FwRequestMeta, req.Data.Integration, agentVersion):
				}
			}
		}
	}
}

func (e *emitter) runReqsRegisteredConsumer(ctx context.Context) {
	for {
		select {
		case _ = <-ctx.Done():
			return

		case eReq := <-e.reqsRegisteredQueue:
			e.processEntityFwRequest(eReq)
		}
	}
}

func (e *emitter) processEntityFwRequest(r fwrequest.EntityFwRequest) {
	// rewrites processing
	agentShortName, err := e.agentContext.IDLookup().AgentShortEntityName()
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", r.Definition.Name).
			Errorf("cannot determine agent short name")
	}
	replaceEntityName(r.Data.Entity, r.EntityRewrite, agentShortName)

	key, err := r.Data.Entity.Key()
	if err != nil {
		elog.
			WithError(err).
			WithField("integration", r.Definition.Name).
			Errorf("cannot determine entity")
	} else {
		e.idCache.CleanOld()
		e.idCache.Put(key, r.ID())
	}

	labels, annos := r.LabelsAndExtraAnnotations()

	plugin := agent.NewExternalPluginCommon(r.Definition.PluginID(r.Integration.Name), e.agentContext, r.Definition.Name)

	emitInventory(&plugin, r.Definition, r.Integration, r.ID(), r.Data, labels)

	emitEvent(&plugin, r.Definition, r.Data, labels, r.ID())

	dmProcessor := IntegrationProcessor{
		IntegrationInterval:         r.Definition.Interval,
		IntegrationLabels:           labels,
		IntegrationExtraAnnotations: annos,
	}
	metrics := dmProcessor.ProcessMetrics(r.Data.Metrics, r.Data.Common, r.Data.Entity)
	if err := e.metricsSender.SendMetricsWithCommonAttributes(r.Data.Common, metrics); err != nil {
		elog.WithField("entity", r.ID()).WithError(err).Warn("discarding metrics")
	}
}

func emitInventory(
	emitter agent.PluginEmitter,
	metadata integration.Definition,
	integrationMetadata protocol.IntegrationMetadata,
	entityID entity.ID,
	dataSet protocol.Dataset,
	labels map[string]string) {
	logEntry := elog.WithField("action", "EmitV4DataSet")

	integrationUser := metadata.ExecutorConfig.User

	if len(dataSet.Inventory) > 0 {
		inventoryDataSet := legacy.BuildInventoryDataSet(
			logEntry, dataSet.Inventory, labels, integrationUser, integrationMetadata.Name,
			dataSet.Entity.Name)
		entityKey := entity.Key(dataSet.Entity.Name)
		emitter.EmitInventory(inventoryDataSet, entity.New(entityKey, entityID))
	}
}

func emitEvent(emitter agent.PluginEmitter, metadata integration.Definition, dataSet protocol.Dataset, labels map[string]string, entityID entity.ID) {
	sharedOpts := []func(protocol.EventData){
		protocol.WithEntity(entity.New(entity.Key(dataSet.Entity.Name), entityID)),
		protocol.WithLabels(labels),
	}

	u := metadata.ExecutorConfig.User
	if u != "" {
		sharedOpts = append(sharedOpts, protocol.WithIntegrationUser(u))
	}

	for _, event := range dataSet.Events {
		opts := append(sharedOpts, protocol.WithEvents(event))

		attributesFromEvent(event, &opts)

		e, err := protocol.NewEventData(opts...)
		if err != nil {
			elog.WithFields(logrus.Fields{
				"payload": event,
				"error":   err,
			}).Warn("discarding event, failed building event data.")
			continue
		}

		emitter.EmitEvent(e, entity.Key(dataSet.Entity.Name))
	}
}

func attributesFromEvent(event protocol.EventData, builder *[]func(protocol.EventData)) {
	if a, ok := event["attributes"]; ok {
		switch t := a.(type) {
		default:
		case map[string]interface{}:
			*builder = append(*builder, protocol.WithAttributes(t))
		}
	}
}

// Replace entity name by applying entity rewrites and replacing loopback
func replaceEntityName(entity entity.Fields, entityRewrite data.EntityRewrites, agentShortName string) {
	newName := entityRewrite.Apply(entity.Name)
	newName = http.ReplaceLocalhost(newName, agentShortName)
	entity.Name = newName
}

// ParsePayloadV4 parses a string containing a JSON payload with the format of our
// SDK for v4 protocol which uses dimensional metrics.
func ParsePayloadV4(raw []byte, ffManager feature_flags.Retriever) (dataV4 protocol.DataV4, err error) {
	if len(raw) == 0 {
		err = NoContentToParseErr
		return
	}

	if enabled, ok := ffManager.GetFeatureFlag(fflag.FlagProtocolV4); !ok || !enabled {
		err = ProtocolV4NotEnabledErr
		return
	}

	err = json.Unmarshal(raw, &dataV4)
	return
}

// Returns a composed error which describes all the errors found during the emit process of each data set
func composeEmitError(emitErrs []error, dataSetLength int) error {
	if len(emitErrs) == 0 {
		return nil
	}

	composedError := fmt.Sprintf("%d out of %d datasets could not be emitted. Reasons: ", len(emitErrs), dataSetLength)
	messages := map[string]struct{}{}

	for _, err := range emitErrs {
		msg := err.Error()
		if _, ok := messages[msg]; !ok { // avoid logging repeated error messages
			messages[msg] = struct{}{}
			composedError += msg + ","
		}
	}
	return errors.New(composedError[:len(composedError)-1])
}
