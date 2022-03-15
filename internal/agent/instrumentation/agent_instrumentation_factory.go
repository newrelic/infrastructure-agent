package instrumentation

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"strings"
)

func InitSelfInstrumentation(c *config.Config, resolver hostname.Resolver) {
	if strings.ToLower(c.SelfInstrumentation) == apmInstrumentationName {
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
