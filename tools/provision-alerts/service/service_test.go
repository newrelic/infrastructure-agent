package service

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"provision-alerts/config"
	"testing"
)

func TestPolicyApiService_Create(t *testing.T) {

	client := &AlertClientMock{}

	s := NewPolicyApiService(client)

	pc := config.PolicyConfig{
		Name:               "testing",
		IncidentPreference: "whatever",
	}
	expectedPolicy := Policy{
		Id:                 454545,
		Name:               "testing",
		IncidentPreference: "whatever",
	}

	post := []byte(`{"policy":{"name":"testing","incident_preference":"whatever"}}`)

	response := []byte(`{
 "policy": {
   "id": 454545,
   "incident_preference": "whatever",
   "name": "testing",
   "created_at": 1634302571179,
   "updated_at": 1634302571179
 }
}`)

	client.ShouldPost("/v2/alerts_policies.json", post, response, nil)

	actualPolicy, err := s.Create(pc)
	assert.NoError(t, err)
	assert.Equal(t, expectedPolicy, actualPolicy)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, client)
}

func TestPolicyApiService_AddCondition(t *testing.T) {

	client := &AlertClientMock{}

	s := NewPolicyApiService(client)

	policy := Policy{
		Id:                 454545,
		Name:               "testing",
		IncidentPreference: "whatever",
	}

	pcc := config.ConditionConfig{
		Name:      "test condition",
		Duration:  10,
		Threshold: 5,
		Operator:  "above",
		NRQL:      "SELECT * FROM *",
	}

	expectedPolicy := Policy{
		Id:                 policy.Id,
		Name:               policy.Name,
		IncidentPreference: policy.IncidentPreference,
		Conditions: Conditions{
			{
				Name:      pcc.Name,
				Duration:  pcc.Duration,
				Operator:  "above",
				Threshold: 5,
				NRQL:      pcc.NRQL,
			},
		},
		Channels: nil,
	}

	post := []byte(`{"nrql_condition":{"type":"static","name":"test condition","enabled":true,"value_function":"single_value","violation_time_limit_seconds":259200,"terms":[{"duration":"10","operator":"above","threshold":"5.0","time_function":"all","priority":"critical"}],"nrql":{"query":"SELECT * FROM *"},"signal":{"aggregation_window":"60","aggregation_method":"EVENT_FLOW","aggregation_delay":120,"fill_option":"none"}}}`)

	response := []byte(`{
  "nrql_condition": {
    "id": 684668,
    "type": "static",
    "name": "test condition",
    "enabled": true,
    "value_function": "single_value",
    "violation_time_limit_seconds": 259200,
    "terms": [
      {
        "duration": "10",
        "operator": "above",
        "threshold": "5.0",
        "time_function": "all",
        "priority": "critical"
      }
    ],
    "nrql": {
      "query": "SELECT * FROM *"
    },
    "signal": {
      "aggregation_window": "60",
      "aggregation_method": "EVENT_FLOW",
      "aggregation_delay": 120,
      "fill_option": "none"
    }
  }
}`)

	client.ShouldPost(fmt.Sprintf("/v2/alerts_nrql_conditions/policies/%d.json", policy.Id), post, response, nil)

	actualPolicy, err := s.AddCondition(policy, pcc)

	assert.NoError(t, err)
	assert.Equal(t, expectedPolicy, actualPolicy)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, client)
}

func TestPolicyApiService_AddChannel(t *testing.T) {

	client := &AlertClientMock{}

	s := NewPolicyApiService(client)

	policy := Policy{
		Id:                 454545,
		Name:               "testing",
		IncidentPreference: "whatever",
	}

	channelId := 123

	expectedPolicy := Policy{
		Id:                 policy.Id,
		Name:               policy.Name,
		IncidentPreference: policy.IncidentPreference,
		Conditions:         nil,
		Channels:           []int{channelId},
	}

	put := fmt.Sprintf("policy_id=%d&channel_ids=%d", policy.Id, channelId)

	response := []byte(fmt.Sprintf(`{"policy":{"id":%d,"channel_ids":[%d]}}`, policy.Id, channelId))

	client.ShouldPut("/v2/alerts_policy_channels.json?"+put, nil, response, nil)

	actualPolicy, err := s.AddChannel(policy, channelId)

	assert.NoError(t, err)
	assert.Equal(t, expectedPolicy, actualPolicy)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, client)
}

func TestPolicyApiService_Delete(t *testing.T) {

	client := &AlertClientMock{}

	s := NewPolicyApiService(client)

	policy := Policy{
		Id:                 454545,
		Name:               "testing",
		IncidentPreference: "whatever",
	}

	id := 123

	response := []byte(fmt.Sprintf(
		`{"policy": {"id": %d,"incident_preference": "%s","name": "%s","created_at": 1636566135672,"updated_at": 1636566135672}}`,
		policy.Id,
		policy.IncidentPreference,
		policy.Name,
	))

	client.ShouldDel(fmt.Sprintf("/v2/alerts_policies/%d.json", id), nil, response, nil)

	err := s.Delete(id)

	assert.NoError(t, err)

	//mocked objects assertions
	mock.AssertExpectationsForObjects(t, client)
}
