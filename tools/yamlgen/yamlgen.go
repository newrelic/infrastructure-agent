// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
)

const (
	CONFIG_FILE_PATH   = "C:\\Program Files\\New Relic\\newrelic-infra"
	CONFIG_FILE_NAME   = "newrelic-infra.yml"
	LICENSE_KEY        = "license_key"
	INDENT             = "    " // Using four-space indentation.
	CRLF               = "\r\n"
	KEY_PREFIX         = "-"
	REQUIRED_ARG_COUNT = 3 // Base -license_key value.
)

func fatalIfErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:\n yamlgen -license_key \"your-key\" [-proxy \"your-proxy\" -display_name \"your-name\"  -custom_attributes \"{'things':'more things','other things':'foobar'}\"")
}

func main() {
	foundLicense := false

	if len(os.Args) < REQUIRED_ARG_COUNT {
		usage()
		os.Exit(1)
	}

	configPath := os.Getenv("CONFIG_FILE_PATH")
	if len(configPath) == 0 {
		configPath = CONFIG_FILE_PATH // os.Args[CONFIG_ARG]
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// For now, assume this is being used as part of an installer that built
		// the target directory.
		// if err := os.MkdirAll(configPath, 0755); err != nil {
		fatalIfErr(err)
		// }
	}

	yamlMap := make(map[string]string)
	for i := 1; i < len(os.Args)-1; i += 1 { // Change this if supporting CONFIG_ARG.
		key := os.Args[i]
		value := os.Args[i+1]
		// Confirm the key has a leading hyphen, then remove it.
		if strings.HasPrefix(key, KEY_PREFIX) && len(key) > 1 && !strings.HasPrefix(value, KEY_PREFIX) && len(value) > 0 {
			key = strings.TrimPrefix(key, KEY_PREFIX)
			if key == LICENSE_KEY {
				foundLicense = true
			}
			yamlMap[key] = value
			i += 1
		}
	}

	fileBytes := []byte("")

	for key, value := range yamlMap {
		switch key {
		case "custom_attributes":
			bytesToWrite := []byte(key + ":" + CRLF + convertCustomAttributes(value))
			fileBytes = append(bytesToWrite, fileBytes...)
		case "custom_supported_filesystems", "file_devices_blacklist", "file_devices_ignored":
			bytesToWrite := []byte(key + ":" + CRLF + convertList(value))
			fileBytes = append(bytesToWrite, fileBytes...)
		default:
			bytesToWrite := []byte(key + ": " + value + CRLF)
			fileBytes = append(bytesToWrite, fileBytes...)
		}
	}

	if len(fileBytes) > 0 && foundLicense {
		header := []byte("# THIS FILE IS MACHINE GENERATED" + CRLF)
		fileBytes = append(header, fileBytes...)
		err := disk.WriteFile(filepath.Join(configPath, CONFIG_FILE_NAME), fileBytes, 0644)
		fatalIfErr(err)
	} else {
		fatalIfErr(fmt.Errorf("Incorrect input, confirm that license_key is provided and other arguments are correct."))
	}
	os.Exit(0)
}

func convertCustomAttributes(customAttributes string) string {
	result := []byte("")
	data := []byte(customAttributes)
	yaml, err := yaml.JSONToYAML(data)
	fatalIfErr(err)

	ca := strings.Split(string(yaml), "\n")
	for i := 0; i < len(ca)-1; i++ {
		// Until the agent can deal with quoted strings...
		kv := strings.Split(ca[i], ":")
		key := kv[0]
		value := strings.Trim(kv[1], "\" ")
		result = append(result, INDENT+key+": "+value+CRLF...)
	}
	return string(result)
}

func convertList(listItem string) string {
	result := []byte("")
	items := strings.Split(listItem, ",")
	for _, item := range items {
		result = append(result, INDENT+"- "+item+CRLF...)
	}
	return string(result)
}
