package models

import "errors"

const ServiceEditorButtonsVersion = "service-editor-buttons/v1"

var ErrInvalidServiceEditorButtons = errors.New("invalid Service SDK service editor buttons descriptor")

// ServiceEditorButtonsDescriptor is declarative metadata. It contains no
// executable code, callback, route, or plugin implementation.
type ServiceEditorButtonsDescriptor struct {
	Version string                `json:"version"`
	Buttons []ServiceEditorButton `json:"buttons"`
}

type ServiceEditorButton struct {
	ID          string              `json:"id"`
	Key         string              `json:"key"`
	Label       ConfigurationUIText `json:"label"`
	Description ConfigurationUIText `json:"description"`
}

// NewServiceEditorButton creates a button whose ID is stable across label
// changes and namespaced by its owning service.
func NewServiceEditorButton(serviceID, key string, label, description ConfigurationUIText) ServiceEditorButton {
	return ServiceEditorButton{ID: ServiceEditorButtonKey(serviceID, key), Key: key, Label: label, Description: description}
}

func ValidateServiceEditorButtons(serviceID string, descriptor *ServiceEditorButtonsDescriptor) error {
	if descriptor == nil {
		return nil
	}
	if !validManifestIdentifier(serviceID) || descriptor.Version != ServiceEditorButtonsVersion || len(descriptor.Buttons) == 0 || len(descriptor.Buttons) > 64 {
		return ErrInvalidServiceEditorButtons
	}
	seen := make(map[string]struct{}, len(descriptor.Buttons))
	for _, button := range descriptor.Buttons {
		if !validManifestIdentifier(button.Key) || button.ID != ServiceEditorButtonKey(serviceID, button.Key) ||
			!validLocalizedText(button.Label, false) || !validLocalizedText(button.Description, false) {
			return ErrInvalidServiceEditorButtons
		}
		if _, ok := seen[button.ID]; ok {
			return ErrInvalidServiceEditorButtons
		}
		seen[button.ID] = struct{}{}
	}
	return nil
}

func ValidateServiceEditorButtonKey(key string) bool { return validManifestIdentifier(key) }
