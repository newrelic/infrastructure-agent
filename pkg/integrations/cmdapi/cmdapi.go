package cmdapi

var (
	integrationsAllowedToRunStopFromCmdAPI = map[string]struct{}{
		"nri-lsi-java":         {},
		"nri-process-detector": {},
	}
)

func IsAllowedToRunStopFromCmdAPI(integrationName string) bool {
	_, ok := integrationsAllowedToRunStopFromCmdAPI[integrationName]

	return ok
}
