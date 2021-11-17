package service

import (
	"encoding/json"
	"fmt"
	"provision-alerts/config"
	"provision-alerts/infrastructure"
	"strconv"
)

const postPolicyPath = "/v2/alerts_policies.json"
const delPolicyPath = "/v2/alerts_policies/%d.json"
const postConditionPath = "/v2/alerts_nrql_conditions/policies/%d.json" // %s = policyId
const putChannelPath = "/v2/alerts_policy_channels.json"

type Policy struct {
	Id                 int    `json:"id"`
	Name               string `json:"name"`
	IncidentPreference string `json:"incident_preference"`

	Conditions Conditions
	Channels   []int
}

type Condition struct {
	Name      string
	Duration  int
	Threshold float64
	Operator  string
	NRQL      string
}

type Conditions []Condition

// interfaces

type ConditionProvider interface {
	Provides() Conditions
}

type PolicyService interface {
	Create(policy config.PolicyConfig) (Policy, error)
	AddCondition(policy Policy, condition config.ConditionConfig) (Policy, error)
	AddChannel(policy Policy, channelId int) (Policy, error)
	Delete(id int) error
	DeleteAll() error
}

// services

func NewPolicyApiService(client AlertClient) PolicyService {
	return &PolicyApiService{
		client: client,
	}
}

type PolicyApiService struct {
	client AlertClient
}

func (pas *PolicyApiService) Create(policyConfig config.PolicyConfig) (Policy, error) {

	jsonPayload, err := json.Marshal(infrastructure.FromPolicyConfig(policyConfig))
	if err != nil {
		return Policy{}, err
	}

	jsonResponse, err := pas.client.Post(postPolicyPath, jsonPayload)
	if err != nil {
		return Policy{}, err
	}

	var policyResponse infrastructure.PolicyResponse
	err = json.Unmarshal(jsonResponse, &policyResponse)
	if err != nil {
		return Policy{}, err
	}

	return Policy{
		Id:                 policyResponse.Policy.Id,
		Name:               policyResponse.Policy.Name,
		IncidentPreference: policyResponse.Policy.IncidentPreference,
	}, nil

}

func (pas *PolicyApiService) AddCondition(policy Policy, condition config.ConditionConfig) (Policy, error) {

	jsonPayload, err := json.Marshal(infrastructure.FromConditionConfig(condition))
	if err != nil {
		return Policy{}, err
	}

	jsonResponse, err := pas.client.Post(fmt.Sprintf(postConditionPath, policy.Id), jsonPayload)
	if err != nil {
		return Policy{}, err
	}

	var policyResponse infrastructure.NRQLConditionResponse
	err = json.Unmarshal(jsonResponse, &policyResponse)
	if err != nil {
		return Policy{}, err
	}

	duration, err := strconv.ParseInt(policyResponse.NRQLConditionDetailsResponse.Terms[0].Duration, 10, 64)
	if err != nil {
		return Policy{}, err
	}
	threshold, err := strconv.ParseFloat(policyResponse.NRQLConditionDetailsResponse.Terms[0].Threshold, 64)
	if err != nil {
		return Policy{}, err
	}
	policy.Conditions = append(policy.Conditions, Condition{
		Name:      policyResponse.NRQLConditionDetailsResponse.Name,
		Duration:  int(duration),
		Threshold: threshold,
		Operator:  policyResponse.NRQLConditionDetailsResponse.Terms[0].Operator,
		NRQL:      policyResponse.NRQLConditionDetailsResponse.Nrql.Query,
	})

	return policy, nil
}

func (pas *PolicyApiService) AddChannel(policy Policy, channelId int) (Policy, error) {
	rawPayload := fmt.Sprintf("policy_id=%d&channel_ids=%d", policy.Id, channelId)

	_, err := pas.client.Put(putChannelPath+"?"+rawPayload, nil)
	if err != nil {
		return Policy{}, err
	}
	policy.Channels = append(policy.Channels, channelId)

	return policy, nil
}

func (pas *PolicyApiService) Delete(policyId int) error {
	_, err := pas.client.Del(fmt.Sprintf(delPolicyPath, policyId), nil)
	return err
}

func (pas *PolicyApiService) DeleteAll() error {
	panic("implement me")
}
