// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package dm

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	telemetry "github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

func Test_sender_SendMetrics(t *testing.T) {
	cannedDuration, _ := time.ParseDuration("1m7s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	type fields struct {
		harvester *mockHarvester
	}
	type args struct {
		metrics []protocol.Metric
	}
	tests := []struct {
		name            string
		fields          fields
		args            args
		expectedMetrics []telemetry.Metric
	}{
		{
			name: "gauge",
			fields: fields{
				harvester: &mockHarvester{},
			},
			args: args{
				metrics: []protocol.Metric{
					{
						Name:       "GaugeMetric",
						Type:       "gauge",
						Value:      json.RawMessage("1.45"),
						Timestamp:  &cannedDateUnix,
						Attributes: map[string]interface{}{"att_key": "att_value"},
					},
				},
			},
			expectedMetrics: []telemetry.Metric{
				telemetry.Gauge{
					Name:       "GaugeMetric",
					Value:      1.45,
					Attributes: map[string]interface{}{"att_key": "att_value"},
					Timestamp:  cannedDate,
				},
			},
		},
		{
			name: "count",
			fields: fields{
				harvester: &mockHarvester{},
			},
			args: args{
				metrics: []protocol.Metric{
					{
						Name:       "CountMetric",
						Type:       "count",
						Value:      json.RawMessage("1.45"),
						Timestamp:  &cannedDateUnix,
						Interval:   &cannedDurationInt,
						Attributes: map[string]interface{}{"att_key": "att_value"},
					},
				},
			},
			expectedMetrics: []telemetry.Metric{
				telemetry.Count{
					Name:       "CountMetric",
					Value:      1.45,
					Attributes: map[string]interface{}{"att_key": "att_value"},
					Timestamp:  cannedDate,
					Interval:   cannedDuration,
				},
			},
		},
		{
			name: "summary",
			fields: fields{
				harvester: &mockHarvester{},
			},
			args: args{
				metrics: []protocol.Metric{
					{
						Name:       "SummaryMetric",
						Type:       "summary",
						Attributes: map[string]interface{}{"att_key": "att_value"},
						Timestamp:  &cannedDateUnix,
						Interval:   &cannedDurationInt,
						Value:      json.RawMessage("{ \"count\": 1, \"sum\": 2, \"min\":3, \"max\":4 }"),
					},
				},
			},
			expectedMetrics: []telemetry.Metric{
				telemetry.Summary{
					Name:       "SummaryMetric",
					Attributes: map[string]interface{}{"att_key": "att_value"},
					Count:      float64(1),
					Sum:        float64(2),
					Min:        float64(3),
					Max:        float64(4),
					Timestamp:  cannedDate,
					Interval:   cannedDuration,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sender{
				harvester: tt.fields.harvester,
			}

			expectedAttributes := telemetry.Attributes{
				"one": 1,
				"two": "two",
			}

			tt.fields.harvester.On("RecordInfraMetrics", expectedAttributes, tt.expectedMetrics).Return(nil)
			err := s.SendMetricsWithCommonAttributes(protocol.Common{
				Timestamp: &cannedDateUnix,
				Interval:  &cannedDurationInt,
				Attributes: map[string]interface{}{
					"one": 1,
					"two": "two",
				},
			}, tt.args.metrics)
			require.NoError(t, err)
			tt.fields.harvester.AssertExpectations(t)
		})
	}
}

func Test_sender_SenderMetric_cumulative_CountCalculator(t *testing.T) {
	cannedDuration, _ := time.ParseDuration("1m7s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()

	name := "CumulativeCountMetric"
	val := 1.45
	attributes := map[string]interface{}{"att_key": "att_value"}

	otherMetricName := "OtherCumulativeCountMetric"
	otherMetricValue := 1.22
	otherMetricAttributes := map[string]interface{}{"other_metric_att_key": "att_value"}

	metrics := []protocol.Metric{
		{
			Name:       name,
			Type:       "cumulative-count",
			Value:      json.RawMessage("1.45"),
			Timestamp:  &cannedDateUnix,
			Interval:   &cannedDurationInt,
			Attributes: attributes,
		},
		{

			Name:       otherMetricName,
			Type:       "cumulative-count",
			Value:      json.RawMessage("1.22"),
			Timestamp:  &cannedDateUnix,
			Interval:   &cannedDurationInt,
			Attributes: otherMetricAttributes,
		},
	}

	expectedMetrics :=
		[]telemetry.Metric{
			telemetry.Count{
				Name:       name,
				Value:      val,
				Attributes: attributes,
				Timestamp:  cannedDate,
				Interval:   cannedDuration,
			},
			telemetry.Count{
				Name:       otherMetricName,
				Value:      otherMetricValue,
				Attributes: otherMetricAttributes,
				Timestamp:  cannedDate,
				Interval:   cannedDuration,
			},
		}

	harvester := &mockHarvester{}
	deltaCalculator := &mockDeltaCalculator{}
	deltaCalculator.On("CountMetric", name, attributes, val, cannedDate).Return(expectedMetrics[0], true)
	deltaCalculator.On("CountMetric", otherMetricName, otherMetricAttributes, otherMetricValue, cannedDate).Return(expectedMetrics[1], true)

	s := &sender{
		harvester:  harvester,
		calculator: Calculator{delta: deltaCalculator},
	}

	harvester.On("RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), expectedMetrics).Return(nil)
	err := s.SendMetricsWithCommonAttributes(protocol.Common{
		Timestamp: &cannedDateUnix,
		Interval:  &cannedDurationInt,
		Attributes: map[string]interface{}{
			"one": 1,
			"two": "two",
		},
	}, metrics)
	require.NoError(t, err)
	harvester.AssertExpectations(t)
}

func Test_sender_SendMetric_cumulative_count_invalid_metric(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	cannedDuration, _ := time.ParseDuration("1m7s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()

	metrics := []protocol.Metric{
		{
			Name:       "CumulativeCountInvalidMetric",
			Type:       "cumulative-count",
			Value:      json.RawMessage("1.45"),
			Timestamp:  &cannedDateUnix,
			Interval:   &cannedDurationInt,
			Attributes: map[string]interface{}{"att_key": "att_value"},
		}}
	harvester := &mockHarvester{}
	deltaCalculator := &mockDeltaCalculator{}
	deltaCalculator.On("CountMetric", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(telemetry.Count{}, false)

	s := &sender{
		harvester:  harvester,
		calculator: Calculator{delta: deltaCalculator},
	}

	err := s.SendMetricsWithCommonAttributes(protocol.Common{
		Timestamp: &cannedDateUnix,
		Interval:  &cannedDurationInt,
		Attributes: map[string]interface{}{
			"one": 1,
			"two": "two",
		},
	}, metrics)
	require.NoError(t, err)

	harvester.AssertNotCalled(t, "RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), mock.AnythingOfType("[]telemetry.Metric"))

	// THEN one long entry found
	require.NotEmpty(t, hook.Entries)
	entry := hook.LastEntry()
	assert.Equal(t, "CumulativeCountInvalidMetric", entry.Data["name"])
	assert.Equal(t, noCalculationMadeErrMsg, entry.Message)
	assert.Equal(t, logrus.DebugLevel, entry.Level)
}

func Test_sender_SendMetric_cumulative_count_invalid_metric_value(t *testing.T) {

	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	cannedDuration, _ := time.ParseDuration("1m17s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	harvester := &mockHarvester{}
	deltaCalculator := &mockDeltaCalculator{}
	s := &sender{
		harvester:  harvester,
		calculator: Calculator{delta: deltaCalculator},
	}
	cumulativeType := protocol.MetricType("cumulative-count")
	name := "CumulativeCountMetric"
	metrics := []protocol.Metric{
		{
			Name:      name,
			Type:      cumulativeType,
			Timestamp: &cannedDateUnix,
			Value:     nil,
		},
	}
	err := s.SendMetricsWithCommonAttributes(protocol.Common{
		Timestamp: &cannedDateUnix,
		Interval:  &cannedDurationInt,
		Attributes: map[string]interface{}{
			"one": 1,
			"two": "two",
		},
	}, metrics)
	require.NoError(t, err)

	harvester.AssertNotCalled(t, "RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), mock.AnythingOfType("[]telemetry.Metric"))

	// THEN one long entry found
	require.NotEmpty(t, hook.Entries)
	entry := hook.LastEntry()
	assert.Equal(t, name, entry.Data["name"])
	assert.Equal(t, cumulativeType, entry.Data["metric-type"])
	assert.Equal(t, "received a metric with invalid value", entry.Message)
	assert.EqualError(t, entry.Data["error"].(error), "unexpected end of JSON input")
	assert.Equal(t, logrus.ErrorLevel, entry.Level)

}

func Test_sender_SendMetrics_cumulative_RateCalculator(t *testing.T) {
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	cannedDuration, _ := time.ParseDuration("1m27s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	attributes := map[string]interface{}{"att_key": "att_value"}
	val := 2.45

	tests := []struct {
		name                 string
		rateCalculatorMethod string
		metricType           protocol.MetricType
	}{
		{
			name:                 "CumulativeRateMetric",
			rateCalculatorMethod: "GetCumulativeRate",
			metricType:           protocol.MetricType("cumulative-rate"),
		},
		{
			name:                 "RateMetric",
			rateCalculatorMethod: "GetRate",
			metricType:           protocol.MetricType("rate"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harvester := &mockHarvester{}
			rateCalculator := &mockRateCalculator{}
			s := &sender{
				harvester:  harvester,
				calculator: Calculator{rate: rateCalculator},
			}
			metrics := []protocol.Metric{
				{
					Name:       tt.name,
					Type:       tt.metricType,
					Value:      json.RawMessage("2.45"),
					Timestamp:  &cannedDateUnix,
					Attributes: attributes,
				},
			}
			expectedMetrics := []telemetry.Metric{
				telemetry.Gauge{
					Name:       tt.name,
					Value:      2.45,
					Attributes: map[string]interface{}{"att_key": "att_value"},
					Timestamp:  cannedDate,
				},
			}
			// Set up mock
			rateCalculator.On(tt.rateCalculatorMethod, tt.name, attributes, val, cannedDate).Return(expectedMetrics[0], true).Once()

			harvester.On("RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), expectedMetrics).Return(nil)

			err := s.SendMetricsWithCommonAttributes(protocol.Common{
				Timestamp: &cannedDateUnix,
				Interval:  &cannedDurationInt,
				Attributes: map[string]interface{}{
					"one": 1,
					"two": "two",
				},
			}, metrics)
			require.NoError(t, err)

			rateCalculator.AssertExpectations(t)
			harvester.AssertExpectations(t)
		})
	}
}

func Test_sender_SendMetric_rate_cumulative_invalid_metric(t *testing.T) {
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	cannedDuration, _ := time.ParseDuration("1m37s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	attributes := map[string]interface{}{"att_key": "att_value"}
	val := 2.45

	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name                 string
		metricType           protocol.MetricType
		rateCalculatorMethod string
	}{
		{
			"RateMetric",
			protocol.MetricType("rate"),
			"GetRate",
		},
		{
			"CumulativeRateMetric",
			protocol.MetricType("cumulative-rate"),
			"GetCumulativeRate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harvester := &mockHarvester{}
			rateCalculator := &mockRateCalculator{}
			s := &sender{
				harvester:  harvester,
				calculator: Calculator{rate: rateCalculator},
			}
			metrics := []protocol.Metric{
				{
					Name:       tt.name,
					Type:       tt.metricType,
					Value:      json.RawMessage("2.45"),
					Timestamp:  &cannedDateUnix,
					Attributes: attributes,
				},
			}
			expectedMetrics := []telemetry.Metric{
				telemetry.Gauge{
					Name:       tt.name,
					Value:      2.45,
					Attributes: map[string]interface{}{"att_key": "att_value"},
					Timestamp:  cannedDate,
				},
			}
			// Set up mock
			rateCalculator.On(tt.rateCalculatorMethod, tt.name, attributes, val, cannedDate).Return(expectedMetrics[0], false).Once()

			err := s.SendMetricsWithCommonAttributes(protocol.Common{
				Timestamp: &cannedDateUnix,
				Interval:  &cannedDurationInt,
				Attributes: map[string]interface{}{
					"one": 1,
					"two": "two",
				},
			}, metrics)
			require.NoError(t, err)

			rateCalculator.AssertExpectations(t)
			harvester.AssertNotCalled(t, "RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), mock.AnythingOfType("[]telemetry.Metric"))

			// THEN one long entry found
			require.NotEmpty(t, hook.Entries)
			entry := hook.LastEntry()
			assert.Equal(t, tt.name, entry.Data["name"])
			assert.Equal(t, tt.metricType, entry.Data["metric-type"])
			assert.Equal(t, noCalculationMadeErrMsg, entry.Message)
			assert.Equal(t, logrus.DebugLevel, entry.Level)
		})
	}
}

func Test_sender_SendMetric_rate_cumulative_invalid_metric_value(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	cannedDuration, _ := time.ParseDuration("1m47s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	harvester := &mockHarvester{}
	rateCalculator := &mockRateCalculator{}
	s := &sender{
		harvester:  harvester,
		calculator: Calculator{rate: rateCalculator},
	}

	tests := []struct {
		name       string
		metricType protocol.MetricType
	}{
		{
			"RateMetric",
			"rate",
		},
		{
			"CumulativeRateMetric",
			"cumulative-rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := []protocol.Metric{
				{
					Name:      tt.name,
					Type:      tt.metricType,
					Timestamp: &cannedDateUnix,
					Value:     nil,
				},
			}

			harvester.On("RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), metrics).Return(nil)
			err := s.SendMetricsWithCommonAttributes(protocol.Common{
				Timestamp: &cannedDateUnix,
				Interval:  &cannedDurationInt,
				Attributes: map[string]interface{}{
					"one": 1,
					"two": "two",
				},
			}, metrics)
			require.NoError(t, err)
			harvester.AssertNotCalled(t, "RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), mock.AnythingOfType("protocol.Metric"))

			// THEN one long entry found
			require.NotEmpty(t, hook.Entries)
			entry := hook.LastEntry()
			assert.Equal(t, tt.name, entry.Data["name"])
			assert.Equal(t, tt.metricType, entry.Data["metric-type"])
			assert.Equal(t, "received a metric with invalid value", entry.Message)
			assert.EqualError(t, entry.Data["error"].(error), "unexpected end of JSON input")
			assert.Equal(t, logrus.ErrorLevel, entry.Level)
		})
	}
}

func TestSender_SendMetrics_invalid_metric_type(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	cannedDuration, _ := time.ParseDuration("1m47s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)

	harvester := &mockHarvester{}
	rateCalculator := &mockRateCalculator{}
	s := &sender{
		harvester:  harvester,
		calculator: Calculator{rate: rateCalculator},
	}
	invalidMetric := protocol.Metric{
		Name: "InvalidMetric",
		Type: "invalidType",
	}

	err := s.SendMetricsWithCommonAttributes(protocol.Common{
		Timestamp: &cannedDateUnix,
		Interval:  &cannedDurationInt,
		Attributes: map[string]interface{}{
			"one": 1,
			"two": "two",
		},
	}, []protocol.Metric{invalidMetric})
	require.NoError(t, err)
	harvester.AssertNotCalled(t, "RecordInfraMetrics", mock.AnythingOfType("telemetryapi.Attributes"), mock.AnythingOfType("[]telemetry.Metrics"))

	// THEN one long entry found
	require.NotEmpty(t, hook.Entries)
	entry := hook.LastEntry()
	assert.Equal(t, "received an unknown metric type", entry.Message)
	assert.Equal(t, entry.Data["name"], "InvalidMetric")
	assert.Equal(t, logrus.WarnLevel, entry.Level, "Incorrect log level")
}

type mockHarvester struct {
	mock.Mock
}

func (m *mockHarvester) RecordMetric(metric telemetry.Metric) {
	m.Called(metric)
}

func (m *mockHarvester) RecordInfraMetrics(commonAttributes telemetry.Attributes, metrics []telemetry.Metric) error {
	args := m.Called(commonAttributes, metrics)
	return args.Error(0)
}

type mockRateCalculator struct {
	mock.Mock
}

func (m *mockRateCalculator) GetRate(
	name string,
	attributes map[string]interface{},
	val float64,
	now time.Time) (gauge telemetry.Gauge, valid bool) {

	args := m.Called(name, attributes, val, now)
	return args.Get(0).(telemetry.Gauge), args.Bool(1)
}

func (m *mockRateCalculator) GetCumulativeRate(
	name string,
	attributes map[string]interface{},
	val float64,
	now time.Time) (gauge telemetry.Gauge, valid bool) {

	args := m.Called(name, attributes, val, now)
	return args.Get(0).(telemetry.Gauge), args.Bool(1)
}

func (m *mockRateCalculator) Clean() {
	m.Called()
}

type mockDeltaCalculator struct {
	mock.Mock
}

func (m *mockDeltaCalculator) CountMetric(
	name string,
	attributes map[string]interface{},
	val float64,
	now time.Time) (count telemetry.Count, valid bool) {

	args := m.Called(name, attributes, val, now)
	return args.Get(0).(telemetry.Count), args.Bool(1)
}
