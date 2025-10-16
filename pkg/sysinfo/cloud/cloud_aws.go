// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
)

// htmlRegexp is used to check if the format response from aws has an unexpected format.
var htmlRegexp = regexp.MustCompile(`[<>]`)

const (
	// Headers for Token and TTL
	ttlHeader   = "x-aws-ec2-metadata-token-ttl-seconds"
	tokenHeader = "x-aws-ec2-metadata-token"
	// when harvester timeout 0 we need a default token TTL to perform any actions
	defaultTokenTTL = "10"

	// tokenEndpoint is the endpoint to get the IMDS token
	tokenEndpoint = "/latest/api/token"

	awsEC2MetadataHostname = "http://169.254.169.254"

	// awsMetaDataPath is the path of the URL used for requesting AWS metadata.
	awsMetaDataPath             = "/latest/meta-data/"
	instanceIdentityDocumentURL = "/latest/dynamic/instance-identity/document"
	defaultTimeout              = 600
)

// AWSHarvester is used to fetch data from AWS api.
type AWSHarvester struct {
	timeout                *Timeout
	tokenTimeout           *Timeout
	disableKeepAlive       bool
	instanceID             string // Cache the amazon instance ID.
	awsEC2MetadataHostname string
	awsEC2MetadataToken    atomic.Value
	instanceIdentityCache  *instanceIdentity
	httpClient             *http.Client
}

type instanceIdentity struct {
	AccountID        string `json:"accountId"`
	AvailabilityZone string `json:"availabilityZone"`
	ImageID          string `json:"imageID"`
	Region           string `json:"region"`
	InstanceType     string `json:"instanceType"`
	InstanceID       string `json:"instanceId"`
}

// NewAWSHarvester returns a new instance of AWSHarvester.
func NewAWSHarvester(disableKeepAlive bool) *AWSHarvester {
	return &AWSHarvester{
		timeout:                NewTimeout(defaultTimeout),
		tokenTimeout:           NewTimeout(defaultTimeout),
		disableKeepAlive:       disableKeepAlive,
		awsEC2MetadataHostname: awsEC2MetadataHostname,
		httpClient:             clientWithFastTimeout(disableKeepAlive),
	}
}

// GetHarvester returns instance of the Harvester detected (or instance of themselves)
func (a *AWSHarvester) GetHarvester() (Harvester, error) {
	return a, nil
}

// GetInstanceID returns the AWS instance ID.
func (a *AWSHarvester) GetInstanceID() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.InstanceID, nil
}

func (a *AWSHarvester) loadInstanceData() (*instanceIdentity, error) {
	if a.instanceIdentityCache != nil && !a.timeout.HasExpired() {
		return a.instanceIdentityCache, nil
	}
	i, err := a.getInstanceIdentity()
	if err != nil {
		return nil, err
	}
	a.instanceIdentityCache = i
	return a.instanceIdentityCache, nil
}

// GetHostType will return the cloud instance type.
func (a *AWSHarvester) GetHostType() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.InstanceType, nil
}

// GetRegion will return the cloud region.
func (a *AWSHarvester) GetRegion() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.Region, nil
}

// GetAccount will return the cloud account.
func (a *AWSHarvester) GetAccountID() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.AccountID, nil
}

// GetAvailability will return the cloud availability zone.
func (a *AWSHarvester) GetZone() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.AvailabilityZone, nil
}

// GetImageID will return the cloud image ID.
func (a *AWSHarvester) GetInstanceImageID() (string, error) {
	icc, err := a.loadInstanceData()
	if err != nil {
		return "", err
	}
	return icc.ImageID, nil
}

// GetCloudType returns the type of the cloud.
func (a *AWSHarvester) GetCloudType() Type {
	return TypeAWS
}

// GetCloudSource returns a string key which will be used as a HostSource (see host_aliases plugin).
func (a *AWSHarvester) GetCloudSource() string {
	return sysinfo.HOST_SOURCE_INSTANCE_ID
}

// GetInstanceDisplayName returns the cloud instance display name (not supported for AWS)
func (a *AWSHarvester) GetInstanceDisplayName() (string, error) {
	return "", ErrMethodNotImplemented
}

// GetVMSize returns the cloud instance VM size (not supported for AWS).
func (a *AWSHarvester) GetVMSize() (string, error) {
	return "", ErrMethodNotImplemented
}

// formatURL prepares the URL used for requesting AWS metadata.
func formatURL(baseUrl string, query string) string {
	return baseUrl + awsMetaDataPath + query
}

