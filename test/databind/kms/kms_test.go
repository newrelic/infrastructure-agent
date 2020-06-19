// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package kms

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	test "github.com/newrelic/infrastructure-agent/test/databind"
)

func TestMain(m *testing.M) {

	if err := test.ComposeUp("./docker-compose.yml"); err != nil {
		log.Println("error on compose-up: ", err.Error())
		os.Exit(-1)
	}

	// KMS sometimes doesn't come up right away
	hc := http.DefaultClient
	for i := 0; i < 10; i++ {
		resp, err := hc.Get("http://localhost:18080/test")
		if err != nil {
			log.Println("error on compose-up: ", err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_, _ = ioutil.ReadAll(resp.Body)
		break
	}

	exitValChn := make(chan int, 1)
	func() {
		defer test.ComposeDown("./docker-compose.yml")
		exitValChn <- m.Run()
	}()

	exitVal := <-exitValChn
	os.Exit(exitVal)
}

func TestKMSData_Plain(t *testing.T) {
	data, cls := encode(t, "everything worked")
	defer cls()

	input := fmt.Sprintf(`
variables:
  kms:
    aws-kms:
      data: %s
      type: plain
      region: eu-west-1
      disableSSL: true
      endpoint: http://localhost:18080
`, data)

	values := fetch(t, input)
	t.Log(values)
	tmpl := map[string]string{
		"outcome": "${kms}",
	}
	matches, err := databind.Replace(&values, tmpl)
	require.NoError(t, err)
	match := matches[0].Variables.(map[string]string)
	assert.Equal(t, "everything worked", match["outcome"])
}

func TestKMSData_JSON(t *testing.T) {
	data, cls := encode(t, `{"everything":"worked","correctly":"yeah"}`)
	defer cls()

	input := fmt.Sprintf(`
variables:
  kms:
    aws-kms:
      data: %s
      type: json
      region: eu-west-1
      disableSSL: true
      endpoint: http://localhost:18080
`, data)

	values := fetch(t, input)
	tmpl := map[string]string{
		"everything": "${kms.everything}",
		"correctly":  "${kms.correctly}",
	}
	matches, err := databind.Replace(&values, tmpl)
	require.NoError(t, err)
	match := matches[0].Variables.(map[string]string)
	assert.Equal(t, "worked", match["everything"])
	assert.Equal(t, "yeah", match["correctly"])
}

func TestKMSData_Equal(t *testing.T) {
	data, cls := encode(t, `everything=worked,correctly=yeah`)
	defer cls()

	input := fmt.Sprintf(`
variables:
  kms:
    aws-kms:
      data: %s
      type: equal
      region: eu-west-1
      disableSSL: true
      endpoint: http://localhost:18080
`, data)

	values := fetch(t, input)
	tmpl := map[string]string{
		"everything": "${kms.everything}",
		"correctly":  "${kms.correctly}",
	}
	matches, err := databind.Replace(&values, tmpl)
	require.NoError(t, err)
	match := matches[0].Variables.(map[string]string)
	assert.Equal(t, "worked", match["everything"])
	assert.Equal(t, "yeah", match["correctly"])
}

// returns the encoded base64 value, the kms instance and the used key Id and a function that must be invoked on defer
func encode(t *testing.T, data string) (string, func()) {
	cfgs := aws.NewConfig().
		WithEndpoint("http://localhost:18080").
		WithDisableSSL(true).
		WithRegion("eu-west-2")

	kmsSession, err := session.NewSession(cfgs)
	require.NoError(t, err)

	k := kms.New(kmsSession)
	keys, err := k.ListKeys(&kms.ListKeysInput{})
	require.NoError(t, err)
	require.NotEmpty(t, keys.Keys)

	ko, err := k.CreateKey(&kms.CreateKeyInput{
		Description: aws.String("integration test key. Remove it")})
	require.NoError(t, err)
	deleteKey := func() {
		_, _ = k.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
			KeyId:               ko.KeyMetadata.KeyId,
			PendingWindowInDays: aws.Int64(int64(7)), // errors if less than 7
		})
	}

	eo, err := k.Encrypt(&kms.EncryptInput{
		KeyId:     ko.KeyMetadata.KeyId,
		Plaintext: []byte(data),
	})
	if err != nil {
		deleteKey()
	}
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(eo.CiphertextBlob), deleteKey
}

func fetch(t *testing.T, input string) databind.Values {
	t.Log(input)
	ctx, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	values, err := databind.Fetch(ctx)
	require.NoError(t, err)
	return values
}
