// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package counter

// ByKind allows counting the number of matches of a given kind
type ByKind map[string]int

// Count increments the match of a kind and returns how many items of this kind have been reached
// previously
func (bk ByKind) Count(kind string) int {
	soFar := bk[kind]
	bk[kind] = soFar + 1
	return soFar
}
