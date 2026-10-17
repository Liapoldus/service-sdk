package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ConfigurationUIVersion identifies the SDK-owned, declarative presentation
// hints carried by a service manifest. It is never executable UI code.
const ConfigurationUIVersion = "service-configuration-ui/v1"

const (
	configurationUIMaxGroups         = 128
	configurationUIMaxFields         = 2048
	configurationUIMaxOrder          = 10000
	configurationUISchemaMaxBranches = 64
)

var ErrInvalidConfigurationUI = errors.New("invalid Service SDK configuration UI descriptor")

// ConfigurationUIDescriptor adds localized grouping and widget hints to the
// service-owned JSON Schema. The schema remains the only validation authority.
type ConfigurationUIDescriptor struct {
	Version string                 `json:"version"`
	Groups  []ConfigurationUIGroup `json:"groups"`
}

type ConfigurationUIText struct {
	RU string `json:"ru"`
	EN string `json:"en"`
}

type ConfigurationUIGroup struct {
	ID          string                 `json:"id"`
	Title       ConfigurationUIText    `json:"title"`
	Description *ConfigurationUIText   `json:"description,omitempty"`
	Order       int                    `json:"order,omitempty"`
	Fields      []ConfigurationUIField `json:"fields"`
}

type ConfigurationUIField struct {
	Path           string               `json:"path"`
	Label          ConfigurationUIText  `json:"label"`
	Help           *ConfigurationUIText `json:"help,omitempty"`
	Unit           *ConfigurationUIText `json:"unit,omitempty"`
	Widget         string               `json:"widget"`
	SecretPurpose  string               `json:"secretPurpose,omitempty"`
	ResourceKind   string               `json:"resourceKind,omitempty"`
	ResourceID     string               `json:"resourceId,omitempty"`
	PeerContractID string               `json:"peerContractId,omitempty"`
	Order          int                  `json:"order,omitempty"`
}

