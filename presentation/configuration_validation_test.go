package presentation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type validationMetadata struct {
	received []byte
	rejected bool
}

func (*validationMetadata) Manifest(context.Context) ([]byte, error) { return []byte(`{}`), nil }
func (*validationMetadata) ConfigurationSchema(context.Context) ([]byte, error) {
	return []byte(`{}`), nil
}
func (metadata *validationMetadata) ValidateConfiguration(_ context.Context, raw []byte) error {
	metadata.received = append([]byte(nil), raw...)
	if metadata.rejected {
		return errors.New("private service rejection")
	}
	return nil
}

func TestConfigurationValidationPreservesBytesAndPublishesNoGeneration(t *testing.T) {
	metadata := &validationMetadata{}
	set := HandlerSet{metadata: metadata, contracts: Contracts{
		ContentTypes: ContentTypes{JSON: "application/json"}, MaximumMetadataBytes: 1024, ReadinessDeadline: time.Second,
		OutcomeProblems: map[string]string{"applyRejected": "rejected", "invalidRequest": "invalid"},
		Errors:          map[string]Problem{"rejected": {Status: 422, Code: "apply_rejected"}, "invalid": {Status: 400, Code: "invalid_request"}},
	}}
	for _, rejected := range []bool{false, true} {
		metadata.rejected = rejected
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/validation", strings.NewReader("{ \"value\": 1 }"))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		set.handleConfigurationValidation(response, request)
		if string(metadata.received) != "{ \"value\": 1 }" {
			t.Fatal("candidate bytes changed")
		}
		if !rejected && response.Code != http.StatusNoContent {
			t.Fatalf("success=%d", response.Code)
		}
		if rejected && (response.Code < 400 || strings.Contains(response.Body.String(), "private")) {
			t.Fatalf("unsafe refusal: %d %s", response.Code, response.Body.String())
		}
	}
}
