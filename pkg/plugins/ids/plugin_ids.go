// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package ids

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultInventoryCategory = "integration"
)

// PluginID identifies a plugin in a taxonomy-like form, given a Category name (e.g. `kernel`) and a Term (e.g. `sysctl`).
// Implements the next interfaces: json.Marshaler, json.Unmarshaler, yaml.Unmarshaler, fmt.Stringer, agent.Sortable
type PluginID struct {
	Category string
	Term     string
}

// PluginID list:
var (
	CustomAttrsID = PluginID{
		Category: "metadata",
		Term:     "attributes",
	}
	HostInfo = PluginID{
		Category: "metadata",
		Term:     "system",
	}
	EmptyInventorySource = PluginID{}
)

// NewPluginID creates a new PluginID.
func NewPluginID(category, term string) *PluginID {
	return &PluginID{
		Category: category,
		Term:     term,
	}
}

// NewDefaultInventoryPluginID creates a PluginID using default inventory category
func NewDefaultInventoryPluginID(term string) PluginID {
	return PluginID{
		Category: DefaultInventoryCategory,
		Term:     term,
	}
}

// FromString creates a PluginID from a category/term string, returning error
// if it is not properly formatted
func FromString(source string) (PluginID, error) {
	parts := strings.Split(source, "/")
	if len(parts) != 2 {
		return PluginID{}, errors.New("inventory source must be " +
			"in the form 'category/term'. Got: " + source)
	}
	return PluginID{Category: parts[0], Term: parts[1]}, nil
}

// UnmarshalYAML populates the PluginID given the yaml.Unmarshaler interface
func (p *PluginID) UnmarshalYAML(unmarshal func(interface{}) error) error {
	contents := new(string)
	err := unmarshal(contents)
	if err != nil {
		return err
	}
	return p.unmarshalBytes([]byte(*contents))
}

const pluginCategorySeparator = "/"

// SortKey returns the "category/term" string representation of the PluginID, ready to be used as a sorting key
func (p PluginID) SortKey() string {
	return p.String()
}

// String returns the "category/term" string representation of the PluginID
func (p PluginID) String() string {
	return p.Category + pluginCategorySeparator + p.Term
}

func (p *PluginID) unmarshalBytes(field []byte) error {
	sep := bytes.IndexByte(field, pluginCategorySeparator[0])
	if sep < 0 {
		return fmt.Errorf("plugin Identifier %q is not in the form 'category/term'", string(field))
	}
	p.Category = string(field[:sep])
	p.Term = string(field[sep+1:])
	return nil
}

// MarshallJSON creates a JSON `category/term` string representation
func (p PluginID) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshallJSON creates a PluginID from its JSON `category/term` string representation
func (p *PluginID) UnmarshalJSON(field []byte) error {
	field = bytes.TrimPrefix(field, []byte(`"`))
	field = bytes.TrimSuffix(field, []byte(`"`))

	return p.unmarshalBytes(field)
}
