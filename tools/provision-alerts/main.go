// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"provision-alerts/config"
	"provision-alerts/infrastructure"
	"provision-alerts/service"
)

var (
	displayNameCurrent  string
	displayNamePrevious string
	apiKey              string
	hostName            string
	template            string
	policyPrefix        string
	del                 bool
)

func main() {

	flag.StringVar(&displayNameCurrent, "display_name_current", "", "display name of current version")
	flag.StringVar(&displayNamePrevious, "display_name_previous", "", "display name of previous version")
	flag.StringVar(&apiKey, "api_key", "", "NR api key")
	flag.StringVar(&hostName, "host_name", "https://staging-api.newrelic.com", "NR api host (default staging)")
	flag.StringVar(&template, "template", "tools/provision-alerts/template/template.yml", "template path")
	flag.StringVar(&policyPrefix, "prefix", "[auto]", "policy name prefix")
	flag.BoolVar(&del, "delete", false, "delete policies matching prefix")

	flag.Parse()
	if !validArgs() {
		flag.Usage()
		os.Exit(1)
	}

	client := infrastructure.NewAlertClientHttp(hostName, apiKey, &http.Client{})
	policyService := service.NewPolicyApiService(client, policyPrefix)

	if del {
		deleteAlerts(policyPrefix, policyService)
		return
	}

	cfg, err := configFromTemplate()
	logFatalIfErr(err)

	cfg, err = config.FulfillConfig(cfg, displayNameCurrent, displayNamePrevious)
	logFatalIfErr(err)

	recreateAlerts(cfg, policyService)
}

func validArgs() bool {
	if del {
		if policyPrefix == "" && apiKey == "" {
			return false
		}
		return true
	}

	return displayNameCurrent != "" && displayNamePrevious != "" && apiKey != "" && template != "" && policyPrefix != ""
}

func configFromTemplate() (config.Config, error) {
	rawYAML, err := ioutil.ReadFile(template)
	logFatalIfErr(err)

	return config.ParseConfig(rawYAML)
}

func recreateAlerts(cfg config.Config, policyService service.PolicyService) {
	for _, policyConfig := range cfg.Policies {
		err := policyService.DeleteByName(policyConfig.Name)
		logFatalIfErr(err)

		policy, err := policyService.Create(policyConfig)
		logFatalIfErr(err)

		for _, conditionConfig := range policyConfig.Conditions {
			policy, err = policyService.AddCondition(policy, conditionConfig)
			logFatalIfErr(err)
		}

		for _, channelId := range policyConfig.Channels {

			policy, err = policyService.AddChannel(policy, channelId)
			logFatalIfErr(err)
		}
	}
}

func deleteAlerts(prefix string, policyService service.PolicyService) {
	policies, err := policyService.FindByName(prefix)
	logFatalIfErr(err)
	for _, pol := range policies {
		err = policyService.Delete(pol.Id)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func logFatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
