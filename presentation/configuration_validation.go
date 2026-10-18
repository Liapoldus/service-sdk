package presentation

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// ConfigurationValidator checks service-owned settings without changing runtime
// state, publishing a generation or redeeming active-generation secret grants.
type ConfigurationValidator interface {
	ValidateConfiguration(context.Context, []byte) error
}

func (set *HandlerSet) handleConfigurationValidation(writer http.ResponseWriter, request *http.Request) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != set.contracts.ContentTypes.JSON {
		set.writeTransportProblem(writer, unsupportedMediaTypeKey)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, set.contracts.MaximumMetadataBytes)
	raw, err := io.ReadAll(request.Body)
	if err != nil {
		if oversized(err) {
			set.writeTransportProblem(writer, payloadOversizedKey)
		} else {
			set.writeOutcomeProblem(writer, models.OutcomeInvalidRequest)
		}
		return
	}
	if !models.ValidJSONObject(raw) {
		set.writeOutcomeProblem(writer, models.OutcomeInvalidRequest)
		return
	}
	validator, ok := set.metadata.(ConfigurationValidator)
	if !ok {
		set.writeTransportProblem(writer, internalErrorKey)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), set.contracts.ReadinessDeadline)
	defer cancel()
	generation := request.Header.Get(set.contracts.ConfigurationValidationGenerationHeader)
	if generation != "" {
		parsed, parseErr := strconv.ParseInt(generation, 10, 64)
		if parseErr != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != generation {
			set.writeOutcomeProblem(writer, models.OutcomeInvalidRequest)
			return
		}
		scope, scoped := set.lifecycle.(ConfigurationValidationScope)
		if !scoped {
			set.writeTransportProblem(writer, internalErrorKey)
			return
		}
		err = scope.ValidateConfigurationGeneration(ctx, generation, func() error {
			return validator.ValidateConfiguration(ctx, raw)
		})
	} else {
		err = validator.ValidateConfiguration(ctx, raw)
	}
	if err != nil || ctx.Err() != nil {
		set.writeOutcomeProblem(writer, models.OutcomeApplyRejected)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
