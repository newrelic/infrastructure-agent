// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	gohttp "net/http"
)

type http struct {
	URL       string
	TLSConfig tlsConfig         `yaml:"tls_config"`
	Headers   map[string]string `yaml:"headers"`
}

type tlsConfig struct {
	Enable             bool   `yaml:"enable"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	MinVersion         uint16 `yaml:"min_version"`
	MaxVersion         uint16 `yaml:"max_version"`
	Ca                 string `yaml:"ca"`
}

func httpRequest(config *http, method string, body io.Reader) ([]byte, error) {
	client := &gohttp.Client{}
	tlsConfig := &tls.Config{
		MinVersion: config.TLSConfig.MinVersion,
		MaxVersion: config.TLSConfig.MaxVersion,
	}
	if config.TLSConfig.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = config.TLSConfig.InsecureSkipVerify
	}

	if config.TLSConfig.Ca != "" {
		rootCAs := x509.NewCertPool()
		ca, err := ioutil.ReadFile(config.TLSConfig.Ca)
		if err != nil {
			return nil, fmt.Errorf("unable to read certificate authority file: %s", err)
		}
		rootCAs.AppendCertsFromPEM(ca)
		tlsConfig.RootCAs = rootCAs
	}
	client.Transport = &gohttp.Transport{
		TLSClientConfig: tlsConfig,
	}

	req, err := gohttp.NewRequest(method, config.URL, body)
	if err != nil {
		return nil, fmt.Errorf("unable to create http request: %s", err)
	}
	for key, value := range config.Headers {
		req.Header.Add(key, value)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send http request: %s", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			slog.WithError(err).Warn("Unable to close response body")
		}
	}()

	if res.StatusCode != gohttp.StatusOK {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		return nil, fmt.Errorf("error response received from server: %s", res.Status)
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read http response body: %s", err)
	}
	return b, nil
}
