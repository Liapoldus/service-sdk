package main

// Fixture-only service-owned boundary and contract composition.
//
// This file is a test fixture and is never part of a production package. The
// applier, the metadata provider and the metrics sink are exactly the service
// owned pieces a real service author writes: the SDK owns the transport, the
// lifecycle and the REST surface, and the service owns what its document means.
// The contract mapping below is the composition step a service author also has to
// write, because the handler layer takes the contract as an injected value and
// never parses the asset itself.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Liapoldus/service-sdk/v2/tests/support/process"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
	"github.com/Liapoldus/service-sdk/v2/infrastructure"
	"github.com/Liapoldus/service-sdk/v2/presentation"
)

var (
	errTestApplyRefused = errors.New("test applier refused the generation")
	errTestAsset        = errors.New("test fixture could not reach the contract asset")
	errTestRoute        = errors.New("test fixture does not serve every contract route")
)

// testApplier is the service-owned validator and atomic applier.
//
// It inspects the service's own document and nothing else, and it activates a
// generation only after the whole document has been accepted, so a refusal
// leaves the previously active configuration fully usable.
type testApplier struct {
	mutex                     sync.Mutex
	active                    models.Configuration
	applied                   uint64
	refused                   uint64
	rejects                   map[string]struct{}
	lastError                 string
	beforeApply               func(context.Context, models.Configuration) (string, error)
	candidateSecretGeneration string
}

func newTestApplier(rejects ...string) *testApplier {
	applier := &testApplier{rejects: make(map[string]struct{}, len(rejects))}
	for _, generation := range rejects {
		applier.rejects[generation] = struct{}{}
	}
	return applier
}

// Apply implements the service-owned applier port.
func (applier *testApplier) Apply(ctx context.Context, configuration models.Configuration) error {
	if err := ctx.Err(); err != nil {
		// A cancelled apply must activate nothing.
		return err
	}
	secretGeneration := ""
	if applier.beforeApply != nil {
		var err error
		secretGeneration, err = applier.beforeApply(ctx, configuration)
		if err != nil {
			return err
		}
	}
	applier.mutex.Lock()
	defer applier.mutex.Unlock()
	if _, refuse := applier.rejects[configuration.Generation]; refuse {
		applier.refused++
		applier.lastError = errTestApplyRefused.Error()
		return errTestApplyRefused
	}
	applier.active = configuration
	applier.candidateSecretGeneration = secretGeneration
	applier.applied++
	applier.lastError = ""
	return nil
}

func (applier *testApplier) lastCandidateSecretGeneration() string {
	applier.mutex.Lock()
	defer applier.mutex.Unlock()
	return applier.candidateSecretGeneration
}

// active reports the configuration the fixture is currently serving.
func (applier *testApplier) activeConfiguration() (models.Configuration, bool) {
	applier.mutex.Lock()
	defer applier.mutex.Unlock()
	if applier.active.Generation == "" {
		return models.Configuration{}, false
	}
	return applier.active, true
}

// counters reports how many generations were activated and how many were refused.
func (applier *testApplier) counters() (applied, refused uint64) {
	applier.mutex.Lock()
	defer applier.mutex.Unlock()
	return applier.applied, applier.refused
}

// testMetadata is the service-owned producer of the two opaque documents the SDK
// publishes verbatim. The SDK never decodes them, so their shape is entirely the
// service's business.
type testMetadata struct {
	provider *presentation.ManifestProvider
}

func newTestMetadata() (testMetadata, error) {
	provider, err := presentation.NewManifestProvider(models.ManifestDescriptor{
		ID:      "reload-runtime-fixture",
		Version: "1.0.0",
		ConfigurationUI: &models.ConfigurationUIDescriptor{
			Version: models.ConfigurationUIVersion,
			Groups: []models.ConfigurationUIGroup{{
				ID:    "general",
				Title: models.ConfigurationUIText{RU: "Общие", EN: "General"},
				Fields: []models.ConfigurationUIField{{
					Path:   "/debug",
					Label:  models.ConfigurationUIText{RU: "Диагностика", EN: "Diagnostics"},
					Widget: "toggle",
				}},
			}},
		},
	}, []byte(testSchemaDocument))
	if err != nil {
		return testMetadata{}, err
	}
	return testMetadata{provider: provider}, nil
}

