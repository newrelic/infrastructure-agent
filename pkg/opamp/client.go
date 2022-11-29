// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package opamp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/oklog/ulid/v2"
	"github.com/open-telemetry/opamp-go/client/types"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	hostnameResolver "github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/protobufs"
)

var (
	opamServerUrl          = "https://opamp.staging-service.newrelic.com/v1/opamp"
	effectiveConfigDefault = "/etc/newrelic-infra-effective.yml"
)

type Client struct {
	opamClient client.OpAMPClient
	ctx        context.Context
	logger     log.Entry
	// effectiveCnf the applied config. It's the result of merging local config + remote config
	effectiveCnf     *config.Config
	effectiveCnfLock sync.RWMutex
	// effectiveCnfExists tracks if effective config file existed and was not empty before running the agent
	effectiveCnfExists bool
	// changedConfigCh
	agentChangedConfigCh chan<- struct{}

	// messageLock not sure if needed, but seems safe to handle one by one
	messageLock sync.Mutex
}

func NewClient(ctx context.Context, agentChangedConfigCh chan<- struct{}, logger log.Entry, buildVersion string) (*Client, error) {
	clt := client.NewHTTP(logger)
	err := withAgentDescription(clt, buildVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot set description: %w", err)
	}

	return &Client{
		ctx:                  ctx,
		opamClient:           clt,
		logger:               logger,
		agentChangedConfigCh: agentChangedConfigCh,
	}, nil
}

func (cl *Client) prepareEffectiveConfig() error {
	var err error
	// try load effective config from effective config file
	if fileExists(effectiveConfigDefault) && !isEmpty(effectiveConfigDefault) {
		cl.effectiveCnf, err = config.LoadConfig(effectiveConfigDefault)
		// necessary to react to empty conf coming form the server. If emtpy conf comes from server
		// we should only restart the server if the current conf was not empty and then loaded from default
		cl.effectiveCnfExists = true
		return err
	}

	// fallback to local config when effective is not present
	cl.effectiveCnf, err = config.LoadConfig("")
	if err != nil {
		return err
	}
	cl.logger.Infof("effective config from locaL")

	return nil
}

func (cl *Client) Start() error {
	err := cl.prepareEffectiveConfig()
	if err != nil {
		cl.logger.WithError(err).Error("cannot prepare effective config")
		return err
	}

	return cl.opamClient.Start(cl.ctx, cl.startSettings())
}

func withAgentDescription(clt client.OpAMPClient, buildVersion string) error {
	// For standalone running Agents (such as OpenTelemetry Collector) the following
	// attributes SHOULD be specified:
	// - service.name should be set to a reverse FQDN that uniquely identifies the
	//   Agent type, e.g. "io.opentelemetry.collector"
	// - service.namespace if it is used in the environment where the Agent runs.
	// - service.version should be set to version number of the Agent build.
	// - service.instance.id should be set. It may be be set equal to the Agent's
	//   instance uid (equal to ServerToAgent.instance_uid field) or any other value
	//   that uniquely identifies the Agent in combination with other attributes.
	// - any other attributes that are necessary for uniquely identifying the Agent's
	//   own telemetry.
	serviceName := "com.newrelic.infrastructure-agent"
	serviceVersion := buildVersion

	resolver := hostnameResolver.CreateResolver("", "", false)
	hostname := resolver.Long()

	return clt.SetAgentDescription(&protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: serviceName},
				},
			},
			{
				Key: "service.version",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{StringValue: serviceVersion},
				},
			},
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "host.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: hostname,
					},
				},
			},
		},
	})
}

