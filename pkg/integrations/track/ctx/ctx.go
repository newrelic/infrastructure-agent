// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ctx

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
)

// CmdChannelRequest DTO storing context required to handle actions on integration run exit.
type CmdChannelRequest struct {
	CmdChannelCmdName string
	CmdChannelCmdHash string
	IntegrationName   string
	IntegrationArgs   []string
	Metadata          map[string]interface{}
}

// NewCmdChannelRequest create new CmdChannelRequest.
func NewCmdChannelRequest(cmdChanCmdName, cmdChanCmdHash, integrationName string, integrationArgs []string, metadata map[string]interface{}) CmdChannelRequest {
	return CmdChannelRequest{
		CmdChannelCmdName: cmdChanCmdName,
		CmdChannelCmdHash: cmdChanCmdHash,
		IntegrationName:   integrationName,
		IntegrationArgs:   integrationArgs,
		Metadata:          metadata,
	}
}

func (r *CmdChannelRequest) Event(summary string) protocol.EventData {
	ev := protocol.EventData{
		"eventType":     "InfrastructureEvent",
		"category":      "notifications",
		"summary":       summary,
		"cmd_name":      r.CmdChannelCmdName,
		"cmd_hash":      r.CmdChannelCmdHash,
		"cmd_args_name": r.IntegrationName,
		"cmd_args_args": fmt.Sprintf("%+v", r.IntegrationArgs),
	}
	for k, v := range r.Metadata {
		ev["cmd_metadata."+k] = v
	}
	return ev
}