func (metadata testMetadata) Manifest(ctx context.Context) ([]byte, error) {
	return metadata.provider.Manifest(ctx)
}

func (metadata testMetadata) ConfigurationSchema(ctx context.Context) ([]byte, error) {
	return metadata.provider.ConfigurationSchema(ctx)
}

type testAdminMetadata struct{}

func (testAdminMetadata) AdminSurface(context.Context) ([]byte, error) {
	return []byte(`{"version":1,"pages":[{"id":"overview"}]}`), nil
}

type testAdminActions struct {
	calls     atomic.Int64
	cancelled atomic.Bool
	started   chan struct{}
	startOnce sync.Once
}

func (actions *testAdminActions) HandleAdminAction(ctx context.Context, input presentation.AdminActionInput) (presentation.AdminActionResponse, error) {
	actions.calls.Add(1)
	var request struct {
		Mode string `json:"mode"`
	}
	process.Must(json.Unmarshal(input.Body, &request))
	switch request.Mode {
	case "handler-error":
		return presentation.AdminActionResponse{}, errors.New("secret-payload should be redacted")
	case "oversized-receipt":
		return presentation.AdminActionResponse{StatusCode: 200, Body: bytes.Repeat([]byte("secret-payload"), 100000)}, nil
	case "slow":
		timer := time.NewTimer(11 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			return presentation.AdminActionResponse{StatusCode: 207, Body: []byte(`{"items":[{"id":"record-1"}]}`)}, nil
		case <-ctx.Done():
			return presentation.AdminActionResponse{}, ctx.Err()
		}
	case "wait-cancel":
		if actions.started != nil {
			actions.startOnce.Do(func() { close(actions.started) })
		}
		<-ctx.Done()
		actions.cancelled.Store(true)
		return presentation.AdminActionResponse{}, ctx.Err()
	default:
		return presentation.AdminActionResponse{StatusCode: 207, Body: []byte(`{"items":[{"id":"record-1"}]}`)}, nil
	}
}

// testMetricsSink is the service author's bridge from the application recorder to
// the infrastructure exposition. The application layer counts a bounded event;
// the infrastructure collector renders it in the contract exposition format. The
// SDK ships the two ends of that bridge but not the join, because which
// instrumentation a service runs is the service's decision.
type testMetricsSink struct {
	collector *infrastructure.ObserverPrometheusCollector
}

func (sink testMetricsSink) Lifecycle(kind string, outcome models.Outcome) {
	sink.collector.Observe(context.Background(), kind, outcome)
}

// PullFailure implements the application metrics port. The contract exposes one
// counter for exact-generation pull failures, so a refusal of any outcome counts
// once against it.
func (sink testMetricsSink) PullFailure(models.Outcome) {
	sink.collector.RecordConfigPullFailure()
}

func (sink testMetricsSink) SetReady(ready bool) {
	sink.collector.SetReady(ready)
}

