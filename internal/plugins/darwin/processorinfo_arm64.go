// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package darwin

import "github.com/newrelic/infrastructure-agent/pkg/helpers"

// getProcessorData returns the processor information.
func getProcessorData(output string) processorInfo {
	return processorInfo{
		ProcessorName: helpers.SplitRightSubstring(output, "Chip: ", "\n"),
		// arm architectures have one processor.
		NumberOfProcessors: "1",
		// Apple doesn’t particularly expose the clock speed on Apple silicon configurations.
		ProcessorSpeed:     "",
		TotalNumberOfCores: helpers.SplitRightSubstring(output, "Total Number of Cores: ", " "),
	}
}
