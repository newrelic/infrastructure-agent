// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fargate

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type HTTPClientMock struct {
	mock.Mock
}

func (s *HTTPClientMock) Get(url string) (*http.Response, error) {
	args := s.Called(url)

	return args.Get(0).(*http.Response), args.Error(1) //nolint
}

func (s *HTTPClientMock) ShouldReturnResponse(url string, resp *http.Response) {
	s.
		On("Get", url).
		Once().
		Return(resp, nil)
}

func (s *HTTPClientMock) ShouldReturnError(url string, err error) {
	s.
		On("Get", url).
		Once().
		Return(&http.Response{}, err)
}

func TestAwsMetadata_ErrorOnApiCall(t *testing.T) {
	httpClientMock := &HTTPClientMock{}
	httpClient = httpClientMock

	err := errors.New("some error") //nolint
	httpClientMock.ShouldReturnError("http://169.254.170.2/v2/metadata", err)

	taskMetadata, err := awsMetadata()
	assert.Nil(t, taskMetadata)
	assert.Equal(t, err, err)
	httpClientMock.AssertExpectations(t)
}

func TestAwsMetadata_ErrorHttp(t *testing.T) {
	httpClientMock := &HTTPClientMock{}
	httpClient = httpClientMock

	statusCode := http.StatusBadRequest
	status := "400 Bad Requesy"
	expectedErr := fmt.Errorf("server responded %v - %v", statusCode, status) //nolint

	body := io.NopCloser(strings.NewReader(fargateOutput2Containers))
	response := http.Response{
		Body:       body,
		StatusCode: http.StatusBadRequest,
		Status:     status,
	}
	httpClientMock.ShouldReturnResponse("http://169.254.170.2/v2/metadata", &response)

	taskMetadata, err := awsMetadata()
	assert.Nil(t, taskMetadata)
	assert.Equal(t, expectedErr, err)
	httpClientMock.AssertExpectations(t)
}

func TestAwsMetadata_MultipleContainers(t *testing.T) {
	httpClientMock := &HTTPClientMock{}
	httpClient = httpClientMock

	expectedTaskMetadata := fargateMetadataTwoContainers

	body := io.NopCloser(strings.NewReader(fargateOutput2Containers))
	response := http.Response{
		Body:       body,
		StatusCode: http.StatusOK,
	}
	httpClientMock.ShouldReturnResponse("http://169.254.170.2/v2/metadata", &response)

	taskMetadata, err := awsMetadata()
	require.NoError(t, err)

	assert.Equal(t, &expectedTaskMetadata, taskMetadata)
	httpClientMock.AssertExpectations(t)
}

func TestMatch(t *testing.T) {
	matchRules := map[string]string{
		"name": "mysql",
	}
	matcher, err := discovery.NewMatcher(matchRules)
	require.NoError(t, err)

	taskMetadata := fargateMetadataTwoContainers

	expectedDiscovery := []discovery.Discovery{
		{
			Variables: data.Map{
				"discovery.private.port":                                    "3306",
				"discovery.private.ports.tcp.0":                             "3306",
				"discovery.private.ports.0":                                 "3306",
				"discovery.private.ports.tcp":                               "3306",
				"discovery.label.com.amazonaws.ecs.task-definition-family":  "family-fargate-test",
				"discovery.ip":                                              "10.10.1.236",
				"discovery.private.ip":                                      "10.10.1.236",
				"discovery.ip.0":                                            "10.10.1.236",
				"discovery.label.my_custom_label":                           "{\"this\":\"is json\"}",
				"discovery.private.ip.0":                                    "10.10.1.236",
				"discovery.label.com.amazonaws.ecs.task-definition-version": "4",
				"discovery.name":                                            "mysql",
				"discovery.label.com.amazonaws.ecs.container-name":          "mysql",
				"discovery.label.com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
				"discovery.label.com.amazonaws.ecs.cluster":                 "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
				"discovery.containerId":                                     "28e5e82b4603401ca54987ad7fc7b4d1-1785357245",
				"discovery.image":                                           "mysql:latest",
			},
			MetricAnnotations: data.InterfaceMap{
				"image":         "mysql:latest",
				"imageId":       "sha256:444f037733d01fc3dfc691a9ab05e346629e8e4d3a6c75da864f21421fb38ced",
				"containerName": "mysql",
				"containerId":   "28e5e82b4603401ca54987ad7fc7b4d1-1785357245",
				"label": map[string]string{
					"com.amazonaws.ecs.cluster":                 "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
					"com.amazonaws.ecs.container-name":          "mysql",
					"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
					"com.amazonaws.ecs.task-definition-family":  "family-fargate-test",
					"com.amazonaws.ecs.task-definition-version": "4",
					"my_custom_label":                           "{\"this\":\"is json\"}",
				},
				"dockerContainerName": "mysql",
			},
			EntityRewrites: []data.EntityRewrite{
				{
					Action:       "replace",
					Match:        "${ip}",
					ReplaceField: "container:${containerId}",
				},
			},
		},
	}
	disc, err := match(&taskMetadata, &matcher)
	require.NoError(t, err)
	assert.Equal(t, expectedDiscovery, disc)
}

func mustParse(layout string, value string) *time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}

	return &t
}

