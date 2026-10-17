package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// ManifestVersion is the version of the SDK-owned runtime manifest envelope.
const ManifestVersion = "service-runtime/v1"

// SDKVersion is the version advertised by the SDK itself. Product services do
// not need to duplicate this value in their composition roots.
const SDKVersion = "service-sdk/v2"

// DefaultProtocolVersion is intentionally a string rather than a dependency on
// wool: the SDK publishes opaque peer contract metadata and remains
// usable by services that select a compatible transport implementation.
const DefaultProtocolVersion = "wool/v3"

const (
	manifestMaximumBytes   = 262144
	identifierMaximum      = 128
	versionMaximum         = 64
	resourceCatalogMaximum = 256
)

var (
	ErrInvalidManifest        = errors.New("invalid Service SDK manifest")
	ErrManifestTooLarge       = errors.New("service SDK manifest exceeds the safety limit")
	ErrManifestContainsSecret = errors.New("service SDK manifest contains a secret or filesystem path")
)

// ManifestDescriptor is the product-owned part of the SDK runtime manifest.
// The SDK fills ManifestVersion and SDKVersion and canonicalizes the resulting
// document before it is exposed on the REST endpoint.
type ManifestDescriptor struct {
	ID              string                     `json:"id"`
	Version         string                     `json:"version"`
	SDKVersion      string                     `json:"sdkVersion"`
	ProtocolVersion string                     `json:"protocolVersion"`
	ManifestVersion string                     `json:"manifestVersion"`
	Capabilities    []CapabilityDescriptor     `json:"capabilities,omitempty"`
	PeerContracts   []PeerContractDescriptor   `json:"peerContracts,omitempty"`
	Resources       []ResourceDescriptor       `json:"resources,omitempty"`
	ConfigurationUI *ConfigurationUIDescriptor `json:"configurationUI,omitempty"`
	Runtime         RuntimeDescriptor          `json:"runtime"`
}

const (
	ResourceProviderProject      = "project"
	ResourceProviderServiceAdmin = "service-admin"
)

// ResourceDescriptor is a safe, declarative catalog binding. It contains no
// executable URL or filesystem path; Studio resolves values through its typed
// host API and CLI → Core Management API boundary.
type ResourceDescriptor struct {
	ID            string                         `json:"id"`
	Kind          string                         `json:"kind"`
	Provider      string                         `json:"provider"`
	PageID        string                         `json:"pageId,omitempty"`
	QueryResource string                         `json:"queryResource,omitempty"`
	ValuePointer  string                         `json:"valuePointer,omitempty"`
	LabelPointer  string                         `json:"labelPointer,omitempty"`
	Publication   *ResourcePublicationDescriptor `json:"publication,omitempty"`
}

// ResourcePublicationDescriptor points to a service-owned artifact action in
// the Core-backed Admin Surface. Consumers must verify that the live surface
// still declares the referenced action as an artifact action before use; this
// descriptor never contains an endpoint, transport address, or permission.
type ResourcePublicationDescriptor struct {
	PageID   string `json:"pageId"`
	ActionID string `json:"actionId"`
}

type CapabilityDescriptor struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// PeerContractDescriptor deliberately carries only opaque contract references.
// Core and wool do not interpret request/response product payloads.
type PeerContractDescriptor struct {
	ID                string   `json:"id"`
	Version           string   `json:"version"`
	Methods           []string `json:"methods,omitempty"`
	RequestSchemaRef  string   `json:"requestSchemaRef,omitempty"`
	ResponseSchemaRef string   `json:"responseSchemaRef,omitempty"`
}

type RuntimeDescriptor struct {
	ControlEndpoint   string `json:"controlEndpoint,omitempty"`
	PeerEndpoint      string `json:"peerEndpoint,omitempty"`
	HealthEndpoint    string `json:"healthEndpoint,omitempty"`
	ReadinessEndpoint string `json:"readinessEndpoint,omitempty"`
}

