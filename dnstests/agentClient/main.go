package main

import (
	"context"
	"flag"
	"fmt"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
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
	transport http.RoundTripper,
) (timedOut bool, err error) {
	var request *http.Request
	if request, err = http.NewRequest("HEAD", collectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare reachability request: %v, error: %s", request, err)
	}

	// enable http traces
	request = http2.WithTracer(request, "checkEndpointReachable")

	client := backendhttp.GetHttpClient(timeout, transport)
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

func lookupHost(defaultResolver bool, dest string) ([]string, error) {
	resolver := net.DefaultResolver
	if !defaultResolver {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(10000),
				}
				return d.DialContext(ctx, network, "8.8.8.8:53")
			},
		}
	}
	return resolver.LookupHost(context.Background(), dest)
}

func main() {

	agentConfig := flag.Bool("use_config", false, "Use agent configuration")
	flag.Parse()

	transport := http.DefaultTransport

	if *agentConfig {
		c, err := config.LoadConfig("")
		if err != nil {
			panic(err)
		}
		transport = backendhttp.BuildTransport(c, backendhttp.ClientTimeout)
	}

	for {
		timeOut, err := checkEndpointReachable(collectorURL, 5*time.Second, transport)
		if err != nil {
			logrus.WithError(err).WithField("collector_url", collectorURL).WithField("timeout", timeOut).WithField("agent_code", "yes").
				Error("Collector endpoint not reachable, retrying...")
			logrus.Info("Trying to resolve address with default resolver")

			addrs, err := lookupHost(true, collectorURL)
			if err != nil || len(addrs) < 1 {
				logrus.WithError(err).Error("Could not resolve the endpoint with default net resolver")
			} else {
				log.Println("Successful using the default resolver with net.LookupHost")
			}

			logrus.Info("Trying to resolve address with google dns resolver")
			addrs, err = lookupHost(true, collectorURL)
			if err != nil || len(addrs) < 1 {
				logrus.WithError(err).Error("Could not resolve the endpoint with GOOGLE net resolver")
			} else {
				log.Println("Successful using the GOOGLE resolver with net.LookupHost")
			}
		} else {
			log.Println("Connection was successful")
		}
		time.Sleep(5 * time.Second)
	}
}