// getInstanceIdentity retrieves the aws ec2 identity document
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
//
// This json contains metadata about the ec2 instance. The following is
// an example response:
//
//	{
//	    "devpayProductCodes" : null,
//	    "marketplaceProductCodes" : [ "1abc2defghijklm3nopqrs4tu" ],
//	    "availabilityZone" : "us-west-2b",
//	    "privateIp" : "10.158.112.84",
//	    "version" : "2017-09-30",
//	    "instanceId" : "i-1234567890abcdef0",
//	    "billingProducts" : null,
//	    "instanceType" : "t2.micro",
//	    "accountId" : "123456789012",
//	    "imageId" : "ami-5fb8c835",
//	    "pendingTime" : "2016-11-19T16:32:11Z",
//	    "architecture" : "x86_64",
//	    "kernelId" : null,
//	    "ramdiskId" : null,
//	    "region" : "us-west-2"
//	}
func (a *AWSHarvester) getInstanceIdentity() (*instanceIdentity, error) {
	token, err := a.getToken()
	if err != nil {
		return nil, err
	}

	documentURL := a.awsEC2MetadataHostname + instanceIdentityDocumentURL
	request, err := http.NewRequest(http.MethodGet, documentURL, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare AWS metadata request: %v", request)
	}

	request.Header.Add("X-aws-ec2-metadata-token", token)

	response, err := a.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch AWS metadata: %s", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(ioutil.Discard, response.Body)
		return nil, fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
	}

	blob, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read cloud metadata response: %s", err)
	}

	var i instanceIdentity
	err = json.Unmarshal(blob, &i)

	if err != nil {
		return nil, fmt.Errorf("unable to decode cloud metadata response: %s", err)
	}

	return &i, nil
}

func (a *AWSHarvester) getToken() (string, error) {

	if token := a.awsEC2MetadataToken.Load(); token != nil && !a.tokenTimeout.HasExpired() {
		return token.(string), nil
	}

	tokenURL := a.awsEC2MetadataHostname + tokenEndpoint
	request, err := http.NewRequest(http.MethodPut, tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("unable to prepare AWS metadata request: %v", request)
	}

	if a.timeout.interval.Seconds() < 1 {
		request.Header.Add(ttlHeader, defaultTokenTTL)
	} else {
		request.Header.Add(ttlHeader, fmt.Sprintf("%v", a.timeout.interval.Seconds()))
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("unable to fetch AWS metadata: %s", err)
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(ioutil.Discard, response.Body)
		return "", fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
	}

	bs, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read cloud metadata response: %s", err)
	}
	token := string(bs)
	a.awsEC2MetadataToken.Store(token)
	return token, nil
}

// GetAWSMetadataValue is used to request metadata from aws API.
func (a *AWSHarvester) GetAWSMetadataValue(fieldName string, disableKeepAlive bool) (data string, err error) {
	url := formatURL(a.awsEC2MetadataHostname, fieldName)

	var request *http.Request
	client := clientWithFastTimeout(disableKeepAlive)
	if request, err = http.NewRequest(http.MethodGet, url, nil); err != nil {
		err = fmt.Errorf("unable to prepare AWS metadata request: %v", request)
		return
	}
	token, err := a.getToken()
	if err != nil {
		return "", err
	}
	request.Header.Add(tokenHeader, token)

	var response *http.Response
	if response, err = client.Do(request); err != nil {
		err = fmt.Errorf("unable to fetch AWS metadata: %s", err)
		return
	}
	defer response.Body.Close()

	data, err = parseAWSMetaResponse(response)
	if err != nil {
		err = fmt.Errorf("can't parse response from %s: %s", url, err)
	}
	return
}

// parseAWSMetaResponse is used to parse the value required from AWS response.
func parseAWSMetaResponse(response *http.Response) (value string, err error) {
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("cloud metadata request returned non-OK response: %d %s", response.StatusCode, response.Status)
		return
	}

	// This is where we start trying to protect ourselves from accidentally ingesting a router error page.
	contentType := response.Header.Get(http.CanonicalHeaderKey("content-type"))
	if contentType != "text/plain" {
		err = fmt.Errorf("got invalid content type back from cloud metadata endpoint: %s", contentType)
		return
	}

	var blob []byte
	if blob, err = ioutil.ReadAll(response.Body); err != nil {
		err = fmt.Errorf("unable to read cloud metadata response: %s", err)
		return
	}

	// the response should just be a bit of text. any newlines or HTML looking things are suspect
	value = strings.TrimSpace(string(blob))
	if htmlRegexp.MatchString(value) {
		err = fmt.Errorf("response from cloud metadata endpoint is an unexpected format")
		return
	}

	return
}
