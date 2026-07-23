// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ErrOCIAPIUnavailable indicates the OCI SDK API clients could not be initialized, typically
// because instance principal authentication is unavailable (no IAM dynamic-group policy, or
// running outside OCI). Phase 2 attributes are left empty when this occurs - it is never fatal.
var ErrOCIAPIUnavailable = errors.New("OCI API unavailable")

// newInstancePrincipalProvider builds the instance principal configuration provider. Var to
// allow test overrides.
var newInstancePrincipalProvider = auth.InstancePrincipalConfigurationProvider //nolint:gochecknoglobals

// initAPIClients builds the OCI SDK Compute and VirtualNetwork clients via instance principal
// authentication, once per harvester lifetime. Every Phase 2 getter calls this first.
func (a *OCIHarvester) initAPIClients() error {
	a.apiOnce.Do(func() {
		provider, err := newInstancePrincipalProvider()
		if err != nil {
			a.apiInitErr = fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
			return
		}

		computeClient, err := core.NewComputeClientWithConfigurationProvider(provider)
		if err != nil {
			a.apiInitErr = fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
			return
		}

		vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(provider)
		if err != nil {
			a.apiInitErr = fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
			return
		}

		a.computeClient = &computeClient
		a.vnClient = &vnClient
	})

	return a.apiInitErr
}

// getInstanceDetails fetches (and caches) the OCI Compute instance details via the SDK.
func (a *OCIHarvester) getInstanceDetails() (*core.Instance, error) {
	if a.instanceDetails != nil && !a.timeout.HasExpired() {
		return a.instanceDetails, nil
	}

	if err := a.initAPIClients(); err != nil {
		return nil, err
	}

	instanceID, err := a.GetInstanceID()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
	}

	response, err := a.computeClient.GetInstance(context.Background(), core.GetInstanceRequest{
		InstanceId: &instanceID,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
	}

	a.instanceDetails = &response.Instance

	return a.instanceDetails, nil
}

// getPrimaryVnic fetches (and caches) the OCI VNIC details for the primary VNIC via the SDK.
func (a *OCIHarvester) getPrimaryVnic() (*core.Vnic, error) {
	if a.vnic != nil && !a.timeout.HasExpired() {
		return a.vnic, nil
	}

	if err := a.initAPIClients(); err != nil {
		return nil, err
	}

	vnics, err := GetOCIVnicsMetadata(a.disableKeepAlive)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
	}
	if len(vnics) == 0 {
		return nil, fmt.Errorf("%w: no VNICs found in IMDS", ErrOCIAPIUnavailable)
	}

	response, err := a.vnClient.GetVnic(context.Background(), core.GetVnicRequest{
		VnicId: &vnics[0].VnicID,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
	}

	a.vnic = &response.Vnic

	return a.vnic, nil
}

// getSubnet fetches (and caches) the OCI subnet details for the primary VNIC's subnet.
func (a *OCIHarvester) getSubnet() (*core.Subnet, error) {
	if a.subnet != nil && !a.timeout.HasExpired() {
		return a.subnet, nil
	}

	vnic, err := a.getPrimaryVnic()
	if err != nil {
		return nil, err
	}
	if vnic.SubnetId == nil {
		return nil, fmt.Errorf("%w: primary VNIC has no subnet ID", ErrOCIAPIUnavailable)
	}

	response, err := a.vnClient.GetSubnet(context.Background(), core.GetSubnetRequest{
		SubnetId: vnic.SubnetId,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOCIAPIUnavailable, err)
	}

	a.subnet = &response.Subnet

	return a.subnet, nil
}

// GetVCNID returns the OCID of the VCN the instance's primary VNIC is in.
func (a *OCIHarvester) GetVCNID() (string, error) {
	subnet, err := a.getSubnet()
	if err != nil {
		return "", err
	}
	if subnet.VcnId == nil {
		return "", nil
	}

	return *subnet.VcnId, nil
}

// GetSubnetID returns the OCID of the subnet the instance's primary VNIC is in.
func (a *OCIHarvester) GetSubnetID() (string, error) {
	vnic, err := a.getPrimaryVnic()
	if err != nil {
		return "", err
	}
	if vnic.SubnetId == nil {
		return "", nil
	}

	return *vnic.SubnetId, nil
}

// GetLifecycleState returns the instance's current lifecycle state (e.g. RUNNING, STOPPED).
func (a *OCIHarvester) GetLifecycleState() (string, error) {
	instance, err := a.getInstanceDetails()
	if err != nil {
		return "", err
	}

	return string(instance.LifecycleState), nil
}

// GetVirtualizationType returns the instance's launch mode (NATIVE, EMULATED, PARAVIRTUALIZED,
// ACCELERATEDPV, or CUSTOM) - the closest OCI equivalent to AWS's virtualization type.
func (a *OCIHarvester) GetVirtualizationType() (string, error) {
	instance, err := a.getInstanceDetails()
	if err != nil {
		return "", err
	}

	return string(instance.LaunchMode), nil
}

// GetDedicatedVMHostID returns the OCID of the dedicated VM host the instance runs on, or an
// empty string if the instance is not on a dedicated host.
func (a *OCIHarvester) GetDedicatedVMHostID() (string, error) {
	instance, err := a.getInstanceDetails()
	if err != nil {
		return "", err
	}
	if instance.DedicatedVmHostId == nil {
		return "", nil
	}

	return *instance.DedicatedVmHostId, nil
}
