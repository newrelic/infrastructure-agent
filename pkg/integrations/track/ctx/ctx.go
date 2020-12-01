package ctx

// CmdChannelRequest DTO storing context required to handle actions on integration run exit.
type CmdChannelRequest struct {
	CmdChannelCmdName string
	CmdChannelCmdHash string
	IntegrationName   string
	IntegrationArgs   []string
}

// NewTrackCtx create new CmdChannelRequest.
func NewTrackCtx(cmdChanCmdName, cmdChanCmdHash, integrationName string, integrationArgs []string) CmdChannelRequest {
	return CmdChannelRequest{
		CmdChannelCmdName: cmdChanCmdName,
		CmdChannelCmdHash: cmdChanCmdHash,
		IntegrationName:   integrationName,
		IntegrationArgs:   integrationArgs,
	}
}
