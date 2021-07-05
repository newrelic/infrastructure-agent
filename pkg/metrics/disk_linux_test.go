// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package metrics

import (
	"reflect"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewDiskMonitor(t *testing.T) {
	m := NewDiskMonitor(nil)

	assert.NotNil(t, m)
}

func TestDiskSample(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	storage := storage.NewSampler(ctx)
	m := NewDiskMonitor(storage)

	result, err := m.Sample()

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestFilterStorageSamples(t *testing.T) {
	var inputValues = sample.EventBatch{
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/sda1",
			MountPoint: "/home/vagrant/test",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/sda2",
			MountPoint: "/home/vagrant/test2",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/sda3",
			MountPoint: "/home/vagrant/test3",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/sda1",
			MountPoint: "/home/vagrant/test1",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/sda4",
			MountPoint: "/home/vagrant/test4",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "/dev/mapper/docker-8:1-399801-e27aa58933f868f68fb7f979a396f59f07b3642aaf5f6fcffdf12ca62ba92569",
			MountPoint: "/var/lib/docker/devicemapper/mnt/e27aa58933f868f68fb7f979a396f59f07b3642aaf5f6fcffdf12ca62ba92569",
		}},
		&storage.Sample{BaseSample: storage.BaseSample{
			Device:     "test",
			MountPoint: "/var/lib/kubelet/",
		}},
	}

	var expectedValues = []*storage.Sample{
		{BaseSample: storage.BaseSample{
			Device:     "/dev/sda1",
			MountPoint: "/home/vagrant/test",
		}},
		{BaseSample: storage.BaseSample{
			Device:     "/dev/sda2",
			MountPoint: "/home/vagrant/test2",
		}},
		{BaseSample: storage.BaseSample{
			Device:     "/dev/sda3",
			MountPoint: "/home/vagrant/test3",
		}},
		{BaseSample: storage.BaseSample{
			Device:     "/dev/sda4",
			MountPoint: "/home/vagrant/test4",
		}},
	}

	resultedValues := FilterStorageSamples(inputValues)

	if !reflect.DeepEqual(expectedValues, resultedValues) {
		assert.Equal(t, resultedValues, expectedValues)
	}
}
