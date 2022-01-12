package main

import (
	"provision-alerts/config"
	"provision-alerts/service"
	"testing"
)

func Test_recreateAlerts(t *testing.T) {

	policy1 := service.Policy{Name: "policy1"}
	policy2 := service.Policy{Name: "policy2"}

	condition11 := service.Condition{Name: "cond11"}
	condition12 := service.Condition{Name: "cond12"}
	condition21 := service.Condition{Name: "cond21"}
	condition22 := service.Condition{Name: "cond22"}

	policy1WithCondition1 := service.Policy{Name: "policy1", Conditions: service.Conditions{condition11}}
	policy1WithCondition12 := service.Policy{Name: "policy1", Conditions: service.Conditions{condition11, condition12}}
	policy2WithCondition1 := service.Policy{Name: "policy2", Conditions: service.Conditions{condition21}}
	policy2WithCondition12 := service.Policy{Name: "policy2", Conditions: service.Conditions{condition21, condition22}}

	channel1 := 1111
	channel2 := 2222
	channel3 := 3333

	policy1WithConditionAndChannel1 := service.Policy{Name: "policy1", Conditions: service.Conditions{condition11, condition12}, Channels: []int{channel1}}
	policy2WithConditionAndChannel1 := service.Policy{Name: "policy2", Conditions: service.Conditions{condition11, condition12}, Channels: []int{channel1}}
	policy2WithConditionAndChannel2 := service.Policy{Name: "policy2", Conditions: service.Conditions{condition11, condition12}, Channels: []int{channel1, channel2}}
	policy2WithConditionAndChannel3 := service.Policy{Name: "policy2", Conditions: service.Conditions{condition11, condition12}, Channels: []int{channel1, channel2, channel3}}

	cfg := config.Config{
		Policies: config.PolicyConfigs{
			{
				Name:       policy1.Name,
				Channels:   []int{channel1},
				Conditions: config.ConditionConfigs{{Name: condition11.Name}, {Name: condition12.Name}},
			},
			{
				Name:       policy2.Name,
				Channels:   []int{channel1, channel2, channel3},
				Conditions: config.ConditionConfigs{{Name: condition21.Name}, {Name: condition22.Name}},
			},
		},
	}

	policyService := service.PolicyServiceMock{}

	policyService.ShouldDeleteByName(policy1.Name, nil)
	policyService.ShouldDeleteByName(policy2.Name, nil)

	policyService.ShouldCreate(cfg.Policies[0], policy1, nil)
	policyService.ShouldCreate(cfg.Policies[1], policy2, nil)

	policyService.ShouldAddCondition(policy1, cfg.Policies[0].Conditions[0], policy1WithCondition1, nil)
	policyService.ShouldAddCondition(policy1WithCondition1, cfg.Policies[0].Conditions[1], policy1WithCondition12, nil)
	policyService.ShouldAddCondition(policy2, cfg.Policies[1].Conditions[0], policy2WithCondition1, nil)
	policyService.ShouldAddCondition(policy2WithCondition1, cfg.Policies[1].Conditions[1], policy2WithCondition12, nil)

	policyService.ShouldAddChannel(policy1WithCondition12, cfg.Policies[0].Channels[0], policy1WithConditionAndChannel1, nil)
	policyService.ShouldAddChannel(policy2WithCondition12, cfg.Policies[1].Channels[0], policy2WithConditionAndChannel1, nil)
	policyService.ShouldAddChannel(policy2WithConditionAndChannel1, cfg.Policies[1].Channels[1], policy2WithConditionAndChannel2, nil)
	policyService.ShouldAddChannel(policy2WithConditionAndChannel2, cfg.Policies[1].Channels[2], policy2WithConditionAndChannel3, nil)

	recreateAlerts(cfg, &policyService)
}