var (
	//nolint
	fargateOutput2Containers = `{
  "Cluster": "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
  "TaskARN": "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
  "Family": "family-fargate-test",
  "Revision": "4",
  "DesiredStatus": "RUNNING",
  "KnownStatus": "SNAPSHOTTER_SELECTED",
  "Containers": [
    {
      "DockerId": "28e5e82b4603401ca54987ad7fc7b4d1-1785357245",
      "Name": "mysql",
      "DockerName": "mysql",
      "Image": "mysql:latest",
      "ImageID": "sha256:444f037733d01fc3dfc691a9ab05e346629e8e4d3a6c75da864f21421fb38ced",
      "Labels": {
        "com.amazonaws.ecs.cluster": "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
        "com.amazonaws.ecs.container-name": "mysql",
        "com.amazonaws.ecs.task-arn": "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
        "com.amazonaws.ecs.task-definition-family": "family-fargate-test",
        "com.amazonaws.ecs.task-definition-version": "4",
        "my_custom_label": "{\"this\":\"is json\"}"
      },
      "DesiredStatus": "RUNNING",
      "KnownStatus": "RUNNING",
      "Limits": {
        "CPU": 2
      },
      "CreatedAt": "2022-07-08T21:38:33.101192734Z",
      "StartedAt": "2022-07-08T21:38:33.101192734Z",
      "Type": "NORMAL",
      "Networks": [
        {
          "NetworkMode": "awsvpc",
          "IPv4Addresses": [
            "10.10.1.236"
          ]
        }
      ],
      "Ports": [
		{
			"ContainerPort": 3306,
			"Protocol": "tcp"
		}
	  ]
    },
    {
      "DockerId": "28e5e82b4603401ca54987ad7fc7b4d1-3894802353",
      "Name": "container-name",
      "DockerName": "container-name",
      "Image": "ghcr.io/owner/image-name:latest",
      "ImageID": "sha256:badea1d454df2e07c870740556bdf15608a9fef56b8456d51f9d59e13b60bb31",
      "Labels": {
        "com.amazonaws.ecs.cluster": "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
        "com.amazonaws.ecs.container-name": "container-name",
        "com.amazonaws.ecs.task-arn": "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
        "com.amazonaws.ecs.task-definition-family": "family-fargate-test",
        "com.amazonaws.ecs.task-definition-version": "4"
      },
      "DesiredStatus": "RUNNING",
      "KnownStatus": "PULLED",
      "ExitCode": 0,
      "Limits": {
        "CPU": 0
      },
      "Type": "NORMAL",
      "Networks": [
        {
          "NetworkMode": "awsvpc",
          "IPv4Addresses": [
            "10.10.1.236"
          ]
        }
      ]
    }
  ],
  "Limits": {
    "CPU": 0.5,
    "Memory": 1024
  },
  "PullStartedAt": "2022-07-08T21:37:58.023738502Z",
  "PullStoppedAt": "2022-07-08T21:38:23.526918729Z",
  "AvailabilityZone": "us-east-2a"
}`

	//nolint
	fargateMetadataTwoContainers = TaskMetadata{
		Cluster:       "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
		TaskArn:       "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
		Family:        "family-fargate-test",
		Revision:      "4",
		DesiredStatus: "RUNNING",
		KnownStatus:   "SNAPSHOTTER_SELECTED",
		Containers: []awsContainer{
			{
				DockerID:   "28e5e82b4603401ca54987ad7fc7b4d1-1785357245",
				Name:       "mysql",
				DockerName: "mysql",
				Image:      "mysql:latest",
				ImageID:    "sha256:444f037733d01fc3dfc691a9ab05e346629e8e4d3a6c75da864f21421fb38ced",
				Labels: map[string]string{
					"com.amazonaws.ecs.cluster":                 "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
					"com.amazonaws.ecs.container-name":          "mysql",
					"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
					"com.amazonaws.ecs.task-definition-family":  "family-fargate-test",
					"com.amazonaws.ecs.task-definition-version": "4",
					"my_custom_label":                           "{\"this\":\"is json\"}",
				},
				Limits: map[string]uint64{
					"CPU": 2,
				},
				DesiredStatus: "RUNNING",
				KnownStatus:   "RUNNING",
				ExitCode:      0,
				CreatedAt:     "2022-07-08T21:38:33.101192734Z",
				StartedAt:     "2022-07-08T21:38:33.101192734Z",
				FinishedAt:    "",
				Type:          "NORMAL",
				Networks: []Network{
					{
						NetworkMode: "awsvpc",
						IPv4Addresses: []string{
							"10.10.1.236",
						},
					},
				},
				Ports: []PortResponse{
					{
						ContainerPort: 3306,
						Protocol:      "tcp",
					},
				},
			},
			{
				DockerID:   "28e5e82b4603401ca54987ad7fc7b4d1-3894802353",
				Name:       "container-name",
				DockerName: "container-name",
				Image:      "ghcr.io/owner/image-name:latest",
				ImageID:    "sha256:badea1d454df2e07c870740556bdf15608a9fef56b8456d51f9d59e13b60bb31",
				Labels: map[string]string{
					"com.amazonaws.ecs.cluster":                 "arn:aws:ecs:us-east-2:000000000000:cluster/test-cluster",
					"com.amazonaws.ecs.container-name":          "container-name",
					"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-east-2:000000000000:task/test-cluster/28e5e82b4603401ca54987ad7fc7b4d1",
					"com.amazonaws.ecs.task-definition-family":  "family-fargate-test",
					"com.amazonaws.ecs.task-definition-version": "4",
				},
				Limits: map[string]uint64{
					"CPU": 0,
				},
				DesiredStatus: "RUNNING",
				KnownStatus:   "PULLED",
				Type:          "NORMAL",
				Networks: []Network{
					{
						NetworkMode: "awsvpc",
						IPv4Addresses: []string{
							"10.10.1.236",
						},
					},
				},
			},
		},
		Limits: map[string]float64{
			"CPU":    0.5,
			"Memory": 1024,
		},
		PullStartedAt:      mustParse(time.RFC3339Nano, "2022-07-08T21:37:58.023738502Z"),
		PullStoppedAt:      mustParse(time.RFC3339Nano, "2022-07-08T21:38:23.526918729Z"),
		ExecutionStoppedAt: nil,
	}
)
