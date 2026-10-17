package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestConfigurationUIDescriptorCanonicalization(t *testing.T) {
	ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{
		{ID: "z-network", Title: ConfigurationUIText{RU: "Сеть", EN: "Network"}, Order: 20, Fields: []ConfigurationUIField{
			{Path: "/routes/*/path", Label: ConfigurationUIText{RU: "Путь", EN: "Path"}, Widget: "text", Order: 20},
			{Path: "/routes", Label: ConfigurationUIText{RU: "Маршруты", EN: "Routes"}, Widget: "array", Order: 10},
		}},
		{ID: "a-general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Order: 10, Fields: []ConfigurationUIField{{
			Path: "/enabled", Label: ConfigurationUIText{RU: "Включено", EN: "Enabled"}, Widget: "auto",
		}}},
	}}
	descriptor := ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui}
	body, err := CanonicalManifest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	var canonical ManifestDescriptor
	if err := json.Unmarshal(body, &canonical); err != nil {
		t.Fatal(err)
	}
	if canonical.ConfigurationUI == nil || canonical.ConfigurationUI.Version != ConfigurationUIVersion {
		t.Fatalf("configuration UI descriptor was not published: %s", body)
	}
	groups := canonical.ConfigurationUI.Groups
	if len(groups) != 2 || groups[0].ID != "a-general" || groups[1].ID != "z-network" {
		t.Fatalf("groups are not deterministic by order and ID: %#v", groups)
	}
	fields := groups[1].Fields
	if len(fields) != 2 || fields[0].Path != "/routes" || fields[1].Path != "/routes/*/path" {
		t.Fatalf("fields are not deterministic by order and path: %#v", fields)
	}
	if strings.Contains(string(body), `"executable"`) || strings.Contains(string(body), `"script"`) {
		t.Fatalf("manifest contains executable UI metadata: %s", body)
	}
}

func TestConfigurationUIValidatesWidgetsAgainstSchema(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"enabled":{"type":"boolean"},"timeout":{"type":"integer","minimum":1,"maximum":60},"routes":{"type":"array","items":{"type":"object","properties":{"path":{"type":"string"}}}}}}`)
	ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
		ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{
			{Path: "/enabled", Label: ConfigurationUIText{RU: "Включено", EN: "Enabled"}, Widget: "toggle"},
			{Path: "/timeout", Label: ConfigurationUIText{RU: "Таймаут", EN: "Timeout"}, Widget: "slider"},
			{Path: "/routes", Label: ConfigurationUIText{RU: "Маршруты", EN: "Routes"}, Widget: "array"},
			{Path: "/routes/*/path", Label: ConfigurationUIText{RU: "Путь", EN: "Path"}, Widget: "text"},
		},
	}}}
	if err := ValidateConfigurationUI(ui, schema); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
}

func TestConfigurationUIResolvesLocalSchemaReferencesAndPeerContracts(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"timeout":{"$ref":"#/$defs/timeout"},"peer":{"type":"string"}},"$defs":{"timeout":{"type":"integer","minimum":1,"maximum":60}}}`)
	ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
		ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{{
			Path: "/timeout", Label: ConfigurationUIText{RU: "Таймаут", EN: "Timeout"}, Widget: "slider",
		}, {
			Path: "/peer", Label: ConfigurationUIText{RU: "Сервис", EN: "Service"}, Widget: "peer", PeerContractID: "echo",
		}},
	}}}
	if err := ValidateConfigurationUI(ui, schema); err != nil {
		t.Fatalf("local schema reference rejected: %v", err)
	}
	manifest := ManifestDescriptor{ID: "server", Version: "1.0.0", ConfigurationUI: ui, PeerContracts: []PeerContractDescriptor{{ID: "echo", Version: "1.0.0"}}}
	if _, err := PrepareManifest(manifest); err != nil {
		t.Fatalf("declared peer contract rejected: %v", err)
	}
	manifest.PeerContracts = nil
	if _, err := PrepareManifest(manifest); !errors.Is(err, ErrInvalidConfigurationUI) {
		t.Fatalf("unknown peer contract did not fail closed: %v", err)
	}
	manifest.PeerContracts = []PeerContractDescriptor{{ID: "echo", Version: "1.0.0"}}
	schemaWithObjectPeer := []byte(`{"type":"object","properties":{"peer":{"type":"object"}}}`)
	objectPeer := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
		ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{{
			Path: "/peer", Label: ConfigurationUIText{RU: "Сервис", EN: "Service"}, Widget: "peer", PeerContractID: "echo",
		}},
	}}}
	if err := ValidateConfigurationUI(objectPeer, schemaWithObjectPeer); !errors.Is(err, ErrInvalidConfigurationUI) {
		t.Fatalf("object peer field must fail closed until a typed object binding contract exists: %v", err)
	}
}

