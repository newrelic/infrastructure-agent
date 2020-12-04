// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type Obfuscated struct {
	Key    string `yaml:"key"`
	Secret string `yaml:"secret"`
}

type obfuscateGatherer struct {
	cfg *Obfuscated
}

// ObfuscateGatherer instantiates a Obfuscate variable gatherer from the given configuration. The fetching process
// will return either a map containing access paths to the stored JSON.
// E.g. if the stored Secret is `{"account":{"user":"test1","password":"test2"}}`, the returned Map
// contents will be:
// "account.user"     -> "test1"
// "account.password" -> "test2"
func ObfuscateGatherer(obfuscated *Obfuscated) func() (interface{}, error) {
	g := obfuscateGatherer{cfg: obfuscated}
	return func() (interface{}, error) {
		dt, err := g.get()
		if err != nil {
			return "", err
		}
		return dt, err
	}
}

func (g *obfuscateGatherer) get() (interface{}, error) {
	credentials := g.cfg

	decrypted, err := decryptStringWithKey(credentials.Secret, credentials.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode obfuscated secret: %v", err)
	}
	result := data.InterfaceMap{}
	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return decrypted, nil
	}

	return result, nil
}

func (o *Obfuscated) Validate() error {
	if o.Key == "" {
		return errors.New("obfuscated secrets must have a Key parameter in order to be set")
	}
	if o.Secret == "" {
		return errors.New("obfuscated secrets must have a Secret parameter in order to be set")
	}
	return nil
}

// decryptStringWithKey decrypts an obfuscated string using a Key
// It XORs each byte of the value using part of the Key
// and converts it to a UTF8-string value.
// This is useful for obfuscating configuration values
func decryptStringWithKey(encodedText string, encodingKey string) (string, error) {
	textToDecodeBytes, err := base64.StdEncoding.DecodeString(encodedText)
	if err != nil {
		return "", err
	}

	textToDecodeBytesLen := len(textToDecodeBytes)

	encodingKeyBytes := []byte(encodingKey)
	encodingKeyLen := len(encodingKeyBytes)

	if encodingKeyLen == 0 || textToDecodeBytesLen == 0 {
		// Nothing to decrypt
		return "", nil
	}

	for i := 0; i < textToDecodeBytesLen; i++ {
		textToDecodeBytes[i] = textToDecodeBytes[i] ^ encodingKeyBytes[i%encodingKeyLen]
	}

	return string(textToDecodeBytes), nil
}
