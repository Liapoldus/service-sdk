package presentation

import (
	"context"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// ManifestProvider is the standard SDK metadata provider. It is intentionally
// immutable: a running service serves the same live manifest and schema bytes
// for its entire process lifetime.
type ManifestProvider struct {
	manifest []byte
	schema   []byte
}

func NewManifestProvider(descriptor models.ManifestDescriptor, schema []byte) (*ManifestProvider, error) {
	manifest, err := models.CanonicalManifest(descriptor)
	if err != nil {
		return nil, err
	}
	canonicalSchema, err := models.CanonicalJSONObject(schema)
	if err != nil {
		return nil, err
	}
	if err := models.ValidateConfigurationUI(descriptor.ConfigurationUI, canonicalSchema); err != nil {
		return nil, err
	}
	return &ManifestProvider{manifest: append([]byte(nil), manifest...), schema: append([]byte(nil), canonicalSchema...)}, nil
}

func (provider *ManifestProvider) Manifest(context.Context) ([]byte, error) {
	return append([]byte(nil), provider.manifest...), nil
}

func (provider *ManifestProvider) ConfigurationSchema(context.Context) ([]byte, error) {
	return append([]byte(nil), provider.schema...), nil
}
