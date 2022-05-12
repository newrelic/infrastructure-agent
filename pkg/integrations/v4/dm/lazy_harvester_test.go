// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/backend/telemetryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/require"
)

func Test_lazyLoadHarvester_RecordMetric(t *testing.T) {
	conf := NewConfig("", false, "", time.Second, 1, 1)
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}

	l := NewLazyLoadedHarvester(conf, http.DefaultTransport, emptyIDProvide)

	ll, ok := l.(*lazyLoadHarvester)
	require.True(t, ok)

	require.IsType(t, nil, ll.harvester)
	l.RecordMetric(telemetryapi.Count{
		Name:           "myCount",
		AttributesJSON: json.RawMessage(`{"foo":"bar"}`),
		Value:          123,
		Timestamp:      time.Now(),
		Interval:       1 * time.Second,
	})
	require.IsType(t, &telemetryapi.Harvester{}, ll.harvester)
}
func Test_lazyLoadHarvester_RecordInfraMetrics(t *testing.T) {
	conf := NewConfig("", false, "", time.Second, 1, 1)
	emptyIDProvide := func() entity.Identity {
		return entity.EmptyIdentity
	}

	l := NewLazyLoadedHarvester(conf, http.DefaultTransport, emptyIDProvide)

	ll, ok := l.(*lazyLoadHarvester)
	require.True(t, ok)

	require.IsType(t, nil, ll.harvester)
	ats := telemetryapi.Attributes(map[string]interface{}{
		"foo": "bar",
	})
	myCount := telemetryapi.Count{
		Name:           "myCount",
		AttributesJSON: json.RawMessage(`{"foo":"bar"}`),
		Value:          123,
		Timestamp:      time.Now(),
		Interval:       1 * time.Second,
	}
	l.RecordInfraMetrics(ats, []telemetryapi.Metric{myCount})
	require.IsType(t, &telemetryapi.Harvester{}, ll.harvester)
}
