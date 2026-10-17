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
		Resources: []ResourceDescriptor{
			{ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin, PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId"},
			{ID: "project-files", Kind: "file", Provider: ResourceProviderProject},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	value := string(body)
	if !strings.Contains(value, `"manifestVersion":"service-runtime/v1"`) || !strings.Contains(value, `"sdkVersion":"service-sdk/v2"`) || !strings.Contains(value, `"protocolVersion":"wool/v3"`) {
		t.Fatalf("defaults missing: %s", value)
	}
	if strings.Index(value, `"id":"a"`) > strings.Index(value, `"id":"z"`) {
		t.Fatalf("capabilities were not canonicalized: %s", value)
	}
	if strings.Index(value, `"id":"project-files"`) > strings.Index(value, `"id":"sites"`) {
		t.Fatalf("resources were not canonicalized by ID: %s", value)
	}
}

func TestManifestResourceCatalogIsTypedBoundedAndDefensivelyCopied(t *testing.T) {
	descriptor := ManifestDescriptor{ID: "server", Version: "1.0.0", Resources: []ResourceDescriptor{{
		ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin,
		PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId",
	}}}
	prepared, err := PrepareManifest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	descriptor.Resources[0].PageID = "changed"
	if prepared.Resources[0].PageID != "sites" {
		t.Fatal("prepared manifest aliases the caller-owned resource descriptors")
	}
	if _, err := CanonicalManifest(prepared); err != nil {
		t.Fatalf("valid resource catalog was rejected: %v", err)
	}
}

func TestManifestResourcePublicationIsExplicitTypedAndDefensivelyCopied(t *testing.T) {
	publication := &ResourcePublicationDescriptor{PageID: "sites", ActionID: "publish"}
	descriptor := ManifestDescriptor{ID: "server", Version: "1.0.0", Resources: []ResourceDescriptor{{
		ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin,
		PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId",
		Publication: publication,
	}}}
	prepared, err := PrepareManifest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	publication.ActionID = "changed"
	if prepared.Resources[0].Publication == nil || prepared.Resources[0].Publication.ActionID != "publish" {
		t.Fatal("prepared manifest aliases the caller-owned resource publication")
	}
	body, err := CanonicalManifest(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"publication":{"pageId":"sites","actionId":"publish"}`) {
		t.Fatalf("canonical manifest omitted the resource publication binding: %s", body)
	}
}

func TestManifestRejectsInvalidResourcePublicationBindings(t *testing.T) {
	base := ResourceDescriptor{
		ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin,
		PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId",
	}
	tests := map[string]ResourceDescriptor{
		"missing page": {ID: base.ID, Kind: base.Kind, Provider: base.Provider, PageID: base.PageID, QueryResource: base.QueryResource,
			ValuePointer: base.ValuePointer, LabelPointer: base.LabelPointer, Publication: &ResourcePublicationDescriptor{ActionID: "publish"}},
		"missing action": {ID: base.ID, Kind: base.Kind, Provider: base.Provider, PageID: base.PageID, QueryResource: base.QueryResource,
			ValuePointer: base.ValuePointer, LabelPointer: base.LabelPointer, Publication: &ResourcePublicationDescriptor{PageID: "sites"}},
		"unsafe action reference": {ID: base.ID, Kind: base.Kind, Provider: base.Provider, PageID: base.PageID, QueryResource: base.QueryResource,
			ValuePointer: base.ValuePointer, LabelPointer: base.LabelPointer, Publication: &ResourcePublicationDescriptor{PageID: "sites", ActionID: "../publish"}},
		"project resource cannot publish through service admin": {ID: "project-files", Kind: "file", Provider: ResourceProviderProject,
			Publication: &ResourcePublicationDescriptor{PageID: "sites", ActionID: "publish"}},
	}
	for name, resource := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0", Resources: []ResourceDescriptor{resource}}); err == nil {
				t.Fatal("accepted an invalid resource publication binding")
			}
		})
	}
}

func TestManifestRejectsInvalidResourceCatalogs(t *testing.T) {
	valid := ResourceDescriptor{ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin, PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId"}
	tests := map[string][]ResourceDescriptor{
		"duplicate IDs":                 {valid, valid},
		"unknown kind":                  {{ID: "files", Kind: "directory", Provider: ResourceProviderProject}},
		"unknown provider":              {{ID: "sites", Kind: "site", Provider: "http"}},
		"query traversal":               {{ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin, PageID: "../sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId"}},
		"missing query mapping":         {{ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin}},
		"project provider query fields": {{ID: "sites", Kind: "site", Provider: ResourceProviderProject, PageID: "sites"}},
		"sensitive field pointer":       {{ID: "certificates", Kind: "certificate", Provider: ResourceProviderServiceAdmin, PageID: "certificates", QueryResource: "certificates", ValuePointer: "/privateKeyRef", LabelPointer: "/domain"}},
	}
	for name, resources := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0", Resources: resources}); err == nil {
				t.Fatal("accepted invalid resource descriptor")
			}
		})
	}
}

func TestManifestConfigurationUIResourceMustResolveDeclaredCatalog(t *testing.T) {
	ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
		ID: "routes", Title: ConfigurationUIText{RU: "Маршруты", EN: "Routes"}, Fields: []ConfigurationUIField{{
			Path: "/siteId", Label: ConfigurationUIText{RU: "Сайт", EN: "Site"}, Widget: "resource", ResourceKind: "site", ResourceID: "sites",
		}},
	}}}
	resource := ResourceDescriptor{ID: "sites", Kind: "site", Provider: ResourceProviderServiceAdmin, PageID: "sites", QueryResource: "sites", ValuePointer: "/siteId", LabelPointer: "/siteId"}
	if _, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui, Resources: []ResourceDescriptor{resource}}); err != nil {
		t.Fatalf("matching resource field and catalog were rejected: %v", err)
	}
	if _, err := CanonicalManifest(ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui}); err == nil {
		t.Fatal("accepted a resource field without a declared catalog")
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
		Runtime: RuntimeDescriptor{HealthEndpoint: "/service/health"},
	})
	if err == nil {
		t.Fatal("accepted a product-specific health route; Service SDK must own GET /healthz")
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

func TestCanonicalJSONObjectRejectsDuplicateKeysAtEveryDepth(t *testing.T) {
	for _, document := range []string{
		`{"type":"string","type":"number"}`,
		`{"properties":{"name":{"type":"string","type":"number"}}}`,
	} {
		t.Run(document, func(t *testing.T) {
			if _, err := CanonicalJSONObject([]byte(document)); err == nil {
				t.Fatal("accepted duplicate JSON keys with ambiguous schema semantics")
			}
		})
	}
}

func TestCanonicalJSONObjectRejectsInvalidUTF8(t *testing.T) {
	document := append([]byte(`{"title":"`), 0xff)
	document = append(document, []byte(`"}`)...)
	if _, err := CanonicalJSONObject(document); err == nil {
		t.Fatal("accepted invalid UTF-8 that JSON canonicalization would rewrite")
	}
}

func TestCanonicalJSONObjectRejectsOversizedRawDocument(t *testing.T) {
	document := []byte(`{"value":"` + strings.Repeat("x", manifestMaximumBytes) + `"}`)
	if _, err := CanonicalJSONObject(document); !errors.Is(err, ErrManifestTooLarge) {
		t.Fatalf("expected raw document size refusal, got %v", err)
	}
}
