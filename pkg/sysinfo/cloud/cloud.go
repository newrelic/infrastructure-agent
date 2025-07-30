// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"errors"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

// The type of the cloud on which the instance is running.
type Type string

const (
	TypeNoCloud    Type = "no_cloud"    // No cloud type has been detected.
	TypeInProgress Type = "in_progress" // Cloud detection is in progress.
	TypeAWS        Type = "aws"         // This instance is running in aws.
	TypeAzure      Type = "azure"       // This instance is running in Azure.
	TypeGCP        Type = "gcp"         // This instance is running in gcp.
	TypeAlibaba    Type = "alibaba"     // This instance is running in alibaba.
	TypeOci        Type = "Oci"         // This instance is running in Oracle Cloud Infrastructure (OCI).
)

var dlog = log.WithComponent("CloudDetector")

// ShouldCollect returns true if we should collect data for this cloud type.
func (t Type) ShouldCollect() bool {
	return t != TypeNoCloud && t != TypeInProgress
}

func (t Type) IsValidCloud() bool {
	return t == TypeAWS ||
		t == TypeAzure ||
		t == TypeGCP ||
		t == TypeAlibaba ||
		t == TypeOci
}

var (
	// ErrDetectorNotInitialized is the error returned when the Detector is not initialized yet.
	ErrDetectorNotInitialized = errors.New("cloud detector not initialized yet")
	// ErrCouldNotDetect is the error returned when the Detector could not be initialized.
	ErrCouldNotDetect = errors.New("detector is unable to detect the cloud type")
	// ErrMethodNotImplemented is the error returned when a method is still not implemented by an implementation.
	ErrMethodNotImplemented = errors.New("cloud harvester does not implement the method")
)

// Harvester is the interfaces that should be implemented by any cloud harvester.
type Harvester interface {
	// GetInstanceID will return the id of the cloud instance.
	GetInstanceID() (string, error)
	// GetHostType will return the cloud instance type.
	GetHostType() (string, error)
	// GetCloudType will return the cloud type on which the instance is running.
	GetCloudType() Type
	// Returns a string key which will be used as a HostSource (see host_aliases plugin).
	GetCloudSource() string
	// GetRegion returns the cloud region
	GetRegion() (string, error)
	// GetAccountID returns the cloud account
	GetAccountID() (string, error)
	// GetZone returns the cloud instance zone
	GetZone() (string, error)
	// GetInstanceImageID returns the cloud instance image ID
	GetInstanceImageID() (string, error)
	// GetInstanceDisplayName returns the cloud instance displayname
	GetInstanceDisplayName() (string, error)
	// GetInstanceTenantID returns the cloud instance tenant ID
	GetInstanceTenantID() (string, error)
	// GetHarvester returns instance of the Harvester detected (or instance of themselves)
	GetHarvester() (Harvester, error)
}

// Detector is used to detect the cloud type on which the instance is running
// and can be queried in order to get the information needed.
type Detector struct {
	lock                 sync.RWMutex
	maxRetriesNumber     int           // Specify how many times the Detector will try in case of failure.
	retryBackOff         time.Duration // Specify how much time to wait between the retries.
	expiryInSec          int           // The interval of time on which the metadata should be expired and re-fetched.
	disableCloudMetadata bool          // If set to true, detector will not try to fetch metadata.
	cloudHarvester       Harvester     // The detected cloud harvester object implementing Harvester interface.
	initialized          bool          // Flag to determine when the Detector is initialized.
	inProgress           bool          // Flag to determine when Detector initialization is in progress.
	disableKeepAlive     bool          // Disables HTTP keep-alives and will only use the connection to the server for a single HTTP request.
}

// NewDetector returns a new Detector instance.
func NewDetector(disableCloudMetadata bool, maxRetriesNumber, retryBackOffSec, expiryInSec int, disableKeepAlive bool) *Detector {
	return &Detector{
		maxRetriesNumber:     maxRetriesNumber,
		retryBackOff:         time.Duration(retryBackOffSec) * time.Second,
		expiryInSec:          expiryInSec,
		disableCloudMetadata: disableCloudMetadata,
		disableKeepAlive:     disableKeepAlive,
	}
}

type DetectorOption func(*Detector)

func WithProvider(cloudType Type) DetectorOption {
	return func(detector *Detector) {
		switch cloudType {
		case TypeAWS:
			detector.setHarvester(NewAWSHarvester(detector.disableKeepAlive))
			detector.finishInit()
		case TypeAzure:
			detector.setHarvester(NewAzureHarvester(detector.disableKeepAlive))
			detector.finishInit()
		case TypeGCP:
			detector.setHarvester(NewGCPHarvester(detector.disableKeepAlive))
			detector.finishInit()
		case TypeAlibaba:
			detector.setHarvester(NewAlibabaHarvester(detector.disableKeepAlive))
			detector.finishInit()
		case TypeOci:
			detector.setHarvester(NewOciHarvester(detector.disableKeepAlive))
			detector.finishInit()
		case TypeNoCloud:
		case TypeInProgress:
		default:
		}
	}
}

