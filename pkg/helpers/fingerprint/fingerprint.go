// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fingerprint

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
	gopsutilnet "github.com/shirou/gopsutil/net"
)

// Harvester harvest agent's fingerprint.
type Harvester interface {
	Harvest() (fp Fingerprint, err error)
}

type harvestor struct {
	config         *config.Config // Agent configuration.
	resolver       hostname.Resolver
	cloudHarvester cloud.Harvester
}

// MockHarvestor Harvester mock
type MockHarvestor struct{}

// Harvest retrieves agent fingerprint.
func (ir *MockHarvestor) Harvest() (Fingerprint, error) {
	return Fingerprint{
		FullHostname:    "test1.newrelic.com",
		Hostname:        "test1",
		CloudProviderId: "1234abc",
		DisplayName:     "foobar",
		BootID:          "qwerty1234",
		IpAddresses:     map[string][]string{},
		MacAddresses:    map[string][]string{},
	}, nil
}

var hlog = log.WithComponent("Harvestor")

// NewHarvestor creates a new Harvester.
func NewHarvestor(config *config.Config, hostnameResolver hostname.Resolver, cloudHarvester cloud.Harvester) (Harvester, error) {
	if config == nil {
		return nil, fmt.Errorf("cannot initialize System Information service: invalid configuration")
	}

	return &harvestor{
		config:         config,
		resolver:       hostnameResolver,
		cloudHarvester: cloudHarvester,
	}, nil
}

// Fingerprint is used in the agent connect step when communicating with the backend. Based on it
// the backend will uniquely identify the agent and respond with the entityKey and entityId.
type Fingerprint struct {
	FullHostname    string    `json:"fullHostname"`
	Hostname        string    `json:"hostname"`
	CloudProviderId string    `json:"cloudProviderId"`
	DisplayName     string    `json:"displayName"`
	BootID          string    `json:"bootId"`
	IpAddresses     Addresses `json:"ipAddresses"`
	MacAddresses    Addresses `json:"macAddresses"`
}

// Addresses will store the nic addresses mapped by the nickname.
type Addresses map[string][]string

// Equals check to see if current fingerprint is equal to provided one.
func (f *Fingerprint) Equals(new Fingerprint) bool {
	return f.Hostname == new.Hostname &&
		f.FullHostname == new.FullHostname &&
		f.CloudProviderId == new.CloudProviderId &&
		f.BootID == new.BootID &&
		f.DisplayName == new.DisplayName &&
		f.IpAddresses.Equals(new.IpAddresses) &&
		f.MacAddresses.Equals(new.MacAddresses)
}

// Equals check if the Address has the same values.
func (a Addresses) Equals(b Addresses) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for keyA, valA := range a {
		valB, exists := b[keyA]
		if !exists {
			return false
		}
		if len(valA) != len(valB) {
			return false
		}

		for i := range valA {
			if valA[i] != valB[i] {
				return false
			}
		}
	}
	return true
}

// Harvest will return the system fingerprint.
func (ir *harvestor) Harvest() (fp Fingerprint, err error) {
	fullHostname, shortHostname, err := ir.resolver.Query()
	if err != nil {
		return
	}

	// Network interfaces information.
	ipAddresses, macAddresses, err := ir.getNetworkInfo()
	if err != nil {
		return
	}

	// Get ec2 instance id.
	instanceID, err := ir.cloudHarvester.GetInstanceID()
	if err != nil {
		// if we know for sure that the host runs on the cloud, do not generate a fingerprint without a cloud id
		// because then the host could start reporting under a different entity id.
		if ir.cloudHarvester.GetCloudType().IsValidCloud() {
			return
		}

		hlog.WithError(err).Error("Unable to get instance id.")
	}

	return Fingerprint{
		FullHostname:    fullHostname,
		Hostname:        shortHostname,
		DisplayName:     ir.config.DisplayName,
		BootID:          GetBootId(),
		IpAddresses:     ipAddresses,
		CloudProviderId: instanceID,
		MacAddresses:    macAddresses,
	}, nil
}

// getNetworkInfo will return ipAddresses and macAddresses mapped by nickname.
func (ir *harvestor) getNetworkInfo() (ipAddresses, macAddresses map[string][]string, err error) {

	// Check if ip data collection is disabled from the configuration.
	if !ir.config.IpData {
		return nil, nil, nil
	}

	var niList []gopsutilnet.InterfaceStat
	niList, err = gopsutilnet.Interfaces()
	if err != nil {
		return
	}

	ipAddresses = make(map[string][]string)
	macAddresses = make(map[string][]string)

	for _, ni := range niList {
		if network_helpers.ShouldIgnoreInterface(ir.config.NetworkInterfaceFilters, ni.Name) {
			continue
		}

		ipv4, ipv6 := network_helpers.IPAddressesByType(ni.Addrs)

		if ipv4 != "" {
			ipAddresses[ni.Name] = append(ipAddresses[ni.Name], ipv4)
		}
		if ipv6 != "" {
			ipAddresses[ni.Name] = append(ipAddresses[ni.Name], ipv6)
		}
		if ni.HardwareAddr != "" {
			macAddresses[ni.Name] = append(macAddresses[ni.Name], ni.HardwareAddr)
		}
	}
	return
}
