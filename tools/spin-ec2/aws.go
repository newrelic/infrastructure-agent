// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

var versionRegex = regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+`)

// getAWSInstances return canary instances names mapped on ids
// it will only return instances prefixed with hostPrefix
// if platform is not empty it will only retrieve platform specific instances
func getAWSInstances(hostPrefix string, platform string) (map[string]string, error) {
	args := []string{
		"ec2",
		"describe-instances",
		"--output", "text",
		"--query", "Reservations[*].Instances[*].[Tags[?Key==`Name`], InstanceId]",
	}

	nameFilter := []string{
		"--filters",
		fmt.Sprintf(`Name=tag:Name,Values=%s*`, hostPrefix),
		`Name=instance-state-name,Values=running`,
	}
	// limit to platform if not empty. Multiple filters are treated as AND. Multiple values in filter as OR
	switch platform {
	case "linux":
		nameFilter = append(nameFilter, `Name=tag:Name,Values=*ubuntu*,*debian*,*centos*,*redhat*,*sles*,*al*`)
	case "windows":
		nameFilter = append(nameFilter, `Name=tag:Name,Values=windows*`)
	}

	args = append(args, nameFilter...)
	cmd := exec.Command("aws", args...)

	var out bytes.Buffer
	var outErr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &outErr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get aws instances, error: %s", outErr.String())
	}

	return parseAWSInstances(out.String())
}

// parseAWSInstances parses the ec2 describe-instances response.
func parseAWSInstances(response string) (map[string]string, error) {
	result := make(map[string]string)
	split := strings.Split(response, "\n")

	for i := 0; i < len(split)-1; i += 2 {
		instanceID := strings.TrimSpace(split[i])

		if !strings.HasPrefix(split[i+1], "Name") {
			return nil, fmt.Errorf("unexpected line received for instanceName: %s", split[i+1])
		}
		instanceName := strings.TrimSpace(strings.TrimPrefix(split[i+1], "Name"))

		result[instanceID] = instanceName
	}

	return result, nil
}

// terminateInstances will terminate all instances.
func terminateInstances(idsToTerminate []string, instances map[string]string, dryRun bool) error {
	if len(idsToTerminate) == 0 {
		return nil
	}

	for _, id := range idsToTerminate {
		fmt.Println(colorizeRed(
			fmt.Sprintf("[DryRun: '%t'] Removing the following instance: %s -> %s",
				dryRun, id, instances[id])))
	}

	args := []string{
		"ec2",
		"terminate-instances",
		"--instance-ids",
	}
	args = append(args, idsToTerminate...)

	if !dryRun {
		cmd := exec.Command("aws", args...)

		var out bytes.Buffer
		var outErr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &outErr

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to terminate aws instances, error: %s", outErr.String())
		}

		fmt.Println(out.String())
	}

	return nil
}

func getPreviousCanaryVersion(instances map[string]string) (string, error) {
	versions, err := detectVersions(instances)
	if err != nil {
		return "", err
	}

	if len(versions) < 2 {
		return "", fmt.Errorf("in order to use it, you need to provision 2 versions of canaries first")
	}

	semver.Sort(versions)

	return versions[len(versions)-2], nil
}

// getInstancesToPrune will filter the instances that should be terminated.
func getInstancesToPrune(instances map[string]string, versionsToKeep int) ([]string, error) {
	versions, err := detectVersions(instances)
	if err != nil {
		return nil, err
	}

	var idsToRemove []string

	for _, versionToRemove := range getVersionsToRemove(versions, versionsToKeep) {
		for id, name := range instances {
			if !strings.Contains(name, versionToRemove) {
				continue
			}
			idsToRemove = append(idsToRemove, id)
		}
	}

	return idsToRemove, nil
}

// getVersionsToRemove keeps the latest 2 version of infra agent.
func getVersionsToRemove(versions []string, versionsToKeep int) []string {
	semver.Sort(versions)

	if len(versions) > versionsToKeep {
		return versions[:len(versions)-versionsToKeep]
	}

	return []string{}
}

// detectVersions will get the instance names, extract the agent version and return a sorted unique list.
func detectVersions(instances map[string]string) ([]string, error) {
	var versions []string

	unique := make(map[string]struct{})

	for _, name := range instances {
		matches := versionRegex.FindAllString(name, 1)
		if len(matches) < 1 {
			return nil, fmt.Errorf("failed to match version in instance name: '%s'", name)
		}

		version := matches[0]
		if _, exists := unique[version]; !exists {
			unique[version] = struct{}{}
			versions = append(versions, version)
		}
	}

	return versions, nil
}
