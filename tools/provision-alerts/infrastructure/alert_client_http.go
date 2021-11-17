package infrastructure

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type AlertClientHttp struct {
	host   string
	apiKey string
	client http.Client
}

func NewAlertClientHttp(host string, apiKey string, client http.Client) *AlertClientHttp {
	return &AlertClientHttp{host: host, apiKey: apiKey, client: client}
}

func (a AlertClientHttp) Post(uri string, body []byte) ([]byte, error) {
	return a.request("POST", uri, body)
}

func (a AlertClientHttp) Put(uri string, body []byte) ([]byte, error) {
	return a.request("PUT", uri, body)
}

func (a AlertClientHttp) Del(uri string, body []byte) ([]byte, error) {
	return a.request("DELETE", uri, body)
}

func (a AlertClientHttp) request(method, uri string, body []byte) ([]byte, error) {

	url := a.host + uri

	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))

	if err != nil {
		return nil, err
	}

	req.Header.Add("Api-Key", a.apiKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := a.client.Do(req)

	if err != nil {
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("error occured in the api client, resp code %d, url: %s, body: %s, err: %s", resp.StatusCode, url, body, bodyBytes)
	}

	return bodyBytes, nil
}
