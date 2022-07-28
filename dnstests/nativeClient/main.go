package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	collectorURL = "https://infra-api.newrelic.com"
)

func checkEndpointReachable(
	collectorURL string,
	timeout time.Duration,
) (timedOut bool, err error) {

	var request *http.Request
	if request, err = http.NewRequest("HEAD", collectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare reachability request: %v, error: %s", request, err)
	}

	client := http.Client{
		Timeout: timeout,
	}
	if _, err = client.Do(request); err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedOut = true
		}
		if errURL, ok := err.(*url.Error); ok {
			logrus.WithError(errURL).Warn("URL error detected. May be a configuration problem or a network connectivity issue.")
			timedOut = true
		}
	}

	return
}

func main() {

	for {
		timeOut, err := checkEndpointReachable(collectorURL, 5*time.Second)
		if err != nil {
			logrus.WithError(err).WithField("collector_url", collectorURL).WithField("timeout", timeOut).WithField("agent_code", "no").
				Error("Collector endpoint not reachable, retrying...")
		} else {
			log.Println("Connection was successful, without using agent's code")
		}
		time.Sleep(5 * time.Second)
	}
}
