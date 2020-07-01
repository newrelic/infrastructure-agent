package test

import "github.com/newrelic/infrastructure-agent/internal/feature_flags"

// EmptyFFRetriever retriever that always returns FFs as non existing.
var EmptyFFRetriever = &emptyFFRetriever{}

type emptyFFRetriever struct{}

func (e *emptyFFRetriever) GetFeatureFlag(name string) (enabled bool, exists bool) {
	return false, false
}

// NewFFRetrieverReturning creates a new FFRetriever mock returning provided values.
func NewFFRetrieverReturning(enabled bool, exists bool) feature_flags.Retriever {
	return &fakeFeatureFlagRetriever{
		enabled: enabled,
		exists:  exists,
	}
}

type fakeFeatureFlagRetriever struct {
	enabled bool
	exists  bool
}

func (ff *fakeFeatureFlagRetriever) GetFeatureFlag(name string) (enabled bool, exists bool) {
	return ff.enabled, ff.exists
}
