package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"provision-alerts/config"
	"provision-alerts/infrastructure"
	"provision-alerts/service"
)

func main() {

	displayNameCurrent := "current"
	displayNamePrevious := "current"

	rawYAML, err := ioutil.ReadFile("template/template.yml")

	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.ParseConfig(rawYAML)

	if err != nil {
		log.Fatal(err)
	}

	cfg, err = config.FulfillConfig(cfg,displayNameCurrent, displayNamePrevious)

	client := infrastructure.NewAlertClientHttp("https://staging-api.newrelic.com", "", http.Client{})

	policyService := service.NewPolicyApiService(client)

	// TODO delete all old policies, conver old ones first

	for _, policyConfig := range cfg.Policies {
		policy, err := policyService.Create(policyConfig)

		if err != nil {
			log.Fatal(err)
		}

		for _, conditionConfig := range policyConfig.Conditions {
			policy, err = policyService.AddCondition(policy, conditionConfig)

			if err != nil {
				log.Fatal(err)
			}
		}

		for _, channelId := range policyConfig.Channels {

			policy, err = policyService.AddChannel(policy, channelId)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
