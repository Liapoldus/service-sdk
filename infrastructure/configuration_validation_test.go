package infrastructure

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type validationCaptureTransport struct {
	request *http.Request
	body    []byte
	status  int
}

func (transport *validationCaptureTransport) Do(request *http.Request) (*http.Response, error) {
	transport.request = request
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	transport.body = body
	transport.status = http.StatusNoContent
	return &http.Response{StatusCode: transport.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestValidateConfigurationTransmitsExactCandidateGenerationAndBytes(t *testing.T) {
	contract := newHTTPContract()
	transport := &validationCaptureTransport{}
	client := &ServiceClient{
		contract:  contract,
		baseURL:   &url.URL{Scheme: "https", Host: "service.example"},
		routes:    map[string]Endpoint{serviceRouteConfigValidate: contract.Service.Endpoints[serviceRouteConfigValidate]},
		transport: transport,
		read:      1,
	}
	raw := []byte("{ \"candidate\" : 1 }\n")
	if err := client.ValidateConfiguration(context.Background(), "19", raw); err != nil {
		t.Fatalf("validation failed status=%d request=%#v: %v", transport.status, transport.request, err)
	}
	if transport.request == nil || transport.request.Header.Get(contract.Service.ConfigurationValidationGenerationHeader) != "19" {
		t.Fatalf("candidate generation header = %#v", transport.request)
	}
	if string(transport.body) != string(raw) {
		t.Fatalf("candidate bytes changed: %q", transport.body)
	}
}

func TestValidateConfigurationRejectsNoncanonicalGenerationBeforeSending(t *testing.T) {
	contract := newHTTPContract()
	transport := &validationCaptureTransport{}
	client := &ServiceClient{
		contract:  contract,
		baseURL:   &url.URL{Scheme: "https", Host: "service.example"},
		routes:    map[string]Endpoint{serviceRouteConfigValidate: contract.Service.Endpoints[serviceRouteConfigValidate]},
		transport: transport,
		read:      1,
	}
	if err := client.ValidateConfiguration(context.Background(), "019", []byte(`{}`)); err == nil {
		t.Fatal("noncanonical candidate generation was accepted")
	}
	if transport.request != nil {
		t.Fatal("invalid generation reached the service")
	}
}
