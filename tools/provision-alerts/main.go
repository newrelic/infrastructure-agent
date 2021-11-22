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
)

func main() {

	flag.StringVar(&displayNameCurrent, "display_name_current", "", "display name of current version")
	flag.StringVar(&displayNamePrevious, "display_name_previous", "", "display name of previous version")
	flag.StringVar(&apiKey, "api_key", "", "NR api key")
	flag.StringVar(&hostName, "host_name", "https://staging-api.newrelic.com", "NR api host (default staging)")
	flag.Parse()
	if !validArgs() {
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := configFromTemplate()
	logFatalIfErr(err)

	cfg, err = config.FulfillConfig(cfg, displayNameCurrent, displayNamePrevious)
	logFatalIfErr(err)

	client := infrastructure.NewAlertClientHttp(hostName, apiKey, &http.Client{})
	policyService := service.NewPolicyApiService(client)

	recreateAlerts(cfg, policyService)
}

func validArgs() bool {
	return displayNameCurrent != "" && displayNamePrevious != "" && apiKey != ""
}

func configFromTemplate() (config.Config, error) {
	rawYAML, err := ioutil.ReadFile("template/template.yml")
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

func logFatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
