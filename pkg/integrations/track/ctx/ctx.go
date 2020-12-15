// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ctx

// CmdChannelRequest DTO storing context required to handle actions on integration run exit.
type CmdChannelRequest struct {
	CmdChannelCmdName string
	CmdChannelCmdHash string
	IntegrationName   string
	IntegrationArgs   []string
}

// NewCmdChannelRequest create new CmdChannelRequest.
func NewCmdChannelRequest(cmdChanCmdName, cmdChanCmdHash, integrationName string, integrationArgs []string) CmdChannelRequest {
	return CmdChannelRequest{
		CmdChannelCmdName: cmdChanCmdName,
		CmdChannelCmdHash: cmdChanCmdHash,
		IntegrationName:   integrationName,
		IntegrationArgs:   integrationArgs,
	}
}