// Initialize should be called in order to Detect the cloud harvester.
func (d *Detector) Initialize(opts ...DetectorOption) {
	for _, opt := range opts {
		opt(d)
	}

	// If the above options did set a harvester, we don't need to initialize.
	if d.isInitialized() {
		return
	}

	harvesters := []Harvester{
		NewAWSHarvester(d.disableKeepAlive),
		NewAzureHarvester(d.disableKeepAlive),
		NewGCPHarvester(d.disableKeepAlive),
		NewAlibabaHarvester(d.disableKeepAlive),
		NewOciHarvester(d.disableKeepAlive),
	}
	d.initialize(harvesters...)
}

// initialize should be called in order to Detect the cloud harvester.
func (d *Detector) initialize(harvesters ...Harvester) {
	if d.isInitialized() || d.isInProgress() {
		return
	}

	if d.disableCloudMetadata {
		d.finishInit()
		return
	}

	d.initializeStart()

	err := d.detect(harvesters...)
	if err != nil {
		go d.detectRetrying(harvesters...)
	}
}

func (d *Detector) GetHarvester() (Harvester, error) {
	if cloudHarvester := d.getHarvester(); cloudHarvester != nil {
		return cloudHarvester, nil
	}
	if !d.isInitialized() {
		return nil, ErrDetectorNotInitialized
	}
	return nil, ErrCouldNotDetect
}

// GetInstanceID will return the id of the cloud instance.
func (d *Detector) GetInstanceID() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetInstanceID()
}

// GetHostType will return the cloud instance type.
func (d *Detector) GetHostType() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetHostType()
}

// GetRegion will return the region of cloud instance.
func (d *Detector) GetRegion() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetRegion()
}

// GetCloudType will return the cloud type on which the instance is running.
func (d *Detector) GetCloudType() Type {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		if err == ErrDetectorNotInitialized {
			return TypeInProgress
		}
		return TypeNoCloud
	}
	return cloudHarvester.GetCloudType()
}

// GetHostType will return the cloud instance type.
func (d *Detector) GetAccountID() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetAccountID()
}

// GetRegion will return the region of cloud instance.
func (d *Detector) GetInstanceImageID() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetInstanceImageID()
}

// GetCloudType will return the cloud type on which the instance is running.
func (d *Detector) GetZone() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetZone()
}

// GetCloudSource Returns a string key which will be used as a HostSource (see host_aliases plugin).
func (d *Detector) GetCloudSource() string {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return ""
	}
	return cloudHarvester.GetCloudSource()
}

// GetInstanceDisplayName returns the cloud instance display name
func (d *Detector) GetInstanceDisplayName() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetInstanceDisplayName()
}

// GetInstanceTenantID returns the cloud instance tenant ID
func (d *Detector) GetInstanceTenantID() (string, error) {
	cloudHarvester, err := d.GetHarvester()
	if err != nil {
		return "", err
	}
	return cloudHarvester.GetInstanceTenantID()
}

// isInitialized will check if the detector is Initialized.
func (d *Detector) isInitialized() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.initialized
}

// isInProgress will return true if Detector is in initialize process.
func (d *Detector) isInProgress() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.inProgress
}

// initializeStart is called when the detector initialization process starts.
func (d *Detector) initializeStart() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.inProgress = true
}

// finishInit is called when the initialize process finishes.
func (d *Detector) finishInit() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.initialized = true
	d.inProgress = false
}

// setHarvester will cache the Harvester instance.
func (d *Detector) setHarvester(harvester Harvester) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.cloudHarvester = harvester
}

// getHarvester returns the Harvester instance.
func (d *Detector) getHarvester() Harvester {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.cloudHarvester
}

// detectInBackground will try to detect the cloud type in background until maxRetriesNumber is reached.
func (d *Detector) detectRetrying(harvesters ...Harvester) {
	for i := 0; i < d.maxRetriesNumber; i++ {
		if err := d.detect(harvesters...); err == nil {
			break
		}
		if i == d.maxRetriesNumber-1 {
			log.Debug("Couldn't detect any known cloud, using no cloud type.")
		} else {
			log.Debugf("Failed to detect the cloud type, retrying in %f seconds", d.retryBackOff.Seconds())
			time.Sleep(d.retryBackOff)
		}
	}
	d.finishInit()
}

// detect will check which cloud harvester is able to successfully request data from API in order to detect the cloud type.
func (d *Detector) detect(harvesters ...Harvester) error {

	for _, harvester := range harvesters {
		if harvester == nil {
			continue
		}

		if instanceID, err := harvester.GetInstanceID(); err == nil {

			dlog.WithFields(logrus.Fields{
				"instanceId": instanceID,
				"cloudType":  harvester.GetCloudType(),
			}).Debug("Detected cloud type and retrieved instance ID")

			d.setHarvester(harvester)
			d.finishInit()
			return nil
		}
	}
	return ErrCouldNotDetect
}

// Timeout is used to check if a period of time has passed.
type Timeout struct {
	expiry   time.Time
	interval time.Duration // Interval on which the timeout should expire.
}

// NewTimeout will create a new Timeout instance.
func NewTimeout(seconds int) *Timeout {
	if seconds < 0 {
		seconds = 0
	}
	interval := time.Duration(seconds) * time.Second
	return &Timeout{
		expiry:   time.Now().Add(interval),
		interval: interval,
	}
}

// HasExpired will check if the timeout has expired.
func (t *Timeout) HasExpired() bool {
	now := time.Now()

	if t.expiry.Before(now) {
		t.expiry = now.Add(t.interval)
		return true
	}
	return false
}
