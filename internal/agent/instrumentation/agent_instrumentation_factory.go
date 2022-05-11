package instrumentation

import (
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

func InitSelfInstrumentation(c *config.Config, resolver hostname.Resolver) {
	if strings.ToLower(c.SelfInstrumentation) == APMInstrumentationName {
		apmSelfInstrumentation, err := NewAgentInstrumentationApm(
			c.License,
			c.SelfInstrumentationApmHost,
			c.MetricURL,
			resolver,
		)
		if err == nil {
			SelfInstrumentation = apmSelfInstrumentation
		}
	}
}
