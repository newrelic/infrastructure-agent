package identityapi

import (
	"context"
	"github.com/newrelic/infrastructure-agent/pkg/identity-client"
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