// presentationContracts maps the loaded contract onto the injected view the
// handler layer validates at construction time.
//
// Three parts come straight off the loaded contract: the seven routes, the
// content types and the error table. The fourth, the transport refusal table,
// lives in a top-level block of the same asset that the loader does not expose,
// so the fixture reads that one block from the same file instead of restating
// it. The version of the file read is checked against the loaded contract, so a
// fixture pointed at a different asset version fails closed at composition time
// rather than serving a mixture of two contracts.
func presentationContracts(contract infrastructure.HTTPContract) (presentation.Contracts, error) {
	problems, err := testTransportProblems(contract)
	if err != nil {
		return presentation.Contracts{}, err
	}
	routes := make(map[string]presentation.Endpoint, len(contract.Service.Endpoints))
	for _, name := range contract.EndpointNames() {
		endpoint, err := contract.Endpoint(name)
		if err != nil {
			return presentation.Contracts{}, err
		}
		routes[name] = presentation.Endpoint{Method: endpoint.Method, Path: endpoint.Path}
	}
	// Every registered route must be served by exactly one handler field, so a
	// contract that grows a route this fixture does not serve is a startup error
	// rather than a silent 404.
	served := make(map[string]presentation.Endpoint, 8)
	for name, endpoint := range map[string]presentation.Endpoint{
		testRouteIdentity:       routes[testRouteIdentity],
		testRouteManifest:       routes[testRouteManifest],
		testRouteConfigSchema:   routes[testRouteConfigSchema],
		testRouteHealth:         routes[testRouteHealth],
		testRouteReady:          routes[testRouteReady],
		testRouteReload:         routes[testRouteReload],
		testRouteArtifactStream: routes[testRouteArtifactStream],
		testRouteAdminSurface:   routes[testRouteAdminSurface],
		testRouteAdminAction:    routes[testRouteAdminAction],
		testRouteMetrics:        routes[testRouteMetrics],
	} {
		if endpoint.Path == "" {
			return presentation.Contracts{}, fmt.Errorf("%w: %s", errTestRoute, name)
		}
		served[name] = endpoint
	}
	if len(served) != len(routes) {
		return presentation.Contracts{}, errTestRoute
	}

	// The contract already registers every error key, so this is a straight copy
	// of the table rather than a re-keyed view of it.
	errorTable := make(map[string]presentation.Problem, len(contract.Errors))
	for key, problem := range contract.Errors {
		errorTable[key] = presentation.Problem{Status: problem.Status, Code: problem.Code}
	}

	service := contract.Service
	contracts := presentation.Contracts{
		ContractVersion:        contract.ContractVersion,
		IdentityEndpoint:       served[testRouteIdentity],
		ManifestEndpoint:       served[testRouteManifest],
		ConfigSchemaEndpoint:   served[testRouteConfigSchema],
		HealthEndpoint:         served[testRouteHealth],
		ReadyEndpoint:          served[testRouteReady],
		ReloadEndpoint:         served[testRouteReload],
		ArtifactStreamEndpoint: served[testRouteArtifactStream],
		AdminSurfaceEndpoint:   served[testRouteAdminSurface],
		AdminActionEndpoint:    served[testRouteAdminAction],
		MetricsEndpoint:        served[testRouteMetrics],
		HealthStatus:           service.Responses.Health.Status,
		HealthBody:             service.Responses.Health.Body,
		ContentTypes:           presentation.ContentTypes{JSON: service.Responses.ContentTypes.JSON, Metrics: service.Responses.ContentTypes.Metrics},
		ReloadRequest:          testDocument(service.ReloadRequest),
		ReloadAcknowledgement:  testDocument(service.ReloadAcknowledgement),
		Readiness:              testDocument(service.Readiness),
		Manifest:               testDocument(service.Manifest),
		ConfigurationSchema:    testDocument(service.ConfigurationSchema),
		AdminSurface:           testDocument(service.AdminSurface),
		AdminAction: presentation.AdminActionContract{
			MediaType:            service.AdminAction.MediaType,
			MaximumRequestBytes:  service.AdminAction.MaximumRequestBytes,
			MaximumResponseBytes: service.AdminAction.MaximumResponseBytes,
			MaximumPageIDBytes:   service.AdminAction.MaximumPageIDBytes,
			MaximumActionIDBytes: service.AdminAction.MaximumActionIDBytes,
			PathSegmentPattern:   service.AdminAction.PathSegmentPattern,
			ResponseStatus:       presentation.StatusRangeContract{Minimum: service.AdminAction.ResponseStatus.Minimum, Maximum: service.AdminAction.ResponseStatus.Maximum},
			Deadline:             testSeconds(service.AdminAction.DeadlineSeconds),
			InvocationContext: presentation.AdminInvocationContract{
				MaximumBytes:        service.AdminAction.InvocationContext.MaximumBytes,
				UnknownHeaderPrefix: service.AdminAction.InvocationContext.UnknownHeaderPrefix,
				Required:            service.AdminAction.InvocationContext.Required,
				Optional:            service.AdminAction.InvocationContext.Optional,
				Headers:             service.AdminAction.InvocationContext.Headers,
			},
		},
		ArtifactStream: presentation.ArtifactStreamContract{
			MediaType:                     service.ArtifactStream.MediaType,
			MetadataMediaType:             service.ArtifactStream.MetadataMediaType,
			Parts:                         service.ArtifactStream.Parts,
			PartOrder:                     service.ArtifactStream.PartOrder,
			MaximumArtifactBytes:          service.ArtifactStream.MaximumArtifactBytes,
			MinimumArtifactBytes:          service.ArtifactStream.MinimumArtifactBytes,
			MaximumMetadataBytes:          service.ArtifactStream.MaximumMetadataBytes,
			MaximumMultipartOverheadBytes: service.ArtifactStream.MaximumMultipartOverheadBytes,
			MaximumRequestBytes:           service.ArtifactStream.MaximumRequestBytes,
			MaximumReceiptBytes:           service.ArtifactStream.MaximumReceiptBytes,
			AcceptedStatus:                service.ArtifactStream.AcceptedStatus,
			FilenameForwarded:             service.ArtifactStream.FilenameForwarded,
			Deadline:                      testSeconds(contract.Deadlines.ArtifactStreamSeconds),
			InvocationContext: presentation.ArtifactInvocationContract{
				MaximumBytes: service.ArtifactStream.InvocationContext.MaximumBytes,
				Required:     service.ArtifactStream.InvocationContext.Required,
				Optional:     service.ArtifactStream.InvocationContext.Optional,
				Headers:      service.ArtifactStream.InvocationContext.Headers,
			},
		},
		Registration:         testDocument(infrastructure.DocumentContract{MediaType: contract.Identity.Registration.MediaType, MaximumBytes: contract.Identity.Registration.MaximumBytes, Required: contract.Identity.Registration.Required}),
		MaximumMetadataBytes: service.MaximumMetadataBytes,
		ReadinessDeadline:    testSeconds(contract.Deadlines.ServiceReadinessSeconds),
		Problems:             problems,
		Errors:               errorTable,
		OutcomeProblems:      contract.OutcomeProblems,
		SuccessOutcomes:      contract.SuccessOutcomes,
	}
	if err := contracts.Validate(); err != nil {
		return presentation.Contracts{}, err
	}
	return contracts, nil
}

