package interfaces

import (
	"context"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// ConfigurationApplier is the service-owned boundary. The service validates the
// exact opaque document against its own versioned JSON Schema and atomically
// activates it in its own runtime before returning nil. The SDK never inspects
// a product field and never applies anything itself.
//
// Returning an error must leave the previously active configuration fully
// usable. A cancelled context must abort the apply without activating anything.
type ConfigurationApplier interface {
	Apply(ctx context.Context, configuration models.Configuration) error
}
