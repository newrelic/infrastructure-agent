package ctx

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