// testDocument converts one contract document description to the injected form.
func testDocument(document infrastructure.DocumentContract) presentation.DocumentContract {
	return presentation.DocumentContract{
		MediaType:       document.MediaType,
		MaximumBytes:    document.MaximumBytes,
		Required:        document.Required,
		DigestAlgorithm: document.DigestAlgorithm,
	}
}

// testAssetDocument is the one part of the asset the loader does not expose: the
// top-level block of refusals the transport itself raises.
type testAssetDocument struct {
	ContractVersion string `json:"contractVersion"`
	Problems        map[string]struct {
		Status int    `json:"status"`
		Code   string `json:"code"`
	} `json:"problems"`
}

// testTransportProblems reads the transport refusal block from the same asset
// file the SDK embeds and checks that it describes the contract the fixture just
// loaded.
func testTransportProblems(contract infrastructure.HTTPContract) (map[string]presentation.Problem, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("%w: caller", errTestAsset)
	}
	// The asset lives at the module root, three directories above this file, so
	// the path is resolved from the source location rather than from the working
	// directory the harness happened to start the fixture in.
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..",
		"infrastructure", "assets", "service-sdk", "v2", "http-contract.json")
	contents, err := os.ReadFile(path) //nolint:gosec // path is derived from the fixture source location and a fixed asset suffix.
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTestAsset, err)
	}
	var asset testAssetDocument
	if err := json.Unmarshal(contents, &asset); err != nil {
		return nil, fmt.Errorf("%w: %w", errTestAsset, err)
	}
	if asset.ContractVersion != contract.ContractVersion {
		// The fixture would otherwise be serving the refusal table of a contract
		// it is not otherwise speaking.
		return nil, fmt.Errorf("%w: asset version %q, loaded %q",
			errTestAsset, asset.ContractVersion, contract.ContractVersion)
	}
	problems := make(map[string]presentation.Problem, len(asset.Problems))
	for key, problem := range asset.Problems {
		problems[key] = presentation.Problem{Status: problem.Status, Code: problem.Code}
	}
	return problems, nil
}
