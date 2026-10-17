package presentation

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
)

// ErrInvalidObservabilityConfiguration means the SDK-owned health and metrics
// surface was not composed from the versioned SDK contract.
var ErrInvalidObservabilityConfiguration = errors.New("invalid Service SDK observability configuration")

// ObservabilityConfiguration describes the universal SDK endpoints that may be
// mounted on a service's operator-facing listener. The product service supplies
// no handler logic: health and metrics are rendered by the SDK from these
// contract values and the SDK metrics collector.
type ObservabilityConfiguration struct {
	HealthEndpoint   Endpoint
	MetricsEndpoint  Endpoint
	HealthStatus     int
	HealthBody       map[string]string
	ContentTypes     ContentTypes
	Metrics          MetricsExposition
	NotFound         Problem
	MethodNotAllowed Problem
	InternalError    Problem
}

// NewObservabilityHandler returns the SDK-owned universal healthz and metrics
// handler. It is deliberately independent from the Core mTLS handler set so a
// product's public listener can expose the same safe endpoints without
// duplicating their implementation. No lifecycle, configuration, manifest or
// Core endpoint is exposed here.
func NewObservabilityHandler(configuration ObservabilityConfiguration) (http.Handler, error) {
	if configuration.HealthEndpoint != (Endpoint{Method: http.MethodGet, Path: "/healthz"}) ||
		configuration.MetricsEndpoint != (Endpoint{Method: http.MethodGet, Path: "/metrics"}) ||
		configuration.HealthStatus != http.StatusOK || len(configuration.HealthBody) != 1 || configuration.HealthBody["status"] != "ok" ||
		configuration.ContentTypes.JSON != "application/json" ||
		configuration.ContentTypes.Metrics != "text/plain; version=0.0.4; charset=utf-8" || configuration.Metrics == nil ||
		configuration.Metrics.ContentType() != configuration.ContentTypes.Metrics ||
		!validProblem(configuration.NotFound) || !validProblem(configuration.MethodNotAllowed) ||
		!validProblem(configuration.InternalError) {
		return nil, ErrInvalidObservabilityConfiguration
	}
	for key, value := range configuration.HealthBody {
		if key == "" || value == "" {
			return nil, ErrInvalidObservabilityConfiguration
		}
	}

	handler := &observabilityHandler{configuration: configuration}
	mux := http.NewServeMux()
	mux.HandleFunc(configuration.HealthEndpoint.Path, handler.serveHealth)
	mux.HandleFunc(configuration.MetricsEndpoint.Path, handler.serveMetrics)
	mux.HandleFunc("/", handler.serveNotFound)
	return mux, nil
}

type observabilityHandler struct {
	configuration ObservabilityConfiguration
}

func (handler *observabilityHandler) serveHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != handler.configuration.HealthEndpoint.Method {
		handler.writeProblem(writer, handler.configuration.MethodNotAllowed, handler.configuration.HealthEndpoint.Method)
		return
	}
	handler.writeJSON(writer, handler.configuration.ContentTypes.JSON, handler.configuration.HealthStatus, handler.configuration.HealthBody)
}

func (handler *observabilityHandler) serveMetrics(writer http.ResponseWriter, request *http.Request) {
	if request.Method != handler.configuration.MetricsEndpoint.Method {
		handler.writeProblem(writer, handler.configuration.MethodNotAllowed, handler.configuration.MetricsEndpoint.Method)
		return
	}
	var body bytes.Buffer
	if err := handler.configuration.Metrics.WriteText(&body); err != nil {
		handler.writeProblem(writer, handler.configuration.InternalError, "")
		return
	}
	writer.Header().Set("Content-Type", handler.configuration.ContentTypes.Metrics)
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(body.Bytes()); err != nil {
		return
	}
}

func (handler *observabilityHandler) serveNotFound(writer http.ResponseWriter, request *http.Request) {
	handler.writeProblem(writer, handler.configuration.NotFound, "")
}

func (handler *observabilityHandler) writeJSON(writer http.ResponseWriter, contentType string, status int, value any) {
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		return
	}
}

func (handler *observabilityHandler) writeProblem(writer http.ResponseWriter, problem Problem, allow string) {
	if allow != "" {
		writer.Header().Set("Allow", allow)
	}
	handler.writeJSON(writer, handler.configuration.ContentTypes.JSON, problem.Status, map[string]string{"code": problem.Code})
}

func validProblem(problem Problem) bool {
	return problem.Status >= 100 && problem.Status <= 599 && problem.Code != ""
}
