package runner

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runner_Run(t *testing.T) {
	def, err := integration.NewDefinition(config.ConfigEntry{
		InstanceName: "foo",
		Exec:         testhelp.Command(fixtures.IntegrationScript, "bar"),
	}, integration.ErrLookup, nil, nil)
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}
	r := NewRunner(def, e, nil, nil, cmdrequest.NoopHandleFn)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	r.Run(ctx, nil, nil)

	dataset, err := e.ReceiveFrom("foo")
	require.NoError(t, err)
	metrics := dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	assert.Equal(t, "TestSample", metrics[0]["event_type"])
	assert.Equal(t, "bar", metrics[0]["value"])
	assert.Empty(t, dataset.Metadata.Labels)
}