// PrepareManifest applies SDK-owned defaults and returns a defensive copy.
func PrepareManifest(descriptor ManifestDescriptor) (ManifestDescriptor, error) {
	descriptor.SDKVersion = SDKVersion
	descriptor.ManifestVersion = ManifestVersion
	if descriptor.Runtime.HealthEndpoint != "" && descriptor.Runtime.HealthEndpoint != "/healthz" {
		return ManifestDescriptor{}, fmt.Errorf("%w: health endpoint is SDK-owned", ErrInvalidManifest)
	}
	descriptor.Runtime.HealthEndpoint = "/healthz"
	if strings.TrimSpace(descriptor.ProtocolVersion) == "" {
		descriptor.ProtocolVersion = DefaultProtocolVersion
	}
	if err := descriptor.Validate(); err != nil {
		return ManifestDescriptor{}, err
	}
	prepared := descriptor
	prepared.Capabilities = append([]CapabilityDescriptor(nil), descriptor.Capabilities...)
	prepared.PeerContracts = append([]PeerContractDescriptor(nil), descriptor.PeerContracts...)
	prepared.Resources = append([]ResourceDescriptor(nil), descriptor.Resources...)
	for index, resource := range descriptor.Resources {
		if resource.Publication != nil {
			publication := *resource.Publication
			prepared.Resources[index].Publication = &publication
		}
	}
	if descriptor.ConfigurationUI != nil {
		configurationUI := *descriptor.ConfigurationUI
		configurationUI.Groups = append([]ConfigurationUIGroup(nil), descriptor.ConfigurationUI.Groups...)
		for index := range configurationUI.Groups {
			if descriptor.ConfigurationUI.Groups[index].Description != nil {
				description := *descriptor.ConfigurationUI.Groups[index].Description
				configurationUI.Groups[index].Description = &description
			}
			configurationUI.Groups[index].Fields = append([]ConfigurationUIField(nil), descriptor.ConfigurationUI.Groups[index].Fields...)
			for fieldIndex := range configurationUI.Groups[index].Fields {
				original := descriptor.ConfigurationUI.Groups[index].Fields[fieldIndex]
				if original.Help != nil {
					help := *original.Help
					configurationUI.Groups[index].Fields[fieldIndex].Help = &help
				}
				if original.Unit != nil {
					unit := *original.Unit
					configurationUI.Groups[index].Fields[fieldIndex].Unit = &unit
				}
			}
		}
		sort.Slice(configurationUI.Groups, func(left, right int) bool {
			if configurationUI.Groups[left].Order != configurationUI.Groups[right].Order {
				return configurationUI.Groups[left].Order < configurationUI.Groups[right].Order
			}
			return configurationUI.Groups[left].ID < configurationUI.Groups[right].ID
		})
		for index := range configurationUI.Groups {
			fields := configurationUI.Groups[index].Fields
			sort.Slice(fields, func(left, right int) bool {
				if fields[left].Order != fields[right].Order {
					return fields[left].Order < fields[right].Order
				}
				return fields[left].Path < fields[right].Path
			})
		}
		prepared.ConfigurationUI = &configurationUI
	}
	for index := range prepared.PeerContracts {
		prepared.PeerContracts[index].Methods = append([]string(nil), descriptor.PeerContracts[index].Methods...)
	}
	sort.Slice(prepared.Capabilities, func(left, right int) bool { return prepared.Capabilities[left].ID < prepared.Capabilities[right].ID })
	sort.Slice(prepared.PeerContracts, func(left, right int) bool { return prepared.PeerContracts[left].ID < prepared.PeerContracts[right].ID })
	sort.Slice(prepared.Resources, func(left, right int) bool { return prepared.Resources[left].ID < prepared.Resources[right].ID })
	for index := range prepared.PeerContracts {
		sort.Strings(prepared.PeerContracts[index].Methods)
	}
	return prepared, nil
}

