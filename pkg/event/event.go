package event

// AttributesPrefix preffix used to prefix attributes with.
const AttributesPrefix = "attr."

// reservedFields reserved event keys.
var reservedFields = map[string]struct{}{
	"":             {},
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
	prefixLen := len(AttributesPrefix)
	if len(field) > prefixLen && field[:prefixLen] == AttributesPrefix {
		return true
	}

	_, ok := reservedFields[field]
	return ok
}
