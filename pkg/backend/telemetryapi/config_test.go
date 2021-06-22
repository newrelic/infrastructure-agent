// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetryapi

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestConfigAPIKey(t *testing.T) {
	apikey := "apikey"
	h, err := NewHarvester(ConfigAPIKey(apikey))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if h.config.APIKey != apikey {
		t.Error("config func does not set APIKey correctly")
	}
}

func TestConfigMissingAPIKey(t *testing.T) {
	h, err := NewHarvester()
	if nil != h || err != errAPIKeyUnset {
		t.Fatal(h, err)
	}
}

func TestConfigHarvestPeriod(t *testing.T) {
	h, err := NewHarvester(ConfigAPIKey("apikey"), ConfigHarvestPeriod(0))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if 0 != h.config.HarvestPeriod {
		t.Error("config func does not set harvest period correctly")
	}
}

func TestConfigBasicErrorLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	h, err := NewHarvester(configTesting, ConfigBasicErrorLogger(buf))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	buf.Reset()
	h.config.logError(map[string]interface{}{"zip": "zap"})
	if log := buf.String(); !strings.Contains(log, "{\"zip\":\"zap\"}") {
		t.Error("message not logged correctly", log)
	}

	buf.Reset()
	h.config.logError(map[string]interface{}{"zip": func() {}})
	if log := buf.String(); !strings.Contains(log, "json: unsupported type: func()") {
		t.Error("message not logged correctly", log)
	}
}

func TestConfigContext(t *testing.T) {
	ctx := context.Background()
	h, err := NewHarvester(ConfigAPIKey("apikey"), ConfigContext(ctx))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if ctx != h.config.Context {
		t.Error("config func does not set context correctly")
	}
}

func TestConfigBasicDebugLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	h, err := NewHarvester(configTesting, ConfigBasicDebugLogger(buf))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	buf.Reset()
	h.config.logDebug(map[string]interface{}{"zip": "zap"})
	if log := buf.String(); !strings.Contains(log, "{\"zip\":\"zap\"}") {
		t.Error("message not logged correctly", log)
	}

	buf.Reset()
	h.config.logDebug(map[string]interface{}{"zip": func() {}})
	if log := buf.String(); !strings.Contains(log, "json: unsupported type: func()") {
		t.Error("message not logged correctly", log)
	}
}

func TestConfigAuditLogger(t *testing.T) {
	h, err := NewHarvester(configTesting)
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if enabled := h.config.auditLogEnabled(); enabled {
		t.Error("audit logging should not be enabled", enabled)
	}
	// This should not panic.
	h.config.logAudit(map[string]interface{}{"zip": "zap"})

	buf := new(bytes.Buffer)
	h, err = NewHarvester(configTesting, ConfigBasicAuditLogger(buf))
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if enabled := h.config.auditLogEnabled(); !enabled {
		t.Error("audit logging should be enabled", enabled)
	}
	h.config.logAudit(map[string]interface{}{"zip": "zap"})
	if lg := buf.String(); !strings.Contains(lg, `{"zip":"zap"}`) {
		t.Error("audit message not logged correctly", lg)
	}
}

func TestConfigMetricURL(t *testing.T) {
	h, err := NewHarvester(configTesting)
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if u := h.config.metricURL(); u != defaultMetricURL {
		t.Fatal(u)
	}

	h, err = NewHarvester(configTesting, func(cfg *Config) {
		cfg.MetricsURLOverride = "metric-url-override"
	})
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if u := h.config.metricURL(); u != "metric-url-override" {
		t.Fatal(u)
	}

	h, err = NewHarvester(func(cfg *Config) {
		cfg.APIKey = "api-key"
		cfg.Fedramp = true
	})
	if u := h.config.metricURL(); u != defaultMetricURLGov {
		t.Fatal(u)
	}
}

