package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
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
	err = ioutil.WriteFile(inventory, newConfigByte, 0644)
	if err != nil {
		panic(err)
	}
}

func executeAnsible() {
	fmt.Printf("Executing Ansible\n")

	curPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(
		"ansible-playbook",
		"-i", path.Join(curPath, "test/automated/ansible/inventory.local"),
		"--extra-vars", "@"+path.Join(curPath, inventory),
		path.Join(curPath, "test/automated/ansible/provision.yml"),
	)

	fmt.Println("Executing command: " + cmd.String())

	var errStdout, errStderr error

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		errStdout = copyAndCapture(os.Stdout, stdoutIn)

		wg.Done()
	}()
	go func() {
		errStderr = copyAndCapture(os.Stderr, stderrIn)

		wg.Done()
	}()

	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()
}
