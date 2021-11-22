package infrastructure

import (
	"fmt"
	"provision-alerts/config"
)

type PolicyDetailsPayload struct {
	Name               string `json:"name"`
	IncidentPreference string `json:"incident_preference"`
}

type PolicyPayload struct {
	Policy PolicyDetailsPayload `json:"policy"`
}

type PolicyDetailsResponse struct {
	Id int `json:"id"`
	PolicyDetailsPayload
}

type PolicyResponse struct {
	Policy PolicyDetailsResponse `json:"policy"`
}

type PoliciesDetailsResponse []PolicyDetailsResponse
type PoliciesResponse struct {
	Policies PoliciesDetailsResponse `json:"policies"`
}

func (p *PoliciesResponse) IsEmpty() bool {
	return len(p.Policies) == 0
}

func FromPolicyConfig(pc config.PolicyConfig) PolicyPayload {
	return PolicyPayload{
		PolicyDetailsPayload{
			Name:               pc.Name,
			IncidentPreference: pc.IncidentPreference,
		},
	}
}

type NRQLConditionResponse struct {
	NRQLConditionDetailsResponse NRQLConditionDetailsResponse `json:"nrql_condition"`
}

type NRQLConditionDetailsResponse struct {
	Id int `json:"id"`
	NRQLConditionPayloadDetails
}

type NRQLConditionPayload struct {
	NrqlCondition NRQLConditionPayloadDetails `json:"nrql_condition"`
}

type NRQLConditionPayloadDetails struct {
	Type                      string     `json:"type"`
	Name                      string     `json:"name"`
	Enabled                   bool       `json:"enabled"`
	ValueFunction             string     `json:"value_function"`
	ViolationTimeLimitSeconds int        `json:"violation_time_limit_seconds"`
	Terms                     []NRQLTerm `json:"terms"`
	Nrql                      struct {
		Query string `json:"query"`
	} `json:"nrql"`
	Signal struct {
		AggregationWindow string `json:"aggregation_window"`
		AggregationMethod string `json:"aggregation_method"`
		AggregationDelay  int    `json:"aggregation_delay"`
		FillOption        string `json:"fill_option"`
	} `json:"signal"`
}

type NRQLTerm struct {
	Duration     string `json:"duration"`
	Operator     string `json:"operator"`
	Threshold    string `json:"threshold"`
	TimeFunction string `json:"time_function"`
	Priority     string `json:"priority"`
}

func FromConditionConfig(condition config.ConditionConfig) NRQLConditionPayload {

	nrqlConditionPayload := NRQLConditionPayload{}
	nrqlConditionPayload.NrqlCondition.Type = "static"
	nrqlConditionPayload.NrqlCondition.Name = condition.Name
	nrqlConditionPayload.NrqlCondition.Enabled = true
	nrqlConditionPayload.NrqlCondition.ValueFunction = "single_value"
	nrqlConditionPayload.NrqlCondition.ViolationTimeLimitSeconds = 259200
	nrqlConditionPayload.NrqlCondition.Terms = []NRQLTerm{{
		Duration:     fmt.Sprintf("%v", condition.Duration),
		Operator:     condition.Operator,
		Threshold:    fmt.Sprintf("%.1f", condition.Threshold),
		TimeFunction: "all",
		Priority:     "critical",
	}}
	nrqlConditionPayload.NrqlCondition.Nrql.Query = condition.NRQL
	nrqlConditionPayload.NrqlCondition.Signal.AggregationWindow = "60"
	nrqlConditionPayload.NrqlCondition.Signal.AggregationMethod = "EVENT_FLOW"
	nrqlConditionPayload.NrqlCondition.Signal.AggregationDelay = 120
	nrqlConditionPayload.NrqlCondition.Signal.FillOption = "none"

	return nrqlConditionPayload
}
