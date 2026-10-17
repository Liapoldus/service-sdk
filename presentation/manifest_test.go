package presentation

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

func TestManifestProviderReturnsDefensiveCopies(t *testing.T) {
	provider, err := NewManifestProvider(models.ManifestDescriptor{ID: "echo", Version: "1.0.0"}, []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manifest[0] = 'x'
	again, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again[0] == 'x' {
		t.Fatal("provider exposed mutable manifest bytes")
	}
}

func TestManifestProviderValidatesAndPublishesConfigurationUI(t *testing.T) {
	ui := &models.ConfigurationUIDescriptor{Version: models.ConfigurationUIVersion, Groups: []models.ConfigurationUIGroup{{
		ID: "general", Title: models.ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []models.ConfigurationUIField{{
			Path: "/timeout", Label: models.ConfigurationUIText{RU: "Таймаут", EN: "Timeout"}, Widget: "slider",
		}},
	}}}
	provider, err := NewManifestProvider(models.ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui}, []byte(`{"type":"object","properties":{"timeout":{"type":"integer","minimum":1,"maximum":60}}}`))
	if err != nil {
		t.Fatalf("construct manifest provider: %v", err)
	}
	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"configurationUI":{"version":"service-configuration-ui/v1"`) {
		t.Fatalf("configuration UI descriptor missing from live manifest: %s", manifest)
	}
	ui.Groups[0].Fields[0].Widget = "toggle"
	if _, err := NewManifestProvider(models.ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui}, []byte(`{"type":"object","properties":{"timeout":{"type":"integer"}}}`)); !errors.Is(err, models.ErrInvalidConfigurationUI) {
		t.Fatalf("schema-incompatible widget did not fail closed: %v", err)
	}
}

func TestManifestAndSchemaMetadataAreServedByteForByte(t *testing.T) {
	provider, err := NewManifestProvider(models.ManifestDescriptor{
		ID: "echo", Version: "1.0.0",
		ConfigurationUI: &models.ConfigurationUIDescriptor{Version: models.ConfigurationUIVersion, Groups: []models.ConfigurationUIGroup{{
			ID: "general", Title: models.ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []models.ConfigurationUIField{{
				Path: "/enabled", Label: models.ConfigurationUIText{RU: "Включено", EN: "Enabled"}, Widget: "toggle",
			}},
		}}},
	}, []byte("{\n  \"type\": \"object\",\n  \"properties\": {\"enabled\": {\"type\": \"boolean\"}}\n}"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := provider.Manifest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	schema, err := provider.ConfigurationSchema(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	set := &HandlerSet{contracts: Contracts{MaximumMetadataBytes: 1 << 18}}
	for _, test := range []struct {
		name     string
		fetch    func(context.Context) ([]byte, error)
		expected []byte
	}{
		{name: "manifest", fetch: provider.Manifest, expected: manifest},
		{name: "configuration schema", fetch: provider.ConfigurationSchema, expected: schema},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			set.serveMetadata(response, request, test.fetch, DocumentContract{MediaType: "application/json", MaximumBytes: 1 << 18})
			if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), test.expected) {
				t.Fatalf("metadata bytes changed in the REST presentation path: status=%d\nactual:   %q\nexpected: %q", response.Code, response.Body.Bytes(), test.expected)
			}
		})
	}
}
