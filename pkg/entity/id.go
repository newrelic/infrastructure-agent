// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ID entity ID
type ID int64

// GUID entity GUID
type GUID string

// Identity entity identifiers
type Identity struct {
	ID   ID
	GUID GUID
}

// Key entity key
type Key string

const (
	EmptyKey  = Key("")
	EmptyID   = ID(0)
	EmptyGUID = GUID("")
)

var (
	EmptyIdentity = Identity{
		ID:   EmptyID,
		GUID: EmptyGUID,
	}
)

// String stringer stuff
func (k Key) String() string {
	return string(k)
}

// IsEmpty returns if key is empty
func (k Key) IsEmpty() bool {
	return k == EmptyKey
}

// String stringer stuff
func (i ID) String() string {
	return strconv.FormatInt(int64(i), 10)
}

// IsEmpty returns if ID is empty
func (i ID) IsEmpty() bool {
	return i == EmptyID
}

// String stringer stuff
func (g GUID) String() string {
	return string(g)
}

// Type is the type of an Entity
type Type string

// IDAttribute is an attribute which defines uniqueness in the entity key.
type IDAttribute struct {
	Key   string
	Value string
}

//IDAttributes this sorted list ensures uniqueness for the entity key.
type IDAttributes []IDAttribute

// Fields store the identifying fields of an entity, which can be used to compose the entity Key
type Fields struct {
	Name         string       `json:"name"`
	Type         Type         `json:"type"`
	IDAttributes IDAttributes `json:"id_attributes"`
}

// IsAgent returns if entity is (local) agent.
func (f *Fields) IsAgent() bool {
	return len(f.Name) == 0
}

//NewKey generates an entity Key to uniquely identify this entity
func (f *Fields) Key() (Key, error) {
	if len(f.Name) == 0 {
		return EmptyKey, nil // Empty value means this agent's default entity identifier
	}
	if f.Type == "" {
		//invalid entity: it has name, but not type.
		return EmptyKey, fmt.Errorf("missing 'type' field for entity name '%v'", f.Name)
	}

	attrsStr := ""
	sort.Sort(f.IDAttributes)
	f.IDAttributes.removeEmptyAndDups()
	for _, attr := range f.IDAttributes {
		attrsStr = fmt.Sprintf("%v:%v=%v", attrsStr, attr.Key, attr.Value)
	}

	return Key(fmt.Sprintf("%v:%v%s", f.Type, f.Name, strings.ToLower(attrsStr))), nil
}

// Len is part of sort.Interface.
func (a IDAttributes) Len() int {
	return len(a)
}

// Swap is part of sort.Interface.
func (a IDAttributes) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// Less is part of sort.Interface.
func (a IDAttributes) Less(i, j int) bool {
	return a[i].Key < a[j].Key
}

func (a *IDAttributes) removeEmptyAndDups() {

	var uniques IDAttributes
	var prev IDAttribute
	for i, attr := range *a {
		if prev.Key != attr.Key && attr.Key != "" {
			uniques = append(uniques, attr)
		} else if uniques.Len() >= 1 {
			uniques[i-1].Value = attr.Value
		}
		prev = attr
	}

	*a = uniques
}