func TestConfigurationUIRejectsUnsupportedOrUnsafeHints(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"name":{"type":"string"},"credential":{"type":"string","writeOnly":true},"auth":{"type":"object","writeOnly":true,"properties":{"token":{"type":"string"}}},"external":{"$ref":"https://example.invalid/schema.json"},"values":{"type":"array","items":{"type":"string"}}}}`)
	valid := func(path, widget string) *ConfigurationUIDescriptor {
		return &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
			ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{{
				Path: path, Label: ConfigurationUIText{RU: "Поле", EN: "Field"}, Widget: widget,
			}},
		}}}
	}
	unsafeCredentialText := valid("/name", "text")
	unsafeCredentialText.Groups[0].Fields[0].Help = &ConfigurationUIText{RU: "Безопасно", EN: "token=example"}
	unsafeAbsolutePath := valid("/name", "text")
	unsafeAbsolutePath.Groups[0].Fields[0].Help = &ConfigurationUIText{RU: "Путь", EN: "Read /Users/alice/key.pem"}
	missingLocalizedTranslation := valid("/name", "text")
	missingLocalizedTranslation.Groups[0].Fields[0].Help = &ConfigurationUIText{RU: "Подсказка"}
	emptyGroup := valid("/name", "text")
	emptyGroup.Groups[0].Fields = nil
	for name, ui := range map[string]*ConfigurationUIDescriptor{
		"unknown version":               {Version: "service-configuration-ui/v2", Groups: valid("/name", "text").Groups},
		"unknown widget":                valid("/name", "custom-js"),
		"unknown path":                  valid("/missing", "text"),
		"secret field":                  valid("/credential", "text"),
		"nested secret field":           valid("/auth/token", "text"),
		"external ref":                  valid("/external", "text"),
		"invalid pointer escape":        valid("/na~2me", "text"),
		"credential in help text":       unsafeCredentialText,
		"absolute path in help text":    unsafeAbsolutePath,
		"missing localized translation": missingLocalizedTranslation,
		"empty group":                   emptyGroup,
		"widget mismatch":               valid("/name", "toggle"),
		"slider without bounds":         valid("/name", "slider"),
		"array mismatch":                valid("/name", "array"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateConfigurationUI(ui, schema); !errors.Is(err, ErrInvalidConfigurationUI) {
				t.Fatalf("expected fail-closed descriptor error, got %v", err)
			}
		})
	}
}

func TestConfigurationUIRejectsSensitiveNamesAndAnnotationsAcrossComposition(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"safeName":{"type":"string"},"tokenValue":{"type":"string"},"credentialValue":{"type":"string","x-sensitive":true},"transport":{"oneOf":[{"type":"object","properties":{"kind":{"const":"http"}}},{"type":"object","properties":{"kind":{"const":"tls"},"keyMaterial":{"type":"string","x-secret":true},"privateKeyRef":{"type":"string"}}}]}}}`)
	for _, path := range []string{"/tokenValue", "/credentialValue", "/transport/keyMaterial", "/transport/privateKeyRef"} {
		t.Run(path, func(t *testing.T) {
			ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
				ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{{
					Path: path, Label: ConfigurationUIText{RU: "Поле", EN: "Field"}, Widget: "auto",
				}},
			}}}
			if err := ValidateConfigurationUI(ui, schema); !errors.Is(err, ErrInvalidConfigurationUI) {
				t.Fatalf("sensitive schema field %q was exposed in UI metadata: %v", path, err)
			}
		})
	}

	safe := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
		ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{{
			Path: "/safeName", Label: ConfigurationUIText{RU: "Имя", EN: "Name"}, Widget: "text",
		}},
	}}}
	if err := ValidateConfigurationUI(safe, schema); err != nil {
		t.Fatalf("safe schema field was rejected: %v", err)
	}
}

