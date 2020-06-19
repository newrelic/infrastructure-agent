// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/stretchr/testify/mock"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/shirou/gopsutil/net"
	"github.com/stretchr/testify/assert"
)

const (
	agentId       = "FakeAgent"
	testFrequency = int64(45)
)

func getPluginId() ids.PluginID {
	return ids.PluginID{"system", "network_interfaces"}
}

func getTestFilters() map[string][]string {
	return map[string][]string{
		"prefix":  {"lo", "offset"},
		"index-1": {"middle", "face"},
	}
}

func getTestConfig() *config.Config {
	return &config.Config{
		NetworkInterfaceFilters:     getTestFilters(),
		NetworkInterfaceIntervalSec: testFrequency,
	}
}
func getTestInterfaces() []net.InterfaceStat {
	return []net.InterfaceStat{
		{
			MTU:          1024,
			Name:         "eth0",
			HardwareAddr: "00:1b:63:84:45:e6",
			Flags:        []string{"flag1", "flag2"},
			Addrs:        []net.InterfaceAddr{{Addr: "127.1.2.3"}, {Addr: "172.16.13.37"}},
		},
		{
			MTU:          1500,
			Name:         "lo0",
			HardwareAddr: "00:2c:74:95:56:f7",
			Flags:        []string{"flag3", "flag4"},
			Addrs:        []net.InterfaceAddr{{Addr: "::ffff:0:0:0"}, {Addr: "64:ff9b::"}},
		},
	}
}

func isIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func isIPv6(address string) bool {
	return strings.Count(address, ":") >= 2
}

func ipAddressesByType(addrs []net.InterfaceAddr) (ipv4, ipv6 string) {
	for _, ia := range addrs {
		if isIPv6(ia.Addr) {
			ipv6 = ia.Addr
		} else if isIPv4(ia.Addr) {
			ipv4 = ia.Addr
		}
	}
	return
}

func interfaceStatAsNetworkInterfaceData(input *net.InterfaceStat) NetworkInterfaceData {
	ipv4, ipv6 := ipAddressesByType(input.Addrs)

	return NetworkInterfaceData{
		InterfaceName:   input.Name,
		HardwareAddress: input.HardwareAddr,
		IpV4Address:     ipv4,
		IpV6Address:     ipv6,
	}
}

func getInterfaces() ([]net.InterfaceStat, error) {
	return getTestInterfaces(), nil
}

func TestSortKey(t *testing.T) {
	nid := interfaceStatAsNetworkInterfaceData(&getTestInterfaces()[0])
	assert.Equal(t, nid.SortKey(), getTestInterfaces()[0].Name)
}

func TestNewNetworkInterfacePlugin(t *testing.T) {
	ctx := &mocks.AgentContext{}
	ctx.On("Config").Return(getTestConfig())

	plugin := NewNetworkInterfacePlugin(getPluginId(), ctx)
	assert.NotNil(t, plugin)
	assert.Equal(t, plugin.frequency, time.Duration(testFrequency)*time.Second)
	assert.Equal(t, plugin.networkInterfaceFilters, getTestFilters())
	assert.NotNil(t, plugin.getInterfaces) // Ensure has default interfaces provider
}

func TestWithInterfacesProvider(t *testing.T) {
	ctx := &mocks.AgentContext{}
	ctx.On("Config").Return(getTestConfig())

	plugin := NewNetworkInterfacePlugin(getPluginId(), ctx)
	assert.NotNil(t, plugin)

	plugin = plugin.WithInterfacesProvider(nil)
	assert.Nil(t, plugin.getInterfaces)

	plugin = plugin.WithInterfacesProvider(network_helpers.GopsutilInterfacesProvider)
	assert.NotNil(t, plugin.getInterfaces)
}

func TestGetNetworkInterfaceData(t *testing.T) {
	ctx := &mocks.AgentContext{}
	ctx.On("Config").Return(getTestConfig())

	plugin := NewNetworkInterfacePlugin(getPluginId(), ctx)
	assert.NotNil(t, plugin)
	plugin.WithInterfacesProvider(getInterfaces)

	data, err := plugin.getNetworkInterfaceData()
	assert.NoError(t, err)

	ni := interfaceStatAsNetworkInterfaceData(&getTestInterfaces()[0])
	assert.NotNil(t, ni)
	assert.Equal(t, data, agent.PluginInventoryDataset{ni})
}

func TestNetworkPlugin(t *testing.T) {
	interfaces, err := getInterfaces()
	assert.NoError(t, err)
	assert.NotNil(t, interfaces)

	pluginInventory := agent.PluginInventoryDataset{}
	for _, ni := range interfaces {
		pluginInventory = append(pluginInventory, interfaceStatAsNetworkInterfaceData(&ni))
	}

	expectedInventory := agent.NewPluginOutput(getPluginId(), agentId, pluginInventory)
	assert.NotNil(t, expectedInventory)

	ctx := &mocks.AgentContext{}
	ctx.On("Config").Return(&config.Config{})
	ctx.On("AgentIdentifier").Return(agentId)
	ch := make(chan mock.Arguments)
	ctx.On("SendData", mock.Anything).Run(func(args mock.Arguments) {
		ch <- args
	})

	plugin := NewNetworkInterfacePlugin(getPluginId(), ctx)
	assert.NotNil(t, plugin)

	plugin.getInterfaces = getInterfaces
	go plugin.Run()

	args := <-ch
	_, ok := args[0].(agent.PluginOutput)
	assert.True(t, ok)
	actualInventory := args[0]

	assert.Equal(t, expectedInventory, actualInventory)
}