// ValidateConfigurationUI validates descriptor structure and, when schema is
// non-empty, checks each hint against a local JSON Schema node. Remote $ref and
// unresolved paths fail closed.
func ValidateConfigurationUI(descriptor *ConfigurationUIDescriptor, schema []byte) error {
	if err := validateConfigurationUIDescriptor(descriptor); err != nil {
		return err
	}
	if descriptor == nil || len(schema) == 0 {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(schema, &root); err != nil || root == nil {
		return fmt.Errorf("%w: invalid service schema", ErrInvalidConfigurationUI)
	}
	for _, group := range descriptor.Groups {
		for _, field := range group.Fields {
			nodes, err := schemaNodes(root, field.Path, field.Widget == "secret")
			if err != nil || len(nodes) == 0 {
				return fmt.Errorf("%w: unresolved field path %q", ErrInvalidConfigurationUI, field.Path)
			}
			for _, node := range nodes {
				if sensitiveSchemaNode(node) || !widgetMatchesSchema(field, node) {
					return fmt.Errorf("%w: widget %q is incompatible with field %q", ErrInvalidConfigurationUI, field.Widget, field.Path)
				}
			}
		}
	}
	return nil
}

func validateConfigurationUIDescriptor(descriptor *ConfigurationUIDescriptor) error {
	if descriptor == nil {
		return nil
	}
	if descriptor.Version != ConfigurationUIVersion || len(descriptor.Groups) == 0 || len(descriptor.Groups) > configurationUIMaxGroups {
		return ErrInvalidConfigurationUI
	}
	groupIDs := make(map[string]struct{}, len(descriptor.Groups))
	fieldPaths := make(map[string]struct{})
	fieldCount := 0
	for _, group := range descriptor.Groups {
		if !validManifestIdentifier(group.ID) || !validUIOrder(group.Order) || !validLocalizedText(group.Title, false) || !validOptionalLocalizedText(group.Description) || len(group.Fields) == 0 {
			return ErrInvalidConfigurationUI
		}
		if _, exists := groupIDs[group.ID]; exists {
			return ErrInvalidConfigurationUI
		}
		groupIDs[group.ID] = struct{}{}
		for _, field := range group.Fields {
			fieldCount++
			if fieldCount > configurationUIMaxFields || !validUIOrder(field.Order) || !validConfigurationPath(field.Path) ||
				!validLocalizedText(field.Label, false) || !validOptionalLocalizedText(field.Help) || !validOptionalLocalizedText(field.Unit) {
				return ErrInvalidConfigurationUI
			}
			if _, exists := fieldPaths[field.Path]; exists {
				return ErrInvalidConfigurationUI
			}
			fieldPaths[field.Path] = struct{}{}
			switch field.Widget {
			case "auto", "text", "textarea", "number", "slider", "toggle", "select", "array":
				if field.SecretPurpose != "" || field.ResourceKind != "" || field.ResourceID != "" || field.PeerContractID != "" {
					return ErrInvalidConfigurationUI
				}
			case "secret":
				if !validConfigurationSecretPurpose(field.SecretPurpose) || field.ResourceKind != "" || field.ResourceID != "" || field.PeerContractID != "" {
					return ErrInvalidConfigurationUI
				}
			case "resource":
				if field.SecretPurpose != "" || field.PeerContractID != "" || !map[string]bool{"file": true, "certificate": true, "site": true, "site-release": true, "artifact": true}[field.ResourceKind] || !validManifestIdentifier(field.ResourceID) {
					return ErrInvalidConfigurationUI
				}
			case "peer":
				if field.SecretPurpose != "" || field.ResourceKind != "" || field.ResourceID != "" || !validManifestIdentifier(field.PeerContractID) {
					return ErrInvalidConfigurationUI
				}
			default:
				return ErrInvalidConfigurationUI
			}
		}
	}
	return nil
}

func validConfigurationSecretPurpose(value string) bool {
	if value == "" || len(value) > 192 || strings.TrimSpace(value) != value {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 8 {
		return false
	}
	for _, part := range parts {
		if part == "" || part[0] < 'a' || part[0] > 'z' {
			return false
		}
		for _, character := range part {
			switch {
			case character >= 'a' && character <= 'z':
			case character >= '0' && character <= '9':
			case character == '-':
			default:
				return false
			}
		}
	}
	return true
}

func validLocalizedText(value ConfigurationUIText, optional bool) bool {
	valid := func(text string) bool {
		return len(text) <= 2048 && !strings.ContainsAny(text, "\x00\r\n") && !unsafeConfigurationUIText(text)
	}
	if optional && value.RU == "" && value.EN == "" {
		return true
	}
	return strings.TrimSpace(value.RU) != "" && strings.TrimSpace(value.EN) != "" && valid(value.RU) && valid(value.EN)
}

func unsafeConfigurationUIText(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"password=", "password:", "secret=", "secret:", "token=", "token:", "private_key=", "private-key=", "privatekey=", "bearer ", "-----begin private key-----"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	for _, marker := range []string{"/users/", "/home/", "/etc/", "/var/", "~/", ":\\", "\\\\"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func validOptionalLocalizedText(value *ConfigurationUIText) bool {
	return value == nil || validLocalizedText(*value, false)
}

func validUIOrder(value int) bool { return value >= 0 && value <= configurationUIMaxOrder }

func validConfigurationPath(value string) bool {
	if value == "" || !strings.HasPrefix(value, "/") || len(value) > 1024 || strings.ContainsRune(value, '\x00') {
		return false
	}
	for _, segment := range strings.Split(value[1:], "/") {
		if segment == "" {
			return false
		}
		for index := 0; index < len(segment); index++ {
			if segment[index] == '~' {
				if index+1 >= len(segment) || (segment[index+1] != '0' && segment[index+1] != '1') {
					return false
				}
				index++
			}
		}
	}
	return true
}

func widgetMatchesSchema(field ConfigurationUIField, schema map[string]any) bool {
	if field.Widget == "auto" {
		return true
	}
	kind := schemaType(schema)
	switch field.Widget {
	case "text", "textarea":
		return kind == "string"
	case "number":
		return kind == "number" || kind == "integer"
	case "slider":
		_, minimum := schema["minimum"]
		_, maximum := schema["maximum"]
		return (kind == "number" || kind == "integer") && minimum && maximum
	case "toggle":
		return kind == "boolean"
	case "select":
		_, hasEnum := schema["enum"]
		_, hasConst := schema["const"]
		return hasEnum || hasConst
	case "array":
		return kind == "array"
	case "resource":
		return kind == "string"
	case "peer":
		// A peer picker commits one service instance ID to this field. Structured
		// peer bindings need an explicit value-mapping contract; an object schema
		// must never be populated by guessing property names.
		return kind == "string"
	case "secret":
		pattern, patternOK := schema["pattern"].(string)
		if !patternOK || pattern != `^ssr:v1:[a-f0-9]{32}$` || schema["x-liapoldus-secret-reference"] != true {
			return false
		}
		for _, key := range []string{"default", "examples", "enum", "const"} {
			if _, exists := schema[key]; exists {
				return false
			}
		}
		return kind == "string"
	default:
		return false
	}
}

func schemaType(schema map[string]any) string {
	switch value := schema["type"].(type) {
	case string:
		return value
	case []any:
		if len(value) == 1 {
			kind, ok := value[0].(string)
			if ok {
				return kind
			}
		}
	}
	return ""
}

func sensitiveSchemaNode(schema map[string]any) bool {
	for _, marker := range []string{"readOnly", "writeOnly", "x-liapoldus-sensitive", "x-sensitive", "x-secret", "x-redacted"} {
		if schema[marker] == true {
			return true
		}
	}
	format, ok := schema["format"].(string)
	return ok && strings.EqualFold(format, "password")
}

func sensitiveConfigurationFieldName(name string) bool {
	normalized := strings.ToLower(name)
	for _, marker := range []string{"secret", "token", "password", "privatekey", "authorization"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func schemaNodes(root map[string]any, path string, allowSecretReference bool) ([]map[string]any, error) {
	nodes := []map[string]any{root}
	if sensitiveSchemaNode(root) {
		return nil, ErrInvalidConfigurationUI
	}
	segments := strings.Split(path[1:], "/")
	for index, encodedSegment := range segments {
		segment := strings.ReplaceAll(strings.ReplaceAll(encodedSegment, "~1", "/"), "~0", "~")
		secretLeaf := allowSecretReference && index == len(segments)-1
		if segment != "*" && sensitiveConfigurationFieldName(segment) && !secretLeaf {
			return nil, ErrInvalidConfigurationUI
		}
		next := make([]map[string]any, 0)
		for _, node := range nodes {
			children, err := schemaChildren(root, node, segment, make(map[string]bool), 0)
			if err != nil {
				return nil, err
			}
			next = append(next, children...)
		}
		nodes = uniqueSchemaNodes(next)
		if len(nodes) == 0 {
			return nil, nil
		}
		resolved := make([]map[string]any, 0, len(nodes))
		for _, node := range nodes {
			value, err := resolveSchemaNode(root, node, make(map[string]bool), 0)
			if err != nil || sensitiveSchemaNode(value) {
				return nil, ErrInvalidConfigurationUI
			}
			resolved = append(resolved, value)
		}
		nodes = uniqueSchemaNodes(resolved)
	}
	return nodes, nil
}

func resolveSchemaNode(root, node map[string]any, seen map[string]bool, depth int) (map[string]any, error) {
	if depth > 32 {
		return nil, ErrInvalidConfigurationUI
	}
	ref, hasRef := node["$ref"].(string)
	if !hasRef {
		return node, nil
	}
	if !strings.HasPrefix(ref, "#/") || seen[ref] {
		return nil, ErrInvalidConfigurationUI
	}
	resolved, err := resolveLocalSchemaRef(root, ref)
	if err != nil {
		return nil, err
	}
	seen[ref] = true
	resolved, err = resolveSchemaNode(root, resolved, seen, depth+1)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]any, len(resolved)+len(node)-1)
	for key, value := range resolved {
		merged[key] = value
	}
	for key, value := range node {
		if key != "$ref" {
			merged[key] = value
		}
	}
	return merged, nil
}

func schemaChildren(root, node map[string]any, segment string, seen map[string]bool, depth int) ([]map[string]any, error) {
	if depth > 32 {
		return nil, ErrInvalidConfigurationUI
	}
	children := make([]map[string]any, 0)
	if ref, ok := node["$ref"].(string); ok {
		if !strings.HasPrefix(ref, "#/") || seen[ref] {
			return nil, ErrInvalidConfigurationUI
		}
		resolved, err := resolveLocalSchemaRef(root, ref)
		if err != nil {
			return nil, err
		}
		refSeen := cloneRefSet(seen)
		refSeen[ref] = true
		values, err := schemaChildren(root, resolved, segment, refSeen, depth+1)
		if err != nil {
			return nil, err
		}
		children = append(children, values...)
	}
	if segment == "*" {
		if items, ok := node["items"].(map[string]any); ok {
			children = append(children, items)
		}
	} else if properties, ok := node["properties"].(map[string]any); ok {
		if property, ok := properties[segment].(map[string]any); ok {
			children = append(children, property)
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		branchValue, exists := node[keyword]
		if !exists {
			continue
		}
		branches, ok := branchValue.([]any)
		if !ok || len(branches) > configurationUISchemaMaxBranches {
			return nil, ErrInvalidConfigurationUI
		}
		for _, branch := range branches {
			branchSchema, ok := branch.(map[string]any)
			if !ok {
				return nil, ErrInvalidConfigurationUI
			}
			values, err := schemaChildren(root, branchSchema, segment, cloneRefSet(seen), depth+1)
			if err != nil {
				return nil, err
			}
			children = append(children, values...)
		}
	}
	return uniqueSchemaNodes(children), nil
}

func resolveLocalSchemaRef(root map[string]any, ref string) (map[string]any, error) {
	node := any(root)
	for _, encodedSegment := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		segment := strings.ReplaceAll(strings.ReplaceAll(encodedSegment, "~1", "/"), "~0", "~")
		object, ok := node.(map[string]any)
		if !ok {
			return nil, ErrInvalidConfigurationUI
		}
		node, ok = object[segment]
		if !ok {
			return nil, ErrInvalidConfigurationUI
		}
	}
	resolved, ok := node.(map[string]any)
	if !ok {
		return nil, ErrInvalidConfigurationUI
	}
	return resolved, nil
}

func cloneRefSet(source map[string]bool) map[string]bool {
	copy := make(map[string]bool, len(source)+1)
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func uniqueSchemaNodes(nodes []map[string]any) []map[string]any {
	seen := make(map[string]bool, len(nodes))
	unique := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		key := fmt.Sprintf("%p", node)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, node)
		}
	}
	return unique
}