func TestConfigurationUIManifestMatchesStableContractFixture(t *testing.T) {
	descriptor := ManifestDescriptor{
		ID: "echo", Version: "1.2.3",
		ConfigurationUI: &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{
			{ID: "response", Title: ConfigurationUIText{RU: "Ответ", EN: "Response"}, Order: 20, Fields: []ConfigurationUIField{{
				Path: "/responseMode", Label: ConfigurationUIText{RU: "Формат ответа", EN: "Response format"}, Widget: "select",
			}}},
			{ID: "request", Title: ConfigurationUIText{RU: "Запрос", EN: "Request"}, Order: 10, Fields: []ConfigurationUIField{
				{Path: "/queryKey", Label: ConfigurationUIText{RU: "Параметр", EN: "Query key"}, Widget: "text", Order: 10},
				{Path: "/maxTextBytes", Label: ConfigurationUIText{RU: "Предел текста", EN: "Text limit"}, Widget: "slider", Order: 20},
			}},
		}},
	}
	actual, err := CanonicalManifest(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/service-configuration-ui-v1-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(actual)) != strings.TrimSpace(string(expected)) {
		t.Fatalf("manifest bytes differ from the stable contract fixture\nactual:   %s\nexpected: %s", actual, expected)
	}
}

func TestConfigurationUIDescriptorEnforcesAdversarialBounds(t *testing.T) {
	baseField := ConfigurationUIField{Path: "/value", Label: ConfigurationUIText{RU: "Значение", EN: "Value"}, Widget: "auto"}
	baseGroup := func(id string) ConfigurationUIGroup {
		return ConfigurationUIGroup{ID: id, Title: ConfigurationUIText{RU: "Группа", EN: "Group"}, Fields: []ConfigurationUIField{baseField}}
	}

	t.Run("maximum group count", func(t *testing.T) {
		groups := make([]ConfigurationUIGroup, configurationUIMaxGroups+1)
		for index := range groups {
			groups[index] = baseGroup(fmt.Sprintf("group-%d", index))
		}
		if err := ValidateConfigurationUI(&ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: groups}, nil); !errors.Is(err, ErrInvalidConfigurationUI) {
			t.Fatalf("expected group bound refusal, got %v", err)
		}
	})

	t.Run("maximum field count", func(t *testing.T) {
		fields := make([]ConfigurationUIField, configurationUIMaxFields+1)
		for index := range fields {
			fields[index] = baseField
			fields[index].Path = fmt.Sprintf("/value-%d", index)
		}
		group := baseGroup("general")
		group.Fields = fields
		if err := ValidateConfigurationUI(&ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{group}}, nil); !errors.Is(err, ErrInvalidConfigurationUI) {
			t.Fatalf("expected field bound refusal, got %v", err)
		}
	})

	t.Run("variant count", func(t *testing.T) {
		branches := make([]any, 65)
		for index := range branches {
			branches[index] = map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}}
		}
		schema, err := json.Marshal(map[string]any{"oneOf": branches})
		if err != nil {
			t.Fatal(err)
		}
		ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
			ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{baseField},
		}}}
		if err := ValidateConfigurationUI(ui, schema); !errors.Is(err, ErrInvalidConfigurationUI) {
			t.Fatalf("expected variant bound refusal, got %v", err)
		}
	})

	t.Run("composition depth", func(t *testing.T) {
		var schemaValue any = map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}}
		for range 40 {
			schemaValue = map[string]any{"allOf": []any{schemaValue}}
		}
		schema, err := json.Marshal(schemaValue)
		if err != nil {
			t.Fatal(err)
		}
		ui := &ConfigurationUIDescriptor{Version: ConfigurationUIVersion, Groups: []ConfigurationUIGroup{{
			ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: []ConfigurationUIField{baseField},
		}}}
		if err := ValidateConfigurationUI(ui, schema); !errors.Is(err, ErrInvalidConfigurationUI) {
			t.Fatalf("expected schema depth refusal, got %v", err)
		}
	})
}

func TestCanonicalManifestRejectsOversizedConfigurationUIDescriptor(t *testing.T) {
	longText := strings.Repeat("x", 2048)
	fields := make([]ConfigurationUIField, 48)
	for index := range fields {
		fields[index] = ConfigurationUIField{
			Path:   fmt.Sprintf("/value-%d", index),
			Label:  ConfigurationUIText{RU: longText, EN: longText},
			Help:   &ConfigurationUIText{RU: longText, EN: longText},
			Widget: "text",
		}
	}
	descriptor := ManifestDescriptor{
		ID: "echo", Version: "1.0.0",
		ConfigurationUI: &ConfigurationUIDescriptor{
			Version: ConfigurationUIVersion,
			Groups: []ConfigurationUIGroup{{
				ID: "general", Title: ConfigurationUIText{RU: "Общие", EN: "General"}, Fields: fields,
			}},
		},
	}
	if _, err := CanonicalManifest(descriptor); !errors.Is(err, ErrManifestTooLarge) {
		t.Fatalf("expected manifest size refusal, got %v", err)
	}
}
