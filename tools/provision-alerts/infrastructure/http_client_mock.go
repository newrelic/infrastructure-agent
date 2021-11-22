package infrastructure

import (
	"github.com/stretchr/testify/mock"
	"net/http"
)

type HttpClientMock struct {
	mock.Mock
}

func (h *HttpClientMock) Do(req *http.Request) (*http.Response, error) {
	args := h.Called(req)

	return args.Get(0).(*http.Response), args.Error(1)
}
