package rate

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCalculator_GetRate_BasicUsage(t *testing.T) {
	now := time.Now()
	cal := NewCalculator()
	attributes := map[string]interface{}{
		"string": "value",
		"int":    123,
		"int64":  int64(456),
		"float":  789.011,
	}
	val := 10.0
	_, valid := cal.GetRate("name", attributes, val, now)
	// no previous timestamp
	assert.False(t, valid)

	gauge, valid := cal.GetRate("name", attributes, val*2, now.Add(time.Second))
	assert.True(t, valid)
	assert.Equal(t, "name", gauge.Name)
	assert.Equal(t, 20.0, gauge.Value)

	gauge, valid = cal.GetRate("name", attributes, val, now.Add(time.Second*5))
	assert.True(t, valid)
	assert.Equal(t, "name", gauge.Name)
	assert.Equal(t, 2.0, gauge.Value)
}

func TestRateCalculator_OlderTimestampsNotAccepted(t *testing.T) {

	now := time.Now()
	cal := NewCalculator()

	attrs := map[string]interface{}{"abc": "123"}
	_, valid := cal.GetRate("errorsPerSecond", attrs, 10, now)
	// no previous timestamp
	assert.False(t, valid)

	g, valid := cal.GetRate("errorsPerSecond", attrs, 20, now.Add(1*time.Second))
	assert.True(t, valid)
	assert.Equal(t, 20.0, g.Value)

	_, valid = cal.GetRate("errorsPerSecond", attrs, 10, now.Add(-5*time.Second))
	assert.False(t, valid)

	g, valid = cal.GetRate("errorsPerSecond", attrs, 10, now.Add(2*time.Second))
	assert.True(t, valid)
	assert.Equal(t, 5.0, g.Value)
}

func TestCalculator_Clean(t *testing.T) {
	lastClean := time.Now().Add(-30 * time.Minute)
	now := lastClean.Add(-10 * time.Minute)
	cal := &calculator{
		datapoints:              make(map[metricIdentity]lastValue, defaultMapSize),
		expirationCheckInterval: 20 * time.Minute,
		expirationAge:           20 * time.Minute,
		lastClean:               lastClean,
	}
	attrs := map[string]interface{}{"abc": "123"}
	cal.GetRate("somethingPerSecond", attrs, 10, now)
	cal.GetRate("somethingPerSecond", attrs, 10, now.Add(time.Minute))
	cal.Clean()
	_, valid := cal.GetRate("somethingPerSecond", attrs, 10, time.Now().Add(-10*time.Minute))
	// should have been removed from map
	assert.False(t, valid)
	// And calling clean again should not remove it
	cal.Clean()
	_, valid = cal.GetRate("somethingPerSecond", attrs, 10, time.Now())
	assert.True(t, valid)
}

func TestCalculator_GetCumulativeRate(t *testing.T) {

	now := time.Now()
	cal := NewCalculator()

	attrs := map[string]interface{}{"abc": "123"}
	_, valid := cal.GetCumulativeRate("requestsPerSecond", attrs, 10, now)
	// no previous value
	assert.False(t, valid)

	g, valid := cal.GetCumulativeRate("requestsPerSecond", attrs, 20, now.Add(1*time.Second))
	assert.True(t, valid)
	assert.Equal(t, 10.0, g.Value)

	g, valid = cal.GetCumulativeRate("requestsPerSecond", attrs, 10, now.Add(2*time.Second))
	assert.True(t, valid)
	assert.Equal(t, 0.0, g.Value)

	g, valid = cal.GetCumulativeRate("requestsPerSecond", attrs, 20, now.Add(10*time.Second))
	assert.True(t, valid)
	assert.Equal(t, 1.0, g.Value)
}
