package instrumentation

import (
	"context"
	wlog "github.com/newrelic/infrastructure-agent/pkg/log"
	"net/http"
	"time"
)

var slog = wlog.WithComponent("SelfInstrumentation")

var SelfInstrumentation AgentInstrumentation = noopInstrumentation{}

type MetricType int64

const (
	Gauge MetricType = iota
	Sum
	Histrogram
)

type metric struct {
	Name       string
	Type       MetricType
	Value      float64
	Timestamp  time.Time
	Attributes map[string]interface{}
}

func NewGauge(name string, val float64) metric {
	return NewGaugeWithAttributes(name, val, nil)
}

func NewGaugeWithAttributes(name string, val float64, attrs map[string]interface{}) metric {
	return metric{Name: name, Type: Gauge, Value: val, Timestamp: time.Now(), Attributes: attrs}
}

// AgentInstrumentation does it make sense to abstract it?
type AgentInstrumentation interface {
	StartTransaction(ctx context.Context, name string) (context.Context, Transaction)
	RecordMetric(ctx context.Context, metric metric)
}

type Transaction interface {
	StartSegment(ctx context.Context, name string) (context.Context, Segment)
	StartExternalSegment(ctx context.Context, name string, req *http.Request) (context.Context, Segment)
	AddAttribute(key string, value interface{})
	End()
	NoticeError(err error)
}

type Segment interface {
	AddAttribute(key string, value interface{})
	End()
}

type noopInstrumentation struct {
}

func (n noopInstrumentation) StartTransaction(ctx context.Context, name string) (context.Context, Transaction) {
	return ctx, NoopTransaction{ctx: ctx}
}

func (n noopInstrumentation) RecordMetric(ctx context.Context, metric metric) {
	//intentionally left empty
}

type NoopTransaction struct {
	ctx context.Context
}

func (n NoopTransaction) StartExternalSegment(ctx context.Context, name string, req *http.Request) (context.Context, Segment) {
	return ctx, NoopSegment{}
}

func (n NoopTransaction) StartSegment(ctx context.Context, _ string) (context.Context, Segment) {
	return ctx, NoopSegment{}
}

func (n NoopTransaction) End() {
	//intentionally left empty
}

func (n NoopTransaction) AddAttribute(key string, value interface{}) {
	//intentionally left empty
}

func (n NoopTransaction) NoticeError(_ error) {
	//intentionally left empty
}

type NoopSegment struct {
}

func (n NoopSegment) AddAttribute(key string, value interface{}) {
	//intentionally left empty
}

func (n NoopSegment) End() {
	//intentionally left empty
}

func TransactionFromContext(ctx context.Context) Transaction {
	if ctx == nil {
		return NoopTransaction{}
	}
	if txn, ok := ctx.Value(transactionInContextKey).(Transaction); ok {
		return txn
	}
	return NoopTransaction{}
}

func ContextWithTransaction(ctx context.Context, txn Transaction) context.Context {
	return context.WithValue(ctx, transactionInContextKey, txn)
}
