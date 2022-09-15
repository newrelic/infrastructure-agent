// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import "github.com/newrelic/infrastructure-agent/pkg/helpers"

// getProcessorData return the processor information.
func getProcessorData(output string) processorInfo {
	return processorInfo{
		ProcessorName:      helpers.SplitRightSubstring(output, "Processor Name: ", "\n"),
		NumberOfProcessors: helpers.SplitRightSubstring(output, "Number of Processors: ", "\n"),
		ProcessorSpeed:     helpers.SplitRightSubstring(output, "Processor Speed: ", "\n"),
		TotalNumberOfCores: helpers.SplitRightSubstring(output, "Total Number of Cores: ", "\n"),
	}
}
