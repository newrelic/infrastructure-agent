// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

//go:build fips

package main

import _ "crypto/tls/fipsonly"