func (descriptor ManifestDescriptor) Validate() error {
	if !validManifestIdentifier(descriptor.ID) || !validVersion(descriptor.Version) ||
		!validVersion(descriptor.SDKVersion) || !validVersion(descriptor.ProtocolVersion) ||
		descriptor.ManifestVersion != ManifestVersion {
		return fmt.Errorf("%w: required versioned identity is invalid", ErrInvalidManifest)
	}
	seen := map[string]struct{}{}
	for _, capability := range descriptor.Capabilities {
		if !validManifestIdentifier(capability.ID) || !validVersion(capability.Version) {
			return fmt.Errorf("%w: capability is invalid", ErrInvalidManifest)
		}
		if _, exists := seen[capability.ID]; exists {
			return fmt.Errorf("%w: duplicate capability %q", ErrInvalidManifest, capability.ID)
		}
		seen[capability.ID] = struct{}{}
	}
	seen = map[string]struct{}{}
	for _, contract := range descriptor.PeerContracts {
		if !validManifestIdentifier(contract.ID) || !validVersion(contract.Version) || len(contract.Methods) > 64 ||
			!validReference(contract.RequestSchemaRef) || !validReference(contract.ResponseSchemaRef) {
			return fmt.Errorf("%w: peer contract is invalid", ErrInvalidManifest)
		}
		if _, exists := seen[contract.ID]; exists {
			return fmt.Errorf("%w: duplicate peer contract %q", ErrInvalidManifest, contract.ID)
		}
		seen[contract.ID] = struct{}{}
		methodSet := map[string]struct{}{}
		for _, method := range contract.Methods {
			if !validManifestIdentifier(method) {
				return fmt.Errorf("%w: peer method is invalid", ErrInvalidManifest)
			}
			if _, exists := methodSet[method]; exists {
				return fmt.Errorf("%w: duplicate peer method %q", ErrInvalidManifest, method)
			}
			methodSet[method] = struct{}{}
		}
	}
	if len(descriptor.Resources) > resourceCatalogMaximum {
		return fmt.Errorf("%w: resource catalog exceeds the safety limit", ErrInvalidManifest)
	}
	resourceIDs := make(map[string]string, len(descriptor.Resources))
	for _, resource := range descriptor.Resources {
		if !validManifestIdentifier(resource.ID) || !validResourceKind(resource.Kind) {
			return fmt.Errorf("%w: resource identity is invalid", ErrInvalidManifest)
		}
		if _, exists := resourceIDs[resource.ID]; exists {
			return fmt.Errorf("%w: duplicate resource ID %q", ErrInvalidManifest, resource.ID)
		}
		resourceIDs[resource.ID] = resource.Kind
		switch resource.Provider {
		case ResourceProviderProject:
			if resource.PageID != "" || resource.QueryResource != "" || resource.ValuePointer != "" || resource.LabelPointer != "" || resource.Publication != nil {
				return fmt.Errorf("%w: project resource cannot declare a remote query", ErrInvalidManifest)
			}
		case ResourceProviderServiceAdmin:
			if !validManifestIdentifier(resource.PageID) || !validManifestIdentifier(resource.QueryResource) ||
				!validConfigurationPath(resource.ValuePointer) || !validConfigurationPath(resource.LabelPointer) ||
				containsSensitiveValue(resource.ValuePointer) || containsSensitiveValue(resource.LabelPointer) {
				return fmt.Errorf("%w: service resource query mapping is invalid", ErrInvalidManifest)
			}
			if resource.Publication != nil && (!validManifestIdentifier(resource.Publication.PageID) || !validManifestIdentifier(resource.Publication.ActionID)) {
				return fmt.Errorf("%w: service resource publication mapping is invalid", ErrInvalidManifest)
			}
		default:
			return fmt.Errorf("%w: resource provider is unsupported", ErrInvalidManifest)
		}
	}
	if err := validateConfigurationUIDescriptor(descriptor.ConfigurationUI); err != nil {
		return err
	}
	if descriptor.ConfigurationUI != nil {
		contracts := make(map[string]bool, len(descriptor.PeerContracts))
		for _, contract := range descriptor.PeerContracts {
			contracts[contract.ID] = true
		}
		for _, group := range descriptor.ConfigurationUI.Groups {
			for _, field := range group.Fields {
				if field.Widget == "peer" && !contracts[field.PeerContractID] {
					return fmt.Errorf("%w: unknown peer contract %q", ErrInvalidConfigurationUI, field.PeerContractID)
				}
				if field.Widget == "resource" && resourceIDs[field.ResourceID] != field.ResourceKind {
					return fmt.Errorf("%w: unknown resource catalog %q", ErrInvalidConfigurationUI, field.ResourceID)
				}
			}
		}
	}
	for _, value := range []string{descriptor.Runtime.ControlEndpoint, descriptor.Runtime.PeerEndpoint} {
		if containsSensitiveValue(value) || strings.HasPrefix(value, "/") {
			return ErrManifestContainsSecret
		}
	}
	for _, value := range []string{descriptor.Runtime.HealthEndpoint, descriptor.Runtime.ReadinessEndpoint} {
		if containsSensitiveRoute(value) {
			return ErrManifestContainsSecret
		}
	}
	return nil
}

func validResourceKind(value string) bool {
	switch value {
	case "file", "certificate", "site", "artifact":
		return true
	default:
		return false
	}
}

// CanonicalManifest returns compact deterministic JSON suitable for hashing or
// serving verbatim from the SDK manifest endpoint.
func CanonicalManifest(descriptor ManifestDescriptor) ([]byte, error) {
	prepared, err := PrepareManifest(descriptor)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(prepared)
	if err != nil {
		return nil, fmt.Errorf("%w: encode: %w", ErrInvalidManifest, err)
	}
	if len(body) > manifestMaximumBytes {
		return nil, ErrManifestTooLarge
	}
	return body, nil
}

func CanonicalJSONObject(document []byte) ([]byte, error) {
	if len(document) > manifestMaximumBytes {
		return nil, ErrManifestTooLarge
	}
	if !utf8.Valid(document) || !ValidJSONObject(document) {
		return nil, fmt.Errorf("%w: document must be one unambiguous UTF-8 JSON object", ErrInvalidManifest)
	}
	var value map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(document))
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, fmt.Errorf("%w: document must be one JSON object", ErrInvalidManifest)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("%w: document must be one JSON object", ErrInvalidManifest)
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode schema: %w", ErrInvalidManifest, err)
	}
	if len(body) > manifestMaximumBytes {
		return nil, ErrManifestTooLarge
	}
	return body, nil
}

func validManifestIdentifier(value string) bool {
	if value == "" || len(value) > identifierMaximum {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9' && index > 0) || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validVersion(value string) bool {
	if value == "" || len(value) > versionMaximum || strings.ContainsAny(value, "\\\x00") {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("./+-", character) {
			continue
		}
		return false
	}
	return true
}

func validReference(value string) bool {
	return value == "" || (!strings.HasPrefix(value, "/") && !strings.Contains(value, "..") && !strings.ContainsAny(value, "\\\x00"))
}

func containsSensitiveValue(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") ||
		strings.Contains(lower, "privatekey") || strings.Contains(value, "\\") || strings.Contains(value, "..")
}

// HTTP health/readiness endpoints are intentionally route paths beginning
// with '/'. They are not filesystem references and must not be rejected by
// the manifest safety check that protects control/peer endpoint metadata.
func containsSensitiveRoute(value string) bool {
	if containsSensitiveValue(value) {
		return true
	}
	return value != "" && !strings.HasPrefix(value, "/")
}