func TestConfigSpanURL(t *testing.T) {
	h, err := NewHarvester(configTesting)
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if u := h.config.spanURL(); u != defaultSpanURL {
		t.Fatal(u)
	}

	h, err = NewHarvester(configTesting, func(cfg *Config) {
		cfg.SpansURLOverride = "span-url-override"
	})
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if u := h.config.spanURL(); u != "span-url-override" {
		t.Fatal(u)
	}

	h, err = NewHarvester(func(cfg *Config) {
		cfg.APIKey = "api-key"
		cfg.Fedramp = true
	})
	if nil == h || err != nil {
		t.Fatal(h, err)
	}
	if u := h.config.spanURL(); u != defaultSpanURLGov {
		t.Fatal(u)
	}
}

func TestConfigUserAgent(t *testing.T) {
	testcases := []struct {
		option func(*Config)
		expect string
	}{
		{
			option: func(*Config) {},
			expect: "NewRelic-Go-TelemetrySDK/" + version,
		},
		{
			option: func(cfg *Config) {
				cfg.Product = "myProduct"
			},
			expect: "NewRelic-Go-TelemetrySDK/" + version + " myProduct",
		},
		{
			option: func(cfg *Config) {
				cfg.Product = "myProduct"
				cfg.ProductVersion = "0.1.0"
			},
			expect: "NewRelic-Go-TelemetrySDK/" + version + " myProduct/0.1.0",
		},
		{
			option: func(cfg *Config) {
				// Only use ProductVersion if Product is set.
				cfg.ProductVersion = "0.1.0"
			},
			expect: "NewRelic-Go-TelemetrySDK/" + version,
		},
	}

	for idx, tc := range testcases {
		h, err := NewHarvester(configTesting, tc.option)
		if nil == h || err != nil {
			t.Fatal(h, err)
		}
		agent := h.config.userAgent()
		if agent != tc.expect {
			t.Error(idx, tc.expect, agent)
		}
	}
}

func TestConfigMaxEntitiesPerRequest(t *testing.T) {
	// Default
	h, err := NewHarvester(
		ConfigAPIKey("TestConfigMaxEntitiesPerRequest"))

	if h == nil || err != nil {
		t.Error("Failed to initialize harvester with err: ", err)
	}

	if h.config.MaxEntitiesPerRequest != DefaultMaxEntitiesPerRequest {
		t.Error("Expected to 'MaxEntitiesPerRequest' setting be: ", DefaultMaxEntitiesPerRequest)
	}

	// With expected value
	expectedMax := 1

	h, err = NewHarvester(
		ConfigAPIKey("TestConfigMax"),
		ConfigMaxEntitiesPerRequest(expectedMax))

	if h == nil || err != nil {
		t.Error("Failed to initialize harvester with err: ", err)
	}

	if h.config.MaxEntitiesPerRequest != expectedMax {
		t.Error("Expected to 'MaxEntitiesPerRequest' setting be: ", expectedMax)
	}
}

func TestConfigMaxEntitiesPerBatch(t *testing.T) {
	// Default
	h, err := NewHarvester(
		ConfigAPIKey("TestConfigMaxEntitiesPerBatch"))

	if h == nil || err != nil {
		t.Error("Failed to initialize harvester with err: ", err)
	}

	if h.config.MaxEntitiesPerBatch != DefaultMaxEntitiesPerBatch {
		t.Error("Expected to 'MaxEntitiesPerBatch' setting be: ", DefaultMaxEntitiesPerBatch)
	}

	// With expected value
	expectedMax := 1

	h, err = NewHarvester(
		ConfigAPIKey("TestConfigMax"),
		ConfigMaxEntitiesPerBatch(expectedMax))

	if h == nil || err != nil {
		t.Error("Failed to initialize harvester with err: ", err)
	}

	if h.config.MaxEntitiesPerBatch != expectedMax {
		t.Error("Expected to 'MaxEntitiesPerBatch' setting be: ", expectedMax)
	}
}
