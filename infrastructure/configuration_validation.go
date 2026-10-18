package infrastructure

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// ValidateConfiguration sends an opaque candidate over the authenticated SDK
// channel. It never publishes, pulls or activates a generation.
func (client *ServiceClient) ValidateConfiguration(ctx context.Context, generation string, raw []byte) error {
	if client == nil || !models.ValidJSONObject(raw) {
		return ErrControlDocument
	}
	if generation != "" {
		parsed, err := strconv.ParseInt(generation, 10, 64)
		if err != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != generation {
			return ErrInvalidControlCall
		}
	}
	maximum := client.contract.Service.MaximumMetadataBytes
	if int64(len(raw)) > maximum {
		return ErrControlRequestOversized
	}
	headers := make(http.Header)
	if generation != "" {
		headers.Set(client.contract.Service.ConfigurationValidationGenerationHeader, generation)
	}
	response, err := client.callWithHeaders(ctx, serviceRouteConfigValidate, client.contract.Service.ConfigurationSchema.MediaType, maximum, controlCallReload, bytes.NewReader(raw), headers)
	if err != nil {
		return err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return client.refusal(response, maximum)
	}
	return nil
}
