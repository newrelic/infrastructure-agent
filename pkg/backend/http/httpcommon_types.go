// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"net/http"
	"time"
)

// HTTP default values
const (
	LicenseHeader       = "X-License-Key"
	EntityKeyHeader     = "X-NRI-Entity-Key" // populated with the agent-id for the backend deny mechanism
	AgentEntityIdHeader = "X-NRI-Agent-Entity-Id"

	TrialStatusHeader = "X-Trial-Status"
	TrialStarting     = "starting"

	ClientTimeout = 30 * time.Second
)

type ResponseMetadata struct {
	Previous           string                 `json:"previous,omitempty"`
	Next               string                 `json:"next,omitempty"`
	Before             string                 `json:"before,omitempty"`
	After              string                 `json:"after,omitempty"`
	PerPage            int                    `json:"per_page,omitempty"`
	RateLimitLimit     int                    `json:"rate_limit_limit,omitempty"`
	RateLimitRemaining int                    `json:"rate_limit_remaining,omitempty"`
	RateLimitReset     int                    `json:"rate_limit_reset,omitempty"`
	Stats              map[string]interface{} `json:"stats,omitempty"`
}

type StandardResponse struct {
	Payload  interface{}       `json:"payload"`
	Metadata *ResponseMetadata `json:"metadata,omitempty"`
}

// IsResponseSuccess is a successful backend response
func IsResponseSuccess(resp *http.Response) bool {
	return resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusCreated ||
		resp.StatusCode == http.StatusAccepted
}

// IsResponseError is a non successful backend response.
func IsResponseError(resp *http.Response) bool {
	return !IsResponseSuccess(resp)
}

// RetryPolicy defines the retry behaviour.
type RetryPolicy struct {
	After      time.Duration
	MaxBackOff time.Duration
}

// ErrorCause is used to identify the type of the ingestError.
type ErrorCause string

const (
	// TrialInactive is returned when the trial had not been started.
	TrialInactive ErrorCause = "trial_inactive"

	// TrialExpired is returned when the trial had expired.
	TrialExpired ErrorCause = "trial_expired"

	// InvalidLicense is returned when the license key is invalid.
	InvalidLicense ErrorCause = "invalid_license"

	// ServiceError is the error returned by the identity service.
	ServiceError ErrorCause = "service_error"
)

// IsResponseUnsuccessful will return the cause of the error if it's the case.
func IsResponseUnsuccessful(resp *http.Response) (hasError bool, cause ErrorCause) {
	hasError = !IsResponseSuccess(resp)

	if !hasError {
		return
	}

	switch resp.StatusCode {

	case http.StatusForbidden:
		trialStatusH := resp.Header.Get(TrialStatusHeader)
		if trialStatusH == TrialStarting {
			cause = TrialInactive
			return
		}
		cause = TrialExpired
		return

	case http.StatusUnauthorized:
		cause = InvalidLicense
		return
	default:
		cause = ServiceError
		return
	}
}