func (cl *Client) startSettings() types.StartSettings {
	return types.StartSettings{
		OpAMPServerURL: opamServerUrl,
		Header:         cl.headers(),
		InstanceUid:    cl.instanceId().String(),
		Callbacks: types.CallbacksStruct{
			OnConnectFunc:       cl.opampCbOnConnect(),
			OnConnectFailedFunc: cl.opampCbOnConnectFailed(),
			OnErrorFunc:         cl.opampCbOnError(),
			// SaveRemoteConfigStatusFunc: cl.opampCbSaveRemoteConfigStatusFunc, //TODO
			GetEffectiveConfigFunc: cl.opampCbGetEffectiveConfigFunc,
			OnMessageFunc:          cl.onMessage,
		},
		// RemoteConfigStatus: remoteConfigStatus, //TODO do we want to store locally a cache of remote config¿? Does even make sense?
		Capabilities: protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
			protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics,
	}
}

// TODO find logic to handle ulid <-> agentGuid
func (cl *Client) instanceId() ulid.ULID {
	return ulid.MustParse("01GK1Y6ZQDN45ND467S7D191KF")
}

func (cl *Client) headers() http.Header {
	return http.Header{
		"api-key": []string{cl.effectiveCnf.License},
	}
}

func (cl *Client) opampCbOnConnect() func() {
	return func() {
		cl.logger.Debugf("Connected to the server.")
	}
}

func (cl *Client) opampCbOnConnectFailed() func(err error) {
	return func(err error) {
		cl.logger.Errorf("Failed to connect to the server: %v", err)
	}
}

func (cl *Client) opampCbOnError() func(err *protobufs.ServerErrorResponse) {
	return func(err *protobufs.ServerErrorResponse) {
		cl.logger.Errorf("Server returned an error response: %v", err.ErrorMessage)
	}
}

//func (cl *Client) opampCbSaveRemoteConfigStatusFunc(_ context.Context, status *protobufs.RemoteConfigStatus) {
//	s.logger.Info(fmt.Sprintf("SaveRemoteConfigStatusFunc: %s", status.String()))
//	remoteConfigStatus = status
//}

func (cl *Client) opampCbGetEffectiveConfigFunc(_ context.Context) (*protobufs.EffectiveConfig, error) {
	cl.effectiveCnfLock.RLock()
	defer cl.effectiveCnfLock.RUnlock()

	var err error
	content := []byte("")

	if cl.effectiveCnf != nil {
		content, err = yaml.Marshal(cl.effectiveCnf)
		if err != nil {
			return nil, err
		}
	}

	cfg := &protobufs.EffectiveConfig{
		ConfigMap: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: content},
			},
		},
	}

	return cfg, nil
}

// TODO revioew error control
func (cl *Client) onRemoteConfigReceived(remoteConf *protobufs.AgentRemoteConfig) error {
	cl.logger.Infof("remote config received")
	configChanged, err := cl.applyRemoteConfig(remoteConf)
	cl.logger.Infof("config changed: %v", configChanged)

	if configChanged {

		if err != nil {
			errRemoteConf := cl.opamClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
				LastRemoteConfigHash: remoteConf.ConfigHash,
				Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
				ErrorMessage:         err.Error(),
			})
			if errRemoteConf != nil {
				return err
			}
			return err
		}

		err = cl.opamClient.SetRemoteConfigStatus(&protobufs.RemoteConfigStatus{
			LastRemoteConfigHash: remoteConf.ConfigHash,
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		})
		if err != nil {
			return err
		}

		err = cl.opamClient.UpdateEffectiveConfig(cl.ctx)
		if err != nil {
			return err
		}
		cl.logger.Infof("agent config successfully updated")
		cl.agentChangedConfigCh <- struct{}{}
	}

	return nil
}

func (cl *Client) onMessage(ctx context.Context, msg *types.MessageData) {
	cl.logger.Infof("MESSAGE RECEIVED")

	cl.messageLock.Lock()
	defer cl.messageLock.Unlock()

	if msg.RemoteConfig != nil {
		err := cl.onRemoteConfigReceived(msg.RemoteConfig)
		if err != nil {
			cl.logger.WithError(err).Error("error onRemoteConfigReceived")
		}
	}

	if msg.OwnMetricsConnSettings != nil {
		cl.logger.Infof("msg.OwnMetricsConnSettings")
	}

	if msg.AgentIdentification != nil {
		newInstanceId, err := ulid.Parse(msg.AgentIdentification.NewInstanceUid)
		if err != nil {
			cl.logger.Errorf(err.Error())
		}
		cl.logger.Infof(fmt.Sprintf("AgentIdentification : %s", newInstanceId))
	}
}

