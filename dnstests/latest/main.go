package main

import (
	"bytes"
	"context"
	"fmt"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os/exec"
	"time"
)

const (
	collectorURL  = "https://infra-api.newrelic.com/cdn-cgi/trace"
	collectorHost = "infra-api.newrelic.com"
	timeout       = time.Second * 10
)

func main() {

	logrus.SetLevel(logrus.DebugLevel)

	for {

		logrus.Info("/*********************************************/")
		logrus.Info("/************ 	START 	*****************/")
		logrus.Info("/*********************************************/")

		/*********************************************/
		//go native LookupHost default resolver
		/*********************************************/
		logrus.Info("************** go native LookupHost default resolver *******************")
		addrs, err := net.LookupHost(collectorHost)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot resolve %s", collectorHost))
		} else {
			logrus.WithField("addrs", addrs).Info("Native lookup host default resolver : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native LookupHost PreferGo Default resolver
		/*********************************************/
		logrus.Info("************** go native LookupHost PreferGo Default resolver *******************")
		resolver := net.DefaultResolver
		resolver.PreferGo = true
		addrs, err = resolver.LookupHost(context.Background(), collectorHost)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot resolve %s", collectorHost))
		} else {
			logrus.WithField("addrs", addrs).Info("go native LookupHost PreferGo resolver : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native LookupIP Default resolver
		/*********************************************/
		logrus.Info("************** go native LookupIP default resolver *******************")
		ips, err := net.LookupIP(collectorHost)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot resolve %s", collectorHost))
		} else {
			logrus.WithField("ips", ips).Info("Native lookup host default resolver : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native LookupIP preferGo Default resolver
		/*********************************************/
		logrus.Info("************** go native LookupIP preferGo default resolver *******************")
		resolver = net.DefaultResolver
		resolver.PreferGo = true
		ips, err = resolver.LookupIP(context.Background(), "ip", collectorHost)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot resolve %s", collectorHost))
		} else {
			logrus.WithField("ips", ips).Info("Native lookup host default resolver : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native http client Head
		/*********************************************/
		logrus.Info("************** go native http.Head *******************")
		resp, err := http.Head(collectorURL)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot HEAD %s", collectorURL))
		} else {
			logrus.WithField("StatusCode", resp.StatusCode).Info("go native http.Head : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native req with tracer
		/*********************************************/
		logrus.Info("************** go native req with tracer *******************")
		req, err := http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			req = http2.WithTracer(req, "testing")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot execute native req with tracer for %s", collectorURL))
			} else {
				logrus.WithField("statusCode", response.StatusCode).Info("go native req with tracer: OK")
			}
		}
		logrus.Info("\n\n")

		/*********************************************/
		//go native req with tracer and prefer go resolver
		/*********************************************/
		logrus.Info("************** go native req with tracer and prefer go resolver *******************")
		req, err = http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			resolver := net.DefaultResolver
			resolver.PreferGo = true
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				Resolver:  resolver,
			}
			customTransport := &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           dialer.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}
			client := http.Client{}
			client.Transport = customTransport
			req = http2.WithTracer(req, "testing")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot execute native req with tracer for %s", collectorURL))
			} else {
				logrus.WithField("statusCode", response.StatusCode).Info("go native req with tracer and prefer go resolver: OK")
			}
		}
		logrus.Info("\n\n")

		/*********************************************/
		//NR http client Head Custom transport for custom dial
		/*********************************************/
		resolver = net.DefaultResolver
		resolver.PreferGo = true
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  resolver,
		}
		customTransport := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		logrus.Info("************** NR http client Head Default transport *******************")
		client := backendhttp.GetHttpClient(timeout, customTransport)
		req, err = http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			req = http2.WithTracer(req, "testing")
			resp, err := client.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot Head Default transport With tracer %s", collectorURL))
			} else {
				logrus.WithField("StatusCode", resp.StatusCode).Info("NR http client Head Default transport With tracer : OK")
			}
			logrus.Info("\n\n")
		}

		/*********************************************/
		//NR http client Head With Custom DNS resolver
		/*********************************************/
		resolver = net.DefaultResolver
		resolver.PreferGo = true
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, "8.8.8.8:53")
		}
		dialer = &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  resolver,
		}
		customTransport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		logrus.Info("************** NR http client Head With Custom DNS resolver *******************")
		client = backendhttp.GetHttpClient(timeout, customTransport)
		req, err = http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			req = http2.WithTracer(req, "testing")
			resp, err := client.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot Head Default transport With tracer %s", collectorURL))
			} else {
				logrus.WithField("StatusCode", resp.StatusCode).Info("NR http client Head With Custom DNS resolver : OK")
			}
			logrus.Info("\n\n")
		}

		/*********************************************/
		//NR http client Head Default transport
		/*********************************************/
		logrus.Info("************** NR http client Head Default transport *******************")
		client = backendhttp.GetHttpClient(timeout, http.DefaultTransport)
		resp, err = client.Head(collectorURL)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot HEAD %s", collectorURL))
		} else {
			logrus.WithField("StatusCode", resp.StatusCode).Info("NR http client Head : OK")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//NR http client Head Default transport With tracer
		/*********************************************/
		logrus.Info("************** NR http client Head Default transport With tracer *******************")
		client = backendhttp.GetHttpClient(timeout, http.DefaultTransport)
		req, err = http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			req = http2.WithTracer(req, "testing")
			resp, err = client.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot Head Default transport With tracer %s", collectorURL))
			} else {
				logrus.WithField("StatusCode", resp.StatusCode).Info("NR http client Head Default transport With tracer : OK")
			}
			logrus.Info("\n\n")
		}

		/*********************************************/
		//shell curl
		/*********************************************/
		logrus.Info("************** Shell curl HEAD *******************")
		err, stdout, stderr := executeCommand("curl", "--head", collectorURL)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Shell curl HEAD %s", collectorURL))
		} else {
			logrus.WithField("stdout", stdout).Info("Shell curl HEAD : OK")
			logrus.WithField("stderr", stderr).Info("Shell curl HEAD : STDERR")
		}
		logrus.Info("\n\n")

		/*********************************************/
		//shell dig
		/*********************************************/
		logrus.Info("************** Shell dig *******************")
		err, stdout, stderr = executeCommand("dig", collectorURL)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Shell dig %s", collectorURL))
		} else {
			logrus.WithField("stdout", stdout).Info("Shell dig : OK")
			logrus.WithField("stderr", stderr).Info("Shell dig : STDERR")
		}
		logrus.Info("\n\n")

		time.Sleep(5 * time.Second)
	}
}

func executeCommand(command string, args ...string) (error, string, string) {

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return err, stdout.String(), stderr.String()
}

func googleResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, "8.8.8.8:53")
		},
	}
}
