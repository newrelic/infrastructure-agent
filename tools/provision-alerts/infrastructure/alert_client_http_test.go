package infrastructure

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAlertClientHttp_request(t *testing.T) {

	tests := []struct {
		responseCode int
		err          error
	}{
		{
			responseCode: 200,
			err:          nil,
		},
		{
			responseCode: 202,
			err:          nil,
		},
		{
			responseCode: 301,
			err:          nil,
		},
		{
			responseCode: 400,
			err:          errors.New("error occurred in the api client, resp code 400, url: https://host.com/some/url, body: , err: response body"),
		},
		{
			responseCode: 503,
			err:          errors.New("error occurred in the api client, resp code 503, url: https://host.com/some/url, body: , err: response body"),
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d response code", tt.responseCode), func(t *testing.T) {
			mockClient := &HttpClientMock{}
			alertClient := AlertClientHttp{
				host:   "https://host.com",
				apiKey: "some api key",
				client: mockClient,
			}

			url := "/some/url"
			var requestBody []byte
			responseBody := io.NopCloser(strings.NewReader("response body"))
			response := &http.Response{StatusCode: tt.responseCode, Body: responseBody}
			request, err := http.NewRequest("POST", alertClient.host+url, strings.NewReader(string(requestBody)))
			request.Header.Add("Api-Key", alertClient.apiKey)
			request.Header.Add("Content-Type", "application/json")

			mockClient.On("Do", mock.Anything).Return(response, nil)

			_, err = alertClient.Post(url, requestBody)
			assert.Equal(t, tt.err, err)
			//mocked objects assertions
			mock.AssertExpectationsForObjects(t, mockClient)
		})
	}
}
