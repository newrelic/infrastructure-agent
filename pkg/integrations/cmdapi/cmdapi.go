package cmdapi

var (
	integrationsAllowedToRunStopFromCmdAPI = map[string]struct{}{
		"nri-lsi-java": {},
	}
)

func IsAllowedToRunStopFromCmdAPI(integrationName string) bool {
	_, ok := integrationsAllowedToRunStopFromCmdAPI[integrationName]

	return ok
}
