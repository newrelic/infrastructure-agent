package emitter

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/executor"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

var nginxOutput = `{
	"name": "com.newrelic.nginx",
	"protocol_version": "3",
	"integration_version": "0.0.0",
	"data": [{
		"entity": {
			"name": "0.0.0.0:8080",
			"type": "server",
			"id_attributes": []
		},
		"metrics": [{
			"event_type": "NginxSample",
			"hostname": "0.0.0.0",
			"net.connectionsAcceptedPerSecond": 0.017241379310344827,
			"net.connectionsActive": 1,
			"net.connectionsDroppedPerSecond": 0,
			"net.connectionsReading": 0,
			"net.connectionsWaiting": 0,
			"net.connectionsWriting": 1,
			"net.requestsPerSecond": 0.017241379310344827,
			"port": "8080",
			"software.edition": "open source",
			"software.version": "1.15.8"
		}],
		"inventory": {},
		"events": []
	}]
}`

func TestEmit(t *testing.T) {
	def := integration.Definition{
		Name: "nri-nginx",
		Labels: map[string]string{
			"env":  "production",
			"role": "load_balancer",
		},
		ExecutorConfig: executor.Config{
			Environment: map[string]string{
				"METRICS":             "true",
				"STATUS_URL":          "http://0.0.0.0:8080/status",
				"STATUS_MODULE":       "discover",
				"REMOTE_MONITORING":   "true",
				"NRI_CONFIG_INTERVAL": "30s",
			},
			Passthrough: []string{
				"PATH",
			},
		},
		Interval: 30 * time.Second,
		Timeout:  120 * time.Second,
	}

	expected := `----------
Integration Name: nri-nginx
Integration Output: {
	"name": "com.newrelic.nginx",
	"protocol_version": "3",
	"integration_version": "0.0.0",
	"data": [{
		"entity": {
			"name": "0.0.0.0:8080",
			"type": "server",
			"id_attributes": []
		},
		"metrics": [{
			"event_type": "NginxSample",
			"hostname": "0.0.0.0",
			"net.connectionsAcceptedPerSecond": 0.017241379310344827,
			"net.connectionsActive": 1,
			"net.connectionsDroppedPerSecond": 0,
			"net.connectionsReading": 0,
			"net.connectionsWaiting": 0,
			"net.connectionsWriting": 1,
			"net.requestsPerSecond": 0.017241379310344827,
			"port": "8080",
			"software.edition": "open source",
			"software.version": "1.15.8"
		}],
		"inventory": {},
		"events": []
	}]
}
----------
`

	var out strings.Builder

	// GIVEN a new PrintEmitter with a SimpleFormat
	emitter := NewPrintEmitter(&out, NewSimpleFormat())

	// WHEN Emit
	err := emitter.Emit(def, nil, nil, []byte(nginxOutput))

	// THEN no error occurs
	assert.NoError(t, err)

	// AND the output is expected
	assert.Equal(t, expected, out.String())
}

func TestEmit_Error(t *testing.T) {
	var out strings.Builder

	// GIVEN a new PintEmitter with nil writer
	emitter := NewPrintEmitter(nil, nil)

	// WHEN Emit
	err := emitter.Emit(integration.Definition{}, nil, nil, []byte(nginxOutput))

	// THEN error reported
	assert.Error(t, err)

	// AND no output
	assert.Len(t, out.String(), 0)
}
