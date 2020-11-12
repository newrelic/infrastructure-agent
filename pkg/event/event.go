package event

// reservedFields reserved event keys.
var reservedFields = map[string]struct{}{
	"timestamp":    {},
	"eventType":    {},
	"entityID":     {},
	"entityGuid":   {},
	"entityKey":    {},
	"entityName":   {},
	"hostname":     {},
	"fullHostname": {},
	"displayName":  {},
	"agentName":    {},
	"coreCount":    {},
}

// IsReserved returns true when field name is a reserved key.
func IsReserved(field string) bool {
	_, ok := reservedFields[field]
	return ok
}
