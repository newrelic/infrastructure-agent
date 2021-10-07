package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"sort"
	"strings"
)

type AnsibleGroupVars struct {
	ProvisionHostPrefix string        `yaml:"provision_host_prefix"`
	Instances           []instanceDef `yaml:"instances"`
}

type instanceDef struct {
	Ami               string `yaml:"ami"`
	InstanceType      string `yaml:"type"`
	Name              string `yaml:"name"`
	Username          string `yaml:"username"`
	PythonInterpreter string `yaml:"python_interpreter"`
	LaunchTemplate    string `yaml:"launch_template"`
}

func readAnsibleGroupVars() (*AnsibleGroupVars, error) {
	yamlContent, err := ioutil.ReadFile(instancesFile)
	if err != nil {
		log.Fatal(err.Error())
	}

	groupVars := &AnsibleGroupVars{}
	err = yaml.Unmarshal(yamlContent, groupVars)
	if err != nil {
		return nil, err
	}

	return groupVars, nil
}

func prepareAnsibleConfig(chosenOptions options, provisionHostPrefix string) {
	// prepare ansible config (tmp list of hosts to create)
	newConfig := AnsibleGroupVars{}
	newConfig.ProvisionHostPrefix = provisionHostPrefix
	for _, chosenOption := range chosenOptions {
		newConfig.Instances = append(newConfig.Instances, chosenOption.instance)
	}
	newConfigByte, err := yaml.Marshal(newConfig)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(inventoryForCreation, newConfigByte, 0644)
	if err != nil {
		panic(err)
	}
}

type provisionOption struct {
	id                 int
	name               string
	playbook           string
	args               map[string]string
	licenseKeyRequired bool
}

func (o provisionOption) Option() string {
	optionFormat := "[%d] %s"
	return fmt.Sprintf(optionFormat, o.id, o.name)
}

func (o provisionOption) renderArgs() string {
	var result string

	for key, val := range o.args {
		result += " -e " + key + "=" + val
	}

	return strings.TrimSpace(result)
}

type provisionOptions map[int]provisionOption

func (o provisionOptions) print() {
	for i := 0; i < len(o); i++ {
		fmt.Println(o[i].Option())
	}
}

func (o provisionOptions) toString() string {

	ordered := make([]int, 0)
	for id := range o {
		ordered = append(ordered, id)
	}
	sort.Ints(ordered)

	var s []string
	for _, id := range ordered {
		s = append(s, o[id].name)
	}
	return strings.Join(s, "\n - ")
}

func (o provisionOptions) filter(optionsIds []int) (provisionOptions, error) {
	filtered := make(provisionOptions)
	for _, optId := range optionsIds {
		if opt, ok := o[optId]; ok {
			filtered[optId] = opt
		} else {
			return nil, fmt.Errorf("%v is not a valid option", optId)
		}
	}
	return filtered, nil
}

const (
	OptionNothing = iota
	OptionInstallLatestProd
	OptionInstallLatestStaging
	OptionTestsProd
	OptionTestsStaging
	OptionHarvestTests
	OptionInstallVersionStaging
)

func newProvisionOptions() provisionOptions {
	opts := make(provisionOptions)

	opts[OptionNothing] = provisionOption{
		id:   0,
		name: "nothing",
	}

	opts[OptionInstallLatestProd] = provisionOption{
		id:       1,
		name:     "install latest version of agent from PROD",
		playbook: "test/packaging/ansible/installation-agent-no-clean.yml",
		args: map[string]string{
			"repo_endpoint": "https://download.newrelic.com/infrastructure_agent",
		},
		licenseKeyRequired: true,
	}

	opts[OptionInstallLatestStaging] = provisionOption{
		id:                 2,
		name:               "install latest version of agent from STG",
		playbook:           "test/packaging/ansible/installation-agent-no-clean.yml",
		licenseKeyRequired: true,
	}

	opts[OptionTestsProd] = provisionOption{
		id:       3,
		name:     "package tests from PROD",
		playbook: "test/packaging/ansible/test.yml",
		args: map[string]string{
			"repo_endpoint": "https://download.newrelic.com/infrastructure_agent",
		},
		licenseKeyRequired: true,
	}

	opts[OptionTestsStaging] = provisionOption{
		id:                 4,
		name:               "package tests from STG",
		playbook:           "test/packaging/ansible/test.yml",
		licenseKeyRequired: true,
	}

	opts[OptionHarvestTests] = provisionOption{
		id:       5,
		name:     "harvest tests",
		playbook: "test/harvest/ansible/test.yml",
	}

	opts[OptionInstallVersionStaging] = provisionOption{
		id:                 6,
		name:               "install given version of agent from STG",
		playbook:           "test/packaging/ansible/installation-agent-pinned-no-clean.yml",
		licenseKeyRequired: true,
	}

	return opts
}
