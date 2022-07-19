package configrequest

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/cache"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	logFailedDefinition     = "Cannot create integration definition for config protocol"
	logFailedConfigTemplate = "Cannot create config for the integration definition"
	logAddedDefinition      = "New definition added to the cache"
	logRemovedDefinition    = "Removed definition from cache"
)

var (
	// helper for testing purposes
	NoopHandleFn = func(configProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition) {}
)

type Entry struct {
	Definition integration.Definition
	Databind   databind.YAMLConfig
}

type HandleFn func(cfgProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition)

// NewHandleFn creates a handler func that runs every command within the request batch independently.
// Each command is run in parallel and won't depend on the results of the other ones.
func NewHandleFn(configProtocolQueue chan<- Entry, terminateDefinitionQueue chan<- string, il integration.InstancesLookup, logger log.Entry) HandleFn {
	return func(cfgProtocol protocol.ConfigProtocol, c cache.Cache, parentDefinition integration.Definition) {
		cfgDefinitions := c.TakeConfig(cfgProtocol.Name())
		logCtx := logrus.Fields{
			"cfg_protocol_version":    cfgProtocol.Version(),
			"cfg_name":                cfgProtocol.Name(),
			"parent_integration_name": parentDefinition.Name,
		}
		for _, ce := range cfgProtocol.Integrations() {
			template, err := integration.LoadConfigTemplate(ce.TemplatePath, ce.Config)
			if err != nil {
				logger.WithError(err).WithFields(logCtx).Warn(logFailedConfigTemplate)
				return
			}

			// Add parent tags.
			for k, v := range parentDefinition.Tags {
				if ce.Tags == nil {
					ce.Tags = make(map[string]string)
				}
				// Do not overwrite tags received from config protocol.
				if _, found := ce.Tags[k]; !found {
					ce.Tags[k] = v
				}
			}

			def, err := integration.NewDefinition(ce, il, parentDefinition.ExecutorConfig.Passthrough, template)
			if err != nil {
				logger.WithError(err).WithFields(logCtx).Warn(logFailedDefinition)
				return
			}

			def.CfgProtocol = &protocol.Context{ParentName: parentDefinition.Name, ConfigName: cfgProtocol.Name()}

			if cfgDefinitions.Add(def) {
				logger.WithFields(logCtx).WithField("definition_name", def.Name).Debug(logAddedDefinition)
				configProtocolQueue <- Entry{def, cfgProtocol.GetConfig()}
			}
		}
		removedDefinitions := c.ApplyConfig(cfgDefinitions)
		for _, rd := range removedDefinitions {
			logger.WithFields(logCtx).WithField("definition_name", rd.Name).Debug(logRemovedDefinition)
			terminateDefinitionQueue <- rd.Hash()
		}
	}
}
