package presentation

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testMetrics struct{}

func (testMetrics) ContentType() string { return "text/plain; version=0.0.4; charset=utf-8" }
func (testMetrics) WriteText(writer io.Writer) error {
	_, err := io.WriteString(writer, "liapoldus_plugin_ready 1\n")
	return err
}

func TestNewObservabilityHandlerProvidesUniversalHealthzAndMetrics(t *testing.T) {
	handler, err := NewObservabilityHandler(ObservabilityConfiguration{
		HealthEndpoint:  Endpoint{Method: http.MethodGet, Path: "/healthz"},
		MetricsEndpoint: Endpoint{Method: http.MethodGet, Path: "/metrics"},
		HealthStatus:    http.StatusOK, HealthBody: map[string]string{"status": "ok"},
		ContentTypes: ContentTypes{JSON: "application/json", Metrics: (testMetrics{}).ContentType()}, Metrics: testMetrics{},
		NotFound:         Problem{Status: http.StatusNotFound, Code: "not_found"},
		MethodNotAllowed: Problem{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed"},
		InternalError:    Problem{Status: http.StatusInternalServerError, Code: "internal_error"},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()
	healthRequest, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(healthRequest)
	if err != nil {
		t.Fatal(err)
	}
	healthBody, readErr := io.ReadAll(response.Body)
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/json" || string(healthBody) != "{\"status\":\"ok\"}\n" {
		t.Fatalf("healthz: status=%d type=%q body=%q err=%v", response.StatusCode, response.Header.Get("Content-Type"), healthBody, readErr)
	}

	metricsRequest, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.DefaultClient.Do(metricsRequest)
	if err != nil {
		t.Fatal(err)
	}
	metricsBody, readErr := io.ReadAll(response.Body)
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != (testMetrics{}).ContentType() || !bytes.Equal(metricsBody, []byte("liapoldus_plugin_ready 1\n")) {
		t.Fatalf("metrics: status=%d type=%q body=%q err=%v", response.StatusCode, response.Header.Get("Content-Type"), metricsBody, readErr)
	}
}

func TestNewObservabilityHandlerRejectsPluginSpecificRoutesAndResponses(t *testing.T) {
	base := ObservabilityConfiguration{
		HealthEndpoint:  Endpoint{Method: http.MethodGet, Path: "/healthz"},
		MetricsEndpoint: Endpoint{Method: http.MethodGet, Path: "/metrics"},
		HealthStatus:    http.StatusOK, HealthBody: map[string]string{"status": "ok"},
		ContentTypes: ContentTypes{JSON: "application/json", Metrics: (testMetrics{}).ContentType()}, Metrics: testMetrics{},
		NotFound:         Problem{Status: http.StatusNotFound, Code: "not_found"},
		MethodNotAllowed: Problem{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed"},
		InternalError:    Problem{Status: http.StatusInternalServerError, Code: "internal_error"},
	}
	for _, test := range []struct {
		name   string
		change func(*ObservabilityConfiguration)
	}{
		{name: "health path", change: func(value *ObservabilityConfiguration) { value.HealthEndpoint.Path = "/plugin/health" }},
		{name: "metrics path", change: func(value *ObservabilityConfiguration) { value.MetricsEndpoint.Path = "/plugin/metrics" }},
		{name: "health body", change: func(value *ObservabilityConfiguration) { value.HealthBody = map[string]string{"healthy": "true"} }},
		{name: "metrics media type", change: func(value *ObservabilityConfiguration) { value.ContentTypes.Metrics = "application/custom" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			configuration := base
			test.change(&configuration)
			if _, err := NewObservabilityHandler(configuration); err == nil {
				t.Fatal("accepted a product-specific observability surface")
			}
		})
	}
}
