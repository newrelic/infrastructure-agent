package rate

import (
	"encoding/json"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"sync"
	"time"
)

const (
	defaultMapSize = 1024
)

type Calculator interface {
	// GetRate creates a gauge metric.
	// If this is the first time the name/attributes combination has been seen then the `valid` return value will be false.
	GetRate(name string, attributes map[string]interface{}, val float64, now time.Time) (gauge telemetry.Gauge, valid bool)

	// GetCumulativeRate creates a Gauge metric with rate of change based on the previous timestamp and value.
	// If no previous timestamp is NOT found, returns false (as no calculation is made)
	// If a previous timestamp is found use it to get the elapsed time (in seconds) and use that as the denominator
	// Rate = value / (now - before)[s]
	GetCumulativeRate(name string, attributes map[string]interface{}, val float64, now time.Time) (gauge telemetry.Gauge, valid bool)

	// Clean removes any data points that need to be cleaned up
	Clean()
}

func NewCalculator() Calculator {
	return &calculator{
		datapoints:              make(map[metricIdentity]lastValue, defaultMapSize),
		expirationCheckInterval: 20 * time.Minute,
		expirationAge:           20 * time.Minute,
	}
}

type metricIdentity struct {
	Name           string
	AttributesJSON string
}

type lastValue struct {
	When  time.Time
	Value float64
}

type calculator struct {
	lock                    sync.Mutex
	datapoints              map[metricIdentity]lastValue
	lastClean               time.Time
	expirationCheckInterval time.Duration
	expirationAge           time.Duration
}

// GetRate creates a gauge metric.
// If this is the first time the name/attributes combination has been seen then the `valid` return value will be false.
func (c *calculator) GetRate(name string, attributes map[string]interface{}, val float64, now time.Time) (gauge telemetry.Gauge, valid bool) {
	var attributesJSON []byte
	var err error
	if nil != attributes {
		attributesJSON, err = json.Marshal(attributes)
		if err != nil {
			return
		}
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	id := metricIdentity{Name: name, AttributesJSON: string(attributesJSON)}

	last, found := c.datapoints[id]
	if found {
		// don't accept timestamps older that the last one for this metric
		if last.When.Before(now) {
			elapsedSeconds := now.Sub(last.When).Seconds()
			rate := val / elapsedSeconds

			gauge.Name = name
			gauge.Timestamp = now
			gauge.Value = rate
			gauge.Attributes = attributes
			gauge.AttributesJSON = attributesJSON

			valid = true
		}
	} else {
		c.datapoints[id] = lastValue{When: now}
	}

	return
}

func (c *calculator) GetCumulativeRate(name string, attributes map[string]interface{}, val float64, now time.Time) (gauge telemetry.Gauge, valid bool) {
	var attributesJSON []byte
	var err error
	if nil != attributes {
		attributesJSON, err = json.Marshal(attributes)
		if err != nil {
			return
		}
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	id := metricIdentity{Name: name, AttributesJSON: string(attributesJSON)}

	last, found := c.datapoints[id]
	if found {
		// don't accept timestamps older that the last one for this metric
		if last.When.Before(now) {
			elapsedSeconds := now.Sub(last.When).Seconds()
			diff := val - last.Value
			// only positive deltas accepted
			if diff >= 0 {
				rate := diff / elapsedSeconds

				gauge.Name = name
				gauge.Timestamp = now
				gauge.Value = rate
				gauge.Attributes = attributes
				gauge.AttributesJSON = attributesJSON

				valid = true
			}
		}
	} else {
		c.datapoints[id] = lastValue{When: now, Value: val}
	}

	return
}

func (c *calculator) Clean() {
	c.lock.Lock()
	defer c.lock.Unlock()
	now := time.Now()
	if now.Sub(c.lastClean) > c.expirationCheckInterval {
		cutoff := now.Add(-c.expirationAge)
		for k, v := range c.datapoints {
			if v.When.Before(cutoff) {
				delete(c.datapoints, k)
			}
		}
		c.lastClean = now
	}
}
