// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	typeJson  = "json"  // the output will be decoded as JSON
	typeEqual = "equal" // the output will be decoded as key1=value1,key2=value2
	typePlain = "plain" // the output will be decoded just as plain text
)

// KMS defines the AWS-KMS data source
type KMS struct {
	Data           string
	File           string
	HTTP           *http
	CredentialFile string `yaml:"credential_file"`
	ConfigFile     string `yaml:"config_file"`
	Region         string `yaml:"region"`
	Endpoint       string `yaml:"endpoint"`
	DisableSSL     bool   `yaml:"disableSSL"`
	Type           string `yaml:"type,omitempty"` // can be 'json', 'equal' and 'plain' (default)
}

type kmsGatherer struct {
	cfg *KMS
}

// KMSGatherer instantiates a KMS variable gatherer from the given configuration. The fetching process
// // will return either a map containing access paths to the stored JSON or ShortHand, or a string if the
// // stored secret is just a string.
// E.g. if the stored secret is `{"car":{"brand":"Opel","model":"Corsa"}}`, the returned Map
// contents will be:
// "car.brand" -> "Opel"
// "car.model" -> "Corsa"
func KMSGatherer(kms *KMS) func() (interface{}, error) {
	g := kmsGatherer{cfg: kms}
	return func() (interface{}, error) {
		dt, err := g.get()
		if err != nil {
			return "", err
		}
		return dt, err
	}
}

func (g *kmsGatherer) get() (interface{}, error) {
	secret := g.cfg
	if secret.Data != "" {
		return g.retrieve([]byte(secret.Data))
	}
	if secret.File != "" {
		dt, err := os.ReadFile(secret.File)
		if err != nil {
			return nil, fmt.Errorf("unable to read aws-kms secret file '%s': %s", secret.File, err)
		}
		return g.retrieve(dt)
	}

	dt, err := httpRequest(secret.HTTP, "GET", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve aws-kms data from http server: %s", err)
	}
	return g.retrieve(dt)
}

// Validate checks if the KMS configuration is correct
func (k *KMS) Validate() error {
	if k.File == "" && k.Data == "" && (k.HTTP == nil || k.HTTP.URL == "") {
		return errors.New("aws-kms must have a file, data or http parameter in order to be set")
	}
	if k.Type != "" && k.Type != typeJson && k.Type != typeEqual && k.Type != typePlain {
		return errors.New("type can be only " + typePlain + ", " + typeJson + " or " + typeEqual)
	}
	return nil
}

func (g *kmsGatherer) retrieve(encoded []byte) (interface{}, error) {
	secret := g.cfg
	dt := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	if n, err := base64.StdEncoding.Decode(dt, encoded); err != nil {
		return nil, fmt.Errorf("unable to base64 decode aws-kms data: %s", err)
	} else {
		dt = dt[:n] // remove decoder leading zeroes
	}

	var err error
	var configLoadOptions []func(*config.LoadOptions) error
	if secret.CredentialFile != "" {
		tlog := slog.WithField("CredentialFile", secret.CredentialFile)
		tlog.Debug("Adding credentials file.")
		_, err := os.Stat(secret.CredentialFile)
		if err != nil {
			tlog.WithError(err).Warn("could not find credentials file so ignoring it")
		} else {
			configLoadOptions = append(configLoadOptions, config.WithSharedCredentialsFiles([]string{secret.CredentialFile}))
		}
	}
	if secret.ConfigFile != "" {
		tlog := slog.WithField("ConfigFile", secret.ConfigFile)
		tlog.Debug("Adding config file.")
		_, err := os.Stat(secret.ConfigFile)
		if err != nil {
			tlog.WithError(err).Warn("could not find config file so ignoring it")
		} else {
			configLoadOptions = append(configLoadOptions, config.WithSharedConfigFiles([]string{secret.ConfigFile}))
		}
	}

	if g.cfg.Region != "" {
		configLoadOptions = append(configLoadOptions, config.WithRegion(g.cfg.Region))
	}

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, configLoadOptions...)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws config for kms: %w", err)
	}

	kmsClient := kms.NewFromConfig(cfg, func(o *kms.Options) {
		if g.cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(g.cfg.Endpoint)
		}

		if g.cfg.DisableSSL {
			o.EndpointOptions.DisableHTTPS = true
		}
	})

	params := &kms.DecryptInput{
		CiphertextBlob: dt,
	}
	res, err := kmsClient.Decrypt(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt secret with aws-kms: %s", err)
	}
	return handleDataType(res.Plaintext, g.cfg.Type)
}

// this function converts from the stored payload to a map (dataType json, equal)
// or a string (dataType plain)
func handleDataType(kmsPayload []byte, dataType string) (interface{}, error) {
	switch dataType {
	case typeJson:
		var jsonResult data.InterfaceMap
		err := json.Unmarshal(kmsPayload, &jsonResult)
		if err != nil {
			return nil, fmt.Errorf("error hanking KMS data: %s", err.Error())
		}
		return jsonResult, nil
	case typeEqual:
		result := data.InterfaceMap{}
		commaSplit := bytes.Split(kmsPayload, []byte{','})
		for _, initialSplit := range commaSplit {
			equalSplit := bytes.SplitN(initialSplit, []byte{'='}, 2)
			if len(equalSplit) == 2 {
				result[string(equalSplit[0])] = string(equalSplit[1])
			}
		}
		return result, nil
	default:
		return string(kmsPayload), nil
	}
}