func (cl *Client) applyRemoteConfig(config *protobufs.AgentRemoteConfig) (bool, error) {
	if config == nil {
		return false, nil
	}

	cl.effectiveCnfLock.Lock()
	defer cl.effectiveCnfLock.Unlock()

	var configHasChanged bool
	for name, file := range config.Config.ConfigMap {
		if name == "" {
			// skip instance config
			continue
		}
		if isAgentConf(name) {
			agentConfigHasChanged, err := cl.applyAgentConfig(file)
			if agentConfigHasChanged {
				cl.logger.Infof("Agent configuration changed")
			}
			if err != nil {
				return false, err
			}
			configHasChanged = configHasChanged || agentConfigHasChanged
		}
		// Integration POC for demo
		if isIntegration(name) {
			// do not restart agent as integrations have hot reloading
			intChanged, err := cl.applyIntegrationConfig(name, file)
			if intChanged {
				cl.logger.Infof("Integration changed: %s", name)
			}
			if err != nil {
				return false, err
			}
			configHasChanged = configHasChanged || intChanged
		}
	}

	return configHasChanged, nil
}

func (cl *Client) applyAgentConfig(file *protobufs.AgentConfigFile) (bool, error) {
	// if server config is empty and effectiveConfig was not, delete effective config and restart
	if strings.TrimSpace(string(file.Body)) == "" {
		if cl.effectiveCnfExists {
			err := cl.removeExistingConfig()
			if err != nil {
				return false, err
			}
			return true, nil
		} else {
			return false, nil
		}
	}

	// copy conf to temp file and load config to test that is ok
	tempFile, err := ioutil.TempFile("/tmp", "effective_config_temp_")
	if err != nil {
		return false, err
	}

	// defer os.Remove(tempFile.Name())
	err = os.WriteFile(tempFile.Name(), file.Body, 0o644)
	if err != nil {
		return false, err
	}

	tempEffectiveCnf, err := config.LoadConfig(tempFile.Name())
	if err != nil {
		return false, err
	}
	// if config has not changed early return
	if equalConfs(tempEffectiveCnf, cl.effectiveCnf) {
		return false, nil
	}

	err = os.WriteFile(effectiveConfigDefault, file.Body, 0o644)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (cl *Client) applyIntegrationConfig(name string, file *protobufs.AgentConfigFile) (bool, error) {
	intPath := "/etc/newrelic-infra/integrations.d"
	confPath := path.Join(intPath, name) + "-conf.yaml"
	// if server config is empty delete the file if it exists
	if strings.TrimSpace(string(file.Body)) == "" && fileExists(confPath) {
		return true, os.Remove(confPath)
	}
	if strings.TrimSpace(string(file.Body)) == "" && !fileExists(confPath) {
		return false, nil
	}
	if !fileExists(confPath) {
		return true, os.WriteFile(confPath, file.Body, 0o644)
	}

	existingFileConf, err := yamlFileToMap(confPath)
	if err != nil {
		return false, err
	}
	newConf, err := yamlContentToMap(file.Body)
	if err != nil {
		return false, err
	}
	if confToStr(existingFileConf) == confToStr(newConf) {
		return false, nil
	}

	return true, os.WriteFile(confPath, file.Body, 0o644)
}

func yamlContentToMap(content []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func yamlFileToMap(filePath string) (map[string]interface{}, error) {
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return yamlContentToMap(yamlFile)
}

func (cl *Client) removeExistingConfig() error {
	return os.Remove(effectiveConfigDefault)
}

func isAgentConf(name string) bool {
	return strings.HasPrefix(name, "newrelic-infra_")
}

func isIntegration(name string) bool {
	return strings.HasPrefix(name, "integration_")
}
