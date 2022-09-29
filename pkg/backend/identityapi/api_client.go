// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"context"
	"github.com/newrelic/infra-identity-client-go/identity"
	"net/http"
)

type apiClient interface {
	RegisterPost(
		ctx context.Context,
		userAgent string,
		xLicenseKey string,
		registerRequest identity.RegisterRequest,
		localVarOptionals *identity.RegisterPostOpts) (identity.RegisterResponse, *http.Response, error)

	RegisterBatchPost(
		ctx context.Context,
		userAgent string,
		xLicenseKey string,
		registerRequest []identity.RegisterRequest,
		localVarOptionals *identity.RegisterBatchPostOpts) ([]identity.RegisterBatchEntityResponse, *http.Response, error)
}
