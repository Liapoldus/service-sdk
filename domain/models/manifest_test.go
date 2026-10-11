package models

import (
	"errors"
	"strings"
	"testing"
)

func TestCanonicalManifestAppliesDefaultsAndSortsCollections(t *testing.T) {
	body, err := CanonicalManifest(ManifestDescriptor{
		ID: "server", Version: "1.0.0",
		Capabilities:  []CapabilityDescriptor{{ID: "z", Version: "1.0.0"}, {ID: "a", Version: "1.0.0"}},
		PeerContracts: []PeerContractDescriptor{{ID: "echo", Version: "1.0.0", Methods: []string{"z", "a"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	value := string(body)
	if !strings.Contains(value, `"manifestVersion":"studio-runtime/v1"`) || !strings.Contains(value, `"sdkVersion":"plugin-sdk/v2"`) {
		t.Fatalf("defaults missing: %s", value)
	}
	if strings.Index(value, `"id":"a"`) > strings.Index(value, `"id":"z"`) {
		t.Fatalf("capabilities were not canonicalized: %s", value)
	}
}

func TestCanonicalManifestRejectsSensitiveRuntimeValue(t *testing.T) {
	_, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0", Runtime: RuntimeDescriptor{ControlEndpoint: "https://host/token/secret"}})
	if !errors.Is(err, ErrManifestContainsSecret) {
		t.Fatalf("expected secret rejection, got %v", err)
	}
}

func TestCanonicalManifestAllowsHTTPRoutePaths(t *testing.T) {
	_, err := CanonicalManifest(ManifestDescriptor{
		ID: "echo", Version: "1.0.0",
		Runtime: RuntimeDescriptor{HealthEndpoint: "/healthz", ReadinessEndpoint: "/_liapoldus/v1/ready"},
	})
	if err != nil {
		t.Fatalf("expected HTTP route paths to be allowed: %v", err)
	}
}

func TestCanonicalManifestOwnsUniversalHealthEndpoint(t *testing.T) {
	_, err := CanonicalManifest(ManifestDescriptor{
		ID: "server", Version: "1.0.0",
		Runtime: RuntimeDescriptor{HealthEndpoint: "/plugin/health"},
	})
	if err == nil {
		t.Fatal("accepted a product-specific health route; Plugin SDK must own GET /healthz")
	}

	body, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"healthEndpoint":"/healthz"`) {
		t.Fatalf("SDK manifest omitted its universal health endpoint: %s", body)
	}
}

func TestCanonicalJSONObjectRejectsNonObject(t *testing.T) {
	if _, err := CanonicalJSONObject([]byte(`[1]`)); err == nil {
		t.Fatal("expected object validation error")
	}
}
