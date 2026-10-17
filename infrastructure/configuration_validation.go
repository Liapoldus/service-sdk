package infrastructure

import (
	"bytes"
	"context"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// ValidateConfiguration sends an opaque candidate over the authenticated SDK
// channel. It never publishes, pulls or activates a generation.
func (client *ServiceClient) ValidateConfiguration(ctx context.Context, raw []byte) error {
	if client == nil || !models.ValidJSONObject(raw) {
		return ErrControlDocument
	}
	maximum := client.contract.Service.MaximumMetadataBytes
	if int64(len(raw)) > maximum {
		return ErrControlRequestOversized
	}
	response, err := client.call(ctx, serviceRouteConfigValidate, client.contract.Service.ConfigurationSchema.MediaType, maximum, controlCallReload, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return client.refusal(response, maximum)
	}
	return nil
}
