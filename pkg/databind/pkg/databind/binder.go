// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"errors"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

var (
	ErrDataNotFound = errors.New("data value not found")
	ErrDataInvalid  = errors.New("data must be an map")
)

// ValuesWithTTL is the interface that a gatherer returned struct can implement to
// allow us detecting if a secret has an expiration.
type ValuesWithTTL interface {
	// TTL returns the duration of the secret to expire.
	// In case that the ttl is not present it should return ErrTTLNotFound
	// In case the ttl is present but invalid it should return ErrTTLInvalid
	TTL() (time.Duration, error)
	// Data returns the data from a payload that contains a TTL. It's the struct responsibility
	// to decide how TTL and Data are structured.
	// In case the Data is not found it should return ErrDataNotFound
	// In case that Data is not structured properly it should return ErrDataInvalid
	Data() (map[string]interface{}, error)
}

// Sources holds the configuration of all the discovery and variable sources.
// It is built from the LoadYAML function
type Sources struct {
	clock      func() time.Time
	discoverer *discoverer
	Info       DiscovererInfo
	variables  map[string]*gatherer // key: variable name
}

func (s *Sources) GetSoonestTTL() time.Time {
	var soonestExpiration time.Time
	for _, v := range s.variables {
		expTime := v.cache.getExpirationTime()

		if soonestExpiration.IsZero() || expTime.Before(soonestExpiration) {
			soonestExpiration = expTime
		}
	}

	return soonestExpiration
}

// NewValues returns an instance of value
func NewValues(vars data.Map, discoveries ...discovery.Discovery) Values {
	return Values{
		vars:   vars,
		discov: discoveries,
	}
}

// NewDiscovery returns an instance of discovery.Discovery aimed to be used for testing as prod should come from unmarshalling.
func NewDiscovery(variables data.Map, metricAnnotations data.InterfaceMap, entityRewrites []data.EntityRewrite) discovery.Discovery {
	return discovery.Discovery{
		Variables:         variables,
		MetricAnnotations: metricAnnotations,
		EntityRewrites:    entityRewrites,
	}
}

// The outcome of a sources Fetch process. It keeps both variables (secrets) and discovered matches
type Values struct {
	vars data.Map
	// discovered, non-secret data. Only one discovery property (with multiple fields) is allowed
	discov []discovery.Discovery
}

// VarsLen amount of variables to be replaced.
func (v *Values) VarsLen() int {
	return len(v.vars)
}

// Fetch queries the Sources for discovery data and user-defined variables, and returns the
// acquired Values.
func Fetch(ctx *Sources) (Values, error) {
	now := ctx.clock()
	vals := NewValues(data.Map{})
	if ctx.discoverer != nil {
		matches, err := ctx.discoverer.do(now)
		if err != nil {
			return vals, err
		}
		vals.discov = matches
	}

	for varName, gatherer := range ctx.variables {
		value, err := gatherer.do(now)
		if err != nil {
			return vals, err
		}
		data.AddValues(vals.vars, varName, value)
	}

	return vals, nil
}

// Binder wraps the functions provided by this package
type Binder interface {
	// Fetch queries the Sources for discovery data and user-defined variables, and returns the
	// acquired Values.
	Fetch(ctx *Sources) (Values, error)

	// Replace receives one template, which may be a map or a struct whose string fields may
	// contain ${variable} placeholders, and returns an array of items of the same type of the
	// template, but replacing the variable placeholders from the respective Values.
	// The Values of type "variable" are the same for all the returned values. The returned
	// array contains one instance per each "discovered" data value.
	Replace(vals *Values, template interface{}, options ...ReplaceOption) (transformedData []data.Transformed, err error)
}

// New returns an instance of Binder
func New() Binder {
	return &binderWrapper{}
}

type binderWrapper struct{}

func (b *binderWrapper) Fetch(ctx *Sources) (Values, error) {
	return Fetch(ctx)
}

func (b *binderWrapper) Replace(vals *Values, template interface{}, options ...ReplaceOption) (transformedData []data.Transformed, err error) {
	return Replace(vals, template, options...)
}
