package service

import (
	"github.com/stretchr/testify/mock"
	"provision-alerts/config"
)

type PolicyServiceMock struct {
	mock.Mock
}

func (p *PolicyServiceMock) DeleteByName(policyName string) error {
	args := p.Called(policyName)

	return args.Error(0)
}

func (p *PolicyServiceMock) ShouldDeleteByName(policyName string, err error) {
	p.
		On("DeleteByName", policyName).
		Once().
		Return(err)
}

func (p *PolicyServiceMock) Create(policyConfig config.PolicyConfig) (Policy, error) {
	args := p.Called(policyConfig)

	return args.Get(0).(Policy), args.Error(1)
}

func (p *PolicyServiceMock) ShouldCreate(policyConfig config.PolicyConfig, policy Policy, err error) {
	p.
		On("Create", policyConfig).
		Once().
		Return(policy, err)
}

func (p *PolicyServiceMock) AddCondition(policy Policy, condition config.ConditionConfig) (Policy, error) {
	args := p.Called(policy, condition)

	return args.Get(0).(Policy), args.Error(1)
}

func (p *PolicyServiceMock) ShouldAddCondition(policy Policy, condition config.ConditionConfig, policyWithCondition Policy, err error) {
	p.
		On("AddCondition", policy, condition).
		Once().
		Return(policyWithCondition, err)
}

func (p *PolicyServiceMock) AddChannel(policy Policy, channelId int) (Policy, error) {
	args := p.Called(policy, channelId)

	return args.Get(0).(Policy), args.Error(1)
}

func (p *PolicyServiceMock) ShouldAddChannel(policy Policy, channelId int, policyWithChannel Policy, err error) {
	p.
		On("AddChannel", policy, channelId).
		Once().
		Return(policyWithChannel, err)
}

func (p *PolicyServiceMock) Delete(id int) error {
	args := p.Called(id)

	return args.Error(0)
}

func (p *PolicyServiceMock) ShouldDelete(policyId int, err error) {
	p.
		On("Delete", policyId).
		Once().
		Return(err)
}

func (p *PolicyServiceMock) DeleteAll() error {
	args := p.Called()

	return args.Error(0)
}

func (p *PolicyServiceMock) ShouldDeleteAll(err error) {
	p.
		On("DeleteAll").
		Once().
		Return(err)
}
