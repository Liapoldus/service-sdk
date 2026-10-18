package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

// The Core-side client reaches the service lifecycle surface over the same
// controlCallReload family the contract publishes a single Core-facing outcome
// group for. control_transport.go registers exactly one Core-to-service family, so
// every read the operator performs is classified in that one vocabulary instead
// of borrowing a service-to-Core or secret outcome.

// The service lifecycle routes are registered in the contract asset under these
// logical names. They are the only endpoint keys this package may use; no path,
// method or route string is written here.
const (
	serviceRouteIdentity       = "identity"
	serviceRouteManifest       = "manifest"
	serviceRouteConfigSchema   = "configSchema"
	serviceRouteConfigValidate = "configValidate"
	serviceRouteHealth         = "health"
	serviceRouteReady          = "ready"
	serviceRouteReload         = "reload"
	serviceRouteArtifactStream = "artifactStream"
	serviceRouteAdminSurface   = "adminSurface"
	serviceRouteAdminAction    = "adminAction"
	serviceRouteMetrics        = "metrics"
)

// controlErrorNotReady is the contract error key of the one non-success answer
// the readiness route is defined to publish. It is read from the asset rather
// than compared against a literal status, so a contract change moves the status
// and the code together.
const controlErrorNotReady = "notReady"

// controlServiceRouteNames lists every service route this client must be able to
// resolve before it is usable.
var controlServiceRouteNames = [...]string{
	serviceRouteIdentity,
	serviceRouteManifest,
	serviceRouteConfigSchema,
	serviceRouteConfigValidate,
	serviceRouteHealth,
	serviceRouteReady,
	serviceRouteReload,
	serviceRouteArtifactStream,
	serviceRouteAdminSurface,
	serviceRouteAdminAction,
	serviceRouteMetrics,
}

// ArtifactStreamResult preserves the product-owned result from a streamed
// action. The SDK does not inspect its fields or classify product errors.
type ArtifactStreamResult struct {
	StatusCode int
	Body       []byte
}

// AdminSurfaceDocument contains the service-owned descriptor as exact bytes and
// the digest calculated over those bytes, without decoding or re-encoding it.
type AdminSurfaceDocument struct {
	Bytes  []byte
	SHA256 string
}

// AdminActionResult preserves the product-owned HTTP status and exact JSON body.
type AdminActionResult struct {
	StatusCode int
	Body       []byte
}

// ServiceClient is the Core-side control client of exactly one service replica.
// It is the client a Core operator and the conformance suite drive: it announces
// a generation, reads back what the replica actually applied, and reads the
// replica's published metadata, health, readiness and metrics.
//
// The client transports and classifies. It never decides lifecycle policy: it
// does not verify a digest, does not judge whether a generation may be applied
// and does not invent an outcome. It reports what the replica published, refuses
// an answer it cannot interpret, and attributes only the failures it saw on the
// wire, so exactly one owner decides each lifecycle outcome.
//
// Every call is a single request. There is no retry, no replay and no redirect
// following anywhere in this type, because a Reload whose outcome the client
// cannot know must not be repeated with the replica credentials attached.
//
// ServiceClient is safe for concurrent use.
type ServiceClient struct {
	contract  HTTPContract
	baseURL   *url.URL
	transport ControlTransport
	peer      models.PeerIdentity
	problems  controlProblemResolver
	routes    map[string]Endpoint
	notReady  int
	reload    int
	read      int
}

// ArtifactStream forwards a JSON metadata part and one live artifact part to
// the service over the contract's per-replica mTLS channel. It consumes and closes
// artifact; callers must pass the request body they are forwarding so cancellation
// can close its source. Neither part is buffered as a whole.
func (client *ServiceClient) ArtifactStream(ctx context.Context, invocation models.ArtifactInvocation, metadata []byte, contentType string, artifact io.ReadCloser) (ArtifactStreamResult, error) {
	if client == nil || artifact == nil {
		return ArtifactStreamResult{}, ErrInvalidControlCall
	}
	defer closeResource(artifact)
	stream := client.contract.Service.ArtifactStream
	if err := invocation.Validate(); err != nil {
		return ArtifactStreamResult{}, err
	}
	if int64(len(metadata)) > stream.MaximumMetadataBytes {
		return ArtifactStreamResult{}, ErrControlRequestOversized
	}
	if !models.ValidJSONObject(metadata) {
		return ArtifactStreamResult{}, ErrControlDocument
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil || strings.ContainsAny(contentType, "\r\n") || int64(len(contentType)) > stream.MaximumMultipartOverheadBytes {
		return ArtifactStreamResult{}, ErrControlMediaType
	}
	invocationHeaders, err := artifactInvocationHeaders(invocation, stream.InvocationContext)
	if err != nil {
		return ArtifactStreamResult{}, err
	}
	endpoint, ok := client.routes[serviceRouteArtifactStream]
	if !ok {
		return ArtifactStreamResult{}, ErrInvalidControlCall
	}
	target, err := controlEndpointURL(client.baseURL, endpoint.Path)
	if err != nil {
		return ArtifactStreamResult{}, err
	}
	callCtx, cancel, err := controlCallContext(ctx, client.contract.Deadlines.ArtifactStreamSeconds)
	if err != nil {
		return ArtifactStreamResult{}, err
	}
	defer cancel()
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	production := make(chan error, 1)
	go func() {
		production <- writeArtifactMultipart(pipeWriter, multipartWriter, stream, metadata, contentType, artifact)
	}()
	request, err := http.NewRequestWithContext(callCtx, endpoint.Method, target.String(), pipeReader)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		closeResource(artifact)
		<-production
		return ArtifactStreamResult{}, ErrControlTransportFailed
	}
	request.Header.Set("content-type", multipartWriter.FormDataContentType())
	request.Header.Set("accept", client.contract.Service.Responses.ContentTypes.JSON)
	for name, value := range invocationHeaders {
		request.Header.Set(name, value)
	}
	transport, ok := client.transport.(ArtifactControlTransport)
	if !ok {
		_ = pipeReader.CloseWithError(ErrInvalidControlCall)
		closeResource(artifact)
		<-production
		return ArtifactStreamResult{}, ErrInvalidControlCall
	}
	response, err := performArtifactControlCall(transport, request) //nolint:bodyclose // the response body is closed on every return path below.
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		closeResource(artifact)
		<-production
		return ArtifactStreamResult{}, err
	}
	if response.Body == nil {
		_ = pipeReader.CloseWithError(ErrControlPlaneUnusable)
		closeResource(artifact)
		<-production
		return ArtifactStreamResult{}, ErrControlPlaneUnusable
	}
	if response.StatusCode != stream.AcceptedStatus &&
		(response.StatusCode < http.StatusBadRequest || response.StatusCode >= http.StatusInternalServerError) {
		closeResource(response.Body)
		_ = pipeReader.CloseWithError(ErrControlDocument)
		closeResource(artifact)
		<-production
		return ArtifactStreamResult{}, controlUnusable(controlCallReload, ErrControlDocument)
	}
	body, readErr := readControlBody(response.Body, stream.MaximumReceiptBytes)
	closeResource(response.Body)
	closeResource(pipeReader)
	closeResource(artifact)
	productionErr := <-production
	if productionErr != nil {
		return ArtifactStreamResult{}, productionErr
	}
	if readErr != nil {
		return ArtifactStreamResult{}, readErr
	}
	if !models.ValidJSONObject(body) {
		return ArtifactStreamResult{}, ErrControlDocument
	}
	return ArtifactStreamResult{StatusCode: response.StatusCode, Body: body}, nil
}

// AdminSurface returns the service-owned management descriptor exactly as
// published. The SDK computes a digest for Core to bind to a subsequent action.
func (client *ServiceClient) AdminSurface(ctx context.Context) (AdminSurfaceDocument, error) {
	if client == nil {
		return AdminSurfaceDocument{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	response, err := client.call(ctx, serviceRouteAdminSurface, client.contract.Service.AdminSurface.MediaType,
		client.contract.Service.AdminSurface.MaximumBytes, controlCallReload, nil)
	if err != nil {
		return AdminSurfaceDocument{}, err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return AdminSurfaceDocument{}, client.refusal(response, client.contract.Service.AdminSurface.MaximumBytes)
	}
	body, err := client.document(response, client.contract.Service.AdminSurface, controlCallReload)
	if err != nil {
		return AdminSurfaceDocument{}, err
	}
	digest := sha256.Sum256(body)
	return AdminSurfaceDocument{Bytes: body, SHA256: "sha256:" + hex.EncodeToString(digest[:])}, nil
}

// AdminAction performs one authorized JSON invocation. It deliberately does not
// retry or interpret the service-owned status/body; callers own product semantics.
func (client *ServiceClient) AdminAction(ctx context.Context, invocation models.AdminActionInvocation, input []byte) (AdminActionResult, error) {
	if client == nil || invocation.Validate() != nil {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	contract := client.contract.Service.AdminAction
	if int64(len(input)) > contract.MaximumRequestBytes || !models.ValidJSONObject(input) ||
		len(invocation.PageID) > contract.MaximumPageIDBytes || len(invocation.ActionID) > contract.MaximumActionIDBytes {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrControlRequestOversized)
	}
	pattern, err := regexp.Compile(contract.PathSegmentPattern)
	if err != nil || !pattern.MatchString(invocation.PageID) || !pattern.MatchString(invocation.ActionID) {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	endpoint, ok := client.routes[serviceRouteAdminAction]
	if !ok {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	path := strings.Replace(endpoint.Path, "{page}", url.PathEscape(invocation.PageID), 1)
	path = strings.Replace(path, "{action}", url.PathEscape(invocation.ActionID), 1)
	target, err := controlEndpointURL(client.baseURL, path)
	if err != nil {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	callCtx, cancel, err := controlCallContext(ctx, contract.DeadlineSeconds)
	if err != nil {
		return AdminActionResult{}, controlUnusable(controlCallReload, err)
	}
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, endpoint.Method, target.String(), bytes.NewReader(input))
	if err != nil {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrControlTransportFailed)
	}
	request.Header.Set("content-type", contract.MediaType)
	request.Header.Set("accept", contract.MediaType)
	for logical, header := range contract.InvocationContext.Headers {
		value := adminInvocationValue(invocation, logical)
		if value != "" {
			request.Header.Set(header, value)
		}
	}
	transport, ok := client.transport.(AdminActionControlTransport)
	if !ok {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	response, err := performAdminActionControlCall(transport, request)
	if err != nil {
		return AdminActionResult{}, err
	}
	if response.Body == nil {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrControlPlaneUnusable)
	}
	defer closeResource(response.Body)
	body, err := readControlBody(response.Body, contract.MaximumResponseBytes)
	if err != nil {
		return AdminActionResult{}, controlBodyFailure(controlCallReload, err)
	}
	if !models.ValidJSONObject(body) || response.StatusCode < contract.ResponseStatus.Minimum || response.StatusCode > contract.ResponseStatus.Maximum {
		return AdminActionResult{}, controlUnusable(controlCallReload, ErrControlDocument)
	}
	return AdminActionResult{StatusCode: response.StatusCode, Body: body}, nil
}

func adminInvocationValue(invocation models.AdminActionInvocation, logical string) string {
	switch logical {
	case "callerId":
		return invocation.CallerID
	case "instanceId":
		return invocation.InstanceID
	case "pageId":
		return invocation.PageID
	case "actionId":
		return invocation.ActionID
	case "surfaceDigest":
		return invocation.SurfaceDigest
	case "requestId":
		return invocation.RequestID
	case "idempotencyKey":
		return invocation.IdempotencyKey
	case "ifMatch":
		return invocation.IfMatch
	default:
		return ""
	}
}

func artifactInvocationHeaders(invocation models.ArtifactInvocation, contract ArtifactInvocationContract) (map[string]string, error) {
	values := map[string]string{
		"callerId": invocation.CallerID, "instanceId": invocation.InstanceID,
		"pageId": invocation.PageID, "actionId": invocation.ActionID,
		"surfaceDigest": invocation.SurfaceDigest, "idempotencyKey": invocation.IdempotencyKey,
		"requestId": invocation.RequestID, "ifMatch": invocation.IfMatch,
	}
	headers := make(map[string]string, len(contract.Headers))
	total := 0
	for logical, header := range contract.Headers {
		value := values[logical]
		if value == "" {
			if contains(contract.Required, logical) {
				return nil, models.ErrInvalidArtifactInvocation
			}
			continue
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, models.ErrInvalidArtifactInvocation
		}
		total += len(logical) + len(header) + len(value)
		headers[header] = value
	}
	if total > contract.MaximumBytes {
		return nil, ErrControlRequestOversized
	}
	return headers, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// ArtifactControlTransport preserves the bounded request-context deadline while
// allowing the streaming endpoint's longer contract deadline than JSON calls.
type ArtifactControlTransport interface {
	DoArtifact(request *http.Request) (*http.Response, error)
}

// AdminActionControlTransport performs a bounded action request using its
// request-context deadline rather than the shorter general control timeout.
type AdminActionControlTransport interface {
	DoAdminAction(request *http.Request) (*http.Response, error)
}

func writeArtifactMultipart(pipe *io.PipeWriter, writer *multipart.Writer, contract ArtifactStreamContract, metadata []byte, contentType string, artifact io.Reader) error {
	metadataHeader := make(textproto.MIMEHeader)
	metadataHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": contract.Parts[0]}))
	metadataHeader.Set("Content-Type", contract.MetadataMediaType)
	metadataPart, err := writer.CreatePart(metadataHeader)
	if err == nil {
		_, err = metadataPart.Write(metadata)
	}
	if err == nil {
		artifactHeader := make(textproto.MIMEHeader)
		artifactHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": contract.Parts[1]}))
		artifactHeader.Set("Content-Type", contentType)
		var artifactPart io.Writer
		artifactPart, err = writer.CreatePart(artifactHeader)
		if err == nil {
			var copied int64
			copied, err = io.Copy(artifactPart, io.LimitReader(artifact, contract.MaximumArtifactBytes+1))
			if copied > contract.MaximumArtifactBytes {
				err = ErrControlRequestOversized
			}
			if err == nil && copied < contract.MinimumArtifactBytes {
				err = ErrControlDocument
			}
		}
	}
	closeErr := writer.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = pipe.CloseWithError(err)
		return err
	}
	return pipe.Close()
}

// NewServiceClient builds the Core-side control client for one service replica.
//
// It fails closed on anything that could not produce a safe call: a base URL
// that is not a bare HTTPS origin, a missing transport, a peer identity that is
// absent, malformed or a wildcard, a contract that no longer registers one of
// the lifecycle routes, a deadline or a document limit the contract omits, a
// media type the contract declares but cannot express, and a call family the
// contract describes no problem for. The pinned identity is exactly one
// models.PeerIdentity: a peer set of zero and a wildcard pattern are both
// refused, so the client can never be pointed at "any" replica.
func NewServiceClient(contract HTTPContract, baseURL string, transport ControlTransport, peer models.PeerIdentity) (*ServiceClient, error) {
	parsed, valid := parseControlURL(baseURL)
	if !valid || transport == nil {
		return nil, ErrInvalidControlCall
	}
	if !peer.Valid() || mutualTLSHasWildcard(peer) {
		return nil, ErrInvalidControlCall
	}
	routes := make(map[string]Endpoint, len(controlServiceRouteNames))
	for _, name := range controlServiceRouteNames {
		endpoint, err := contract.Endpoint(name)
		if err != nil {
			return nil, err
		}
		routes[name] = endpoint
	}
	if err := requireControlMediaTypes([]string{
		contract.Service.ReloadRequest.MediaType,
		contract.Service.ReloadAcknowledgement.MediaType,
		contract.Service.Readiness.MediaType,
		contract.Service.Manifest.MediaType,
		contract.Service.ConfigurationSchema.MediaType,
		contract.Identity.Registration.MediaType,
		contract.Service.Responses.ContentTypes.JSON,
		contract.Service.Responses.ContentTypes.Metrics,
	}); err != nil {
		return nil, err
	}
	if contract.Service.Responses.Health.Status < 100 ||
		len(contract.Service.Responses.Health.Body) == 0 ||
		contract.Service.MaximumMetadataBytes <= 0 {
		return nil, ErrInvalidControlCall
	}
	notReady, err := contract.Problem(controlErrorNotReady)
	if err != nil {
		return nil, err
	}
	problems, err := newControlProblemResolver(contract, controlCallReload)
	if err != nil {
		return nil, err
	}
	if contract.Deadlines.ServiceReloadSeconds <= 0 || contract.Deadlines.ServiceReadinessSeconds <= 0 {
		return nil, ErrInvalidControlCall
	}
	return &ServiceClient{
		contract:  contract,
		baseURL:   parsed,
		transport: transport,
		peer:      peer,
		problems:  problems,
		routes:    routes,
		notReady:  notReady.Status,
		reload:    contract.Deadlines.ServiceReloadSeconds,
		read:      contract.Deadlines.ServiceReadinessSeconds,
	}, nil
}

// PeerIdentity returns the single registered Core replica identity this client
// pins. It is the identity the transport must verify, and it is exposed so an
// operator can confirm which replica the client was built for.
func (client *ServiceClient) PeerIdentity() models.PeerIdentity {
	if client == nil {
		return models.PeerIdentity{}
	}
	return client.peer
}

// ClassifyOutcome implements the optional adapter port of the application layer.
// It reports only what the call actually saw, only for the Core-to-service family,
// and an empty outcome whenever the failure was not attributable to a published
// contract problem, leaving the decision to the caller's own classification.
func (client *ServiceClient) ClassifyOutcome(err error) models.Outcome {
	if client == nil {
		return ""
	}
	return classifyControlOutcome(err, controlCallReload)
}

// Reload announces one immutable generation and returns the acknowledgement the
// replica published after its own validator and applier ran.
//
// The request is checked against the descriptor syntax and the contract request
// limit before anything is sent, so an unusable announcement is refused locally
// instead of consuming a lifecycle operation on the replica. A refused answer is
// attributed to the contract outcome the replica published, so the operator
// learns a stale generation from a stale generation. The acknowledgement is
// returned exactly as the contract declares it, with its outcome checked against
// the closed vocabulary: an outcome outside that vocabulary is a document the
// client cannot interpret, not a lifecycle result.
//
// Reload is never retried. An unanswered or refused announcement is the Core
// replica's decision to repeat: repeating it here would re-send the replica
// credentials against an outcome the client does not know.
func (client *ServiceClient) Reload(ctx context.Context, reload models.Reload) (models.ReloadAcknowledgement, error) {
	if client == nil {
		return models.ReloadAcknowledgement{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	if err := reload.Validate(); err != nil {
		// The announcement never reached the replica, so no contract outcome is
		// claimed for it: the refusal is entirely local.
		return models.ReloadAcknowledgement{}, controlUnusable(controlCallReload, models.ErrInvalidReload)
	}
	document := client.contract.Service.ReloadRequest
	contents, err := client.marshal(reload, document.MediaType, document.MaximumBytes,
		document.Required, controlCallReload)
	if err != nil {
		return models.ReloadAcknowledgement{}, err
	}
	response, err := client.call(ctx, serviceRouteReload, document.MediaType,
		document.MaximumBytes, controlCallReload, bytes.NewReader(contents))
	if err != nil {
		return models.ReloadAcknowledgement{}, err
	}
	defer closeResource(response.Body)
	acknowledgement := client.contract.Service.ReloadAcknowledgement
	if !controlIsSuccess(response.StatusCode) {
		return models.ReloadAcknowledgement{}, client.refusal(response, acknowledgement.MaximumBytes)
	}
	body, err := client.document(response, acknowledgement, controlCallReload)
	if err != nil {
		return models.ReloadAcknowledgement{}, err
	}
	var published models.ReloadAcknowledgement
	if err := decodeControlDocument(body, &published); err != nil {
		return models.ReloadAcknowledgement{}, controlUnusable(controlCallReload, err)
	}
	if !published.Outcome.Valid() {
		// An outcome outside the closed vocabulary is not a lifecycle result this
		// client can attribute, so the answer is refused without claiming one.
		return models.ReloadAcknowledgement{}, controlUnusable(controlCallReload, ErrControlDocument)
	}
	return published, nil
}

// Readiness reports which generation this replica has actually applied. A
// replica that is not ready is a documented answer, not a failure: the contract
// registers one non-success status for the route, and the document is returned
// as published so the operator can see the pending generation the replica refused.
//
// A document that contradicts its own status line is refused rather than
// reported, because a body claiming readiness on a not-ready status is not a
// fact about the replica.
func (client *ServiceClient) Readiness(ctx context.Context) (models.Readiness, error) {
	if client == nil {
		return models.Readiness{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	document := client.contract.Service.Readiness
	response, err := client.call(ctx, serviceRouteReady, document.MediaType, document.MaximumBytes,
		controlCallReload, nil)
	if err != nil {
		return models.Readiness{}, err
	}
	defer closeResource(response.Body)
	ready := client.notReady
	if !controlIsSuccess(response.StatusCode) && response.StatusCode != ready {
		return models.Readiness{}, client.refusal(response, document.MaximumBytes)
	}
	body, err := client.document(response, document, controlCallReload)
	if err != nil {
		return models.Readiness{}, err
	}
	var published models.Readiness
	if err := decodeControlDocument(body, &published); err != nil {
		return models.Readiness{}, controlUnusable(controlCallReload, err)
	}
	if published.Ready != controlIsSuccess(response.StatusCode) {
		return models.Readiness{}, controlUnusable(controlCallReload, ErrControlPlaneUnusable)
	}
	return published, nil
}

// Health reports whether the replica serves the contract health answer. It
// returns no document: health is a liveness fact, and the contract publishes it
// as a fixed status plus a fixed body.
//
// The contract registers no dedicated health byte ceiling, so the service
// metadata bound is used. It is a contract value, not a Go constant, and it
// keeps an unbounded peer body from being buffered.
func (client *ServiceClient) Health(ctx context.Context) error {
	if client == nil {
		return controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	health := client.contract.Service.Responses.Health
	maximum := client.contract.Service.MaximumMetadataBytes
	response, err := client.call(ctx, serviceRouteHealth, client.contract.Service.Responses.ContentTypes.JSON,
		maximum, controlCallReload, nil)
	if err != nil {
		return err
	}
	defer closeResource(response.Body)
	if response.StatusCode != health.Status {
		return client.refusal(response, maximum)
	}
	body, err := client.document(response, DocumentContract{
		MediaType:    client.contract.Service.Responses.ContentTypes.JSON,
		MaximumBytes: maximum,
	}, controlCallReload)
	if err != nil {
		return err
	}
	return controlUnusableOnFailure(controlCallReload, controlHealthBody(body, health.Body))
}

// Manifest returns the replica's manifest document as the exact bytes the
// replica published. The document is not parsed, decoded or re-encoded: the SDK
// does not interpret a product field, and a caller that needs to read it owns
// that decision.
func (client *ServiceClient) Manifest(ctx context.Context) ([]byte, error) {
	if client == nil {
		return nil, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	return client.documentRoute(ctx, serviceRouteManifest, client.contract.Service.Manifest)
}

// ConfigurationSchema returns the replica's configuration schema as the exact
// bytes the replica published, for the same reason Manifest does.
func (client *ServiceClient) ConfigurationSchema(ctx context.Context) ([]byte, error) {
	if client == nil {
		return nil, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	return client.documentRoute(ctx, serviceRouteConfigSchema, client.contract.Service.ConfigurationSchema)
}

// Identity returns the bootstrap registration document this replica publishes so
// Core can reconcile a reconnected replica. The document is validated against
// the contract's required fields and against the identity syntax the domain
// owns, and is otherwise returned as published.
func (client *ServiceClient) Identity(ctx context.Context) (models.Registration, error) {
	if client == nil {
		return models.Registration{}, controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	registration := client.contract.Identity.Registration
	response, err := client.call(ctx, serviceRouteIdentity, registration.MediaType,
		registration.MaximumBytes, controlCallReload, nil)
	if err != nil {
		return models.Registration{}, err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return models.Registration{}, client.refusal(response, registration.MaximumBytes)
	}
	body, err := client.document(response, DocumentContract{
		MediaType: registration.MediaType, MaximumBytes: registration.MaximumBytes, Required: registration.Required,
	}, controlCallReload)
	if err != nil {
		return models.Registration{}, err
	}
	var published models.Registration
	if err := decodeControlDocument(body, &published); err != nil {
		return models.Registration{}, controlUnusable(controlCallReload, err)
	}
	if err := published.Validate(); err != nil {
		return models.Registration{}, controlUnusable(controlCallReload, models.ErrInvalidIdentity)
	}
	return published, nil
}

// Metrics returns the replica's metrics exposition as the exact text the replica
// published. The document is not parsed: the exposition format and the metric
// names are the replica's to publish, and the SDK does not second-guess them.
func (client *ServiceClient) Metrics(ctx context.Context) (string, error) {
	if client == nil {
		return "", controlUnusable(controlCallReload, ErrInvalidControlCall)
	}
	mediaType := client.contract.Service.Responses.ContentTypes.Metrics
	maximum := client.contract.Service.MaximumMetadataBytes
	response, err := client.call(ctx, serviceRouteMetrics, mediaType, maximum, controlCallReload, nil)
	if err != nil {
		return "", err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return "", client.refusal(response, maximum)
	}
	body, err := readControlExposition(response, DocumentContract{MediaType: mediaType, MaximumBytes: maximum},
		controlCallReload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// CloseIdleConnections releases pooled control connections when the transport
// supports it, so a rotated replica credential is not served over an already
// established connection. A transport that pools nothing is not an error.
func (client *ServiceClient) CloseIdleConnections() {
	if client == nil {
		return
	}
	if closer, ok := client.transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// documentRoute reads a metadata route and returns the published bytes.
func (client *ServiceClient) documentRoute(ctx context.Context, name string, document DocumentContract) ([]byte, error) {
	response, err := client.call(ctx, name, document.MediaType, document.MaximumBytes, controlCallReload, nil)
	if err != nil {
		return nil, err
	}
	defer closeResource(response.Body)
	if !controlIsSuccess(response.StatusCode) {
		return nil, client.refusal(response, document.MaximumBytes)
	}
	return client.document(response, document, controlCallReload)
}

// call performs exactly one control-plane request on a service route. It resolves
// the route from the contract, bounds the call with the deadline of the operation,
// and hands the request to the shared transport. It never retries and never
// follows a redirect: the transport is the only thing that can move bytes, and
// refusing both here keeps a replica credential on exactly one origin.
//
// body is the request document, or nil for a read. A nil body is passed to
// net/http as an untyped nil so a read never carries a request body at all,
// rather than as a nil reader of a concrete type, which would still be a
// non-nil io.Reader.
func (client *ServiceClient) call(ctx context.Context, name, accept string, maximum int64, kind controlCallKind, body io.Reader) (*http.Response, error) {
	return client.callWithHeaders(ctx, name, accept, maximum, kind, body, nil)
}

func (client *ServiceClient) callWithHeaders(ctx context.Context, name, accept string, maximum int64, kind controlCallKind, body io.Reader, headers http.Header) (*http.Response, error) {
	if maximum <= 0 {
		return nil, controlUnusable(kind, ErrInvalidControlCall)
	}
	route, resolved := client.routes[name]
	if !resolved {
		return nil, controlUnusable(kind, ErrInvalidControlCall)
	}
	target, err := controlEndpointURL(client.baseURL, route.Path)
	if err != nil {
		return nil, controlUnusable(kind, err)
	}
	deadline := client.read
	if name == serviceRouteReload {
		deadline = client.reload
	}
	callCtx, cancel, err := controlCallContext(ctx, deadline)
	if err != nil {
		return nil, controlUnusable(kind, err)
	}
	request, err := http.NewRequestWithContext(callCtx, route.Method, target.String(), body)
	if err != nil {
		cancel()
		return nil, controlUnusable(kind, ErrControlTransportFailed)
	}
	if body != nil {
		request.Header.Set("content-type", client.contract.Service.ReloadRequest.MediaType)
	}
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	request.Header.Set("accept", accept)
	response, err := performControlCall(client.transport, request, kind)
	if err != nil {
		cancel()
		return nil, err
	}
	if response.Body == nil {
		cancel()
		return nil, controlUnusable(kind, ErrControlPlaneUnusable)
	}
	response.Body = &controlResponseBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

// controlResponseBody keeps the request deadline active while the caller reads
// the response. Cancelling in call() before the body is consumed can truncate
// an otherwise successful response after its headers arrive.
type controlResponseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *controlResponseBody) Close() error {
	body.cancel()
	return body.ReadCloser.Close()
}

// document applies the contract's rules to one successful response: the exact
// media type, the byte limit, and every field the contract marks required. It
// returns the bytes exactly as published and never decodes them.
func (client *ServiceClient) document(response *http.Response, document DocumentContract, kind controlCallKind) ([]byte, error) {
	expected, err := controlMediaType(document.MediaType)
	if err != nil {
		return nil, controlUnusable(kind, err)
	}
	if err := requireControlMediaType(response, expected); err != nil {
		return nil, controlUnusable(kind, err)
	}
	contents, err := readControlBody(response.Body, document.MaximumBytes)
	if err != nil {
		return nil, controlBodyFailure(kind, err)
	}
	if err := requireControlFields(contents, document.Required); err != nil {
		return nil, controlUnusable(kind, err)
	}
	return contents, nil
}

// marshal renders one request document and applies the contract's request rules:
// the exact media type, the byte limit, and every field the contract marks
// required. An oversized request is never sent, so the caller learns its own
// input was unusable rather than discovering it from a replica.
func (client *ServiceClient) marshal(document any, mediaType string, maximum int64, required []string, kind controlCallKind) ([]byte, error) {
	if _, err := controlMediaType(mediaType); err != nil {
		return nil, controlUnusable(kind, err)
	}
	contents, err := json.Marshal(document)
	if err != nil {
		return nil, controlUnusable(kind, ErrControlRequestOversized)
	}
	if err := requireControlRequestSize(contents, maximum); err != nil {
		return nil, controlUnusable(kind, err)
	}
	if err := requireControlFields(contents, required); err != nil {
		return nil, controlUnusable(kind, err)
	}
	return contents, nil
}

// refusal attributes a non-success answer to the problem the replica published.
// A body the client cannot read is not a licence to guess: the call stays
// unattributed and the caller's own transport classification decides.
func (client *ServiceClient) refusal(response *http.Response, maximum int64) error {
	contents, err := readControlBody(response.Body, maximum)
	if err != nil {
		return controlUnusable(controlCallReload, ErrControlRequestRefused)
	}
	outcome, attributed := client.problems.resolve(response.StatusCode, decodeControlProblem(contents))
	if !attributed {
		return controlUnusable(controlCallReload, ErrControlRequestRefused)
	}
	return controlFailure(controlCallReload, outcome, ErrControlRequestRefused)
}

// controlMediaType returns the bare media type of a contract-declared value.
// The contract may attach parameters, as it does for the metrics exposition, and
// a response may attach its own, so only the media type is compared. A declared
// value that is not a media type at all is refused rather than defaulted.
func controlMediaType(declared string) (string, error) {
	mediaType, _, err := mime.ParseMediaType(declared)
	if err != nil || mediaType == "" {
		return "", ErrInvalidControlCall
	}
	return mediaType, nil
}

// requireControlMediaTypes refuses a contract whose declared media types cannot
// be expressed, so a client is never built that would reject every response.
func requireControlMediaTypes(declared []string) error {
	for _, value := range declared {
		if _, err := controlMediaType(value); err != nil {
			return err
		}
	}
	return nil
}

// controlIsSuccess reports whether a status is a success answer. Every route but
// health accepts any success status, and health additionally requires the exact
// status the contract declares.
func controlIsSuccess(status int) bool {
	return status >= 200 && status < 300
}

// controlHealthBody requires the fixed health document the contract declares. A
// field the contract names must be present with the declared value; nothing else
// in the body is interpreted, because the health document is a liveness fact and
// not a product document.
func controlHealthBody(contents []byte, expected map[string]string) error {
	if !models.ValidJSONObject(contents) {
		return ErrControlDocument
	}
	var fields map[string]string
	if err := json.Unmarshal(contents, &fields); err != nil {
		return ErrControlDocument
	}
	for key, want := range expected {
		if published, present := fields[key]; !present || published != want {
			return ErrControlPlaneUnusable
		}
	}
	return nil
}

// controlUnusableOnFailure attributes a document that failed validation. The
// check produced no contract outcome of its own, so the call stays unattributed
// and the caller's own transport classification decides.
func controlUnusableOnFailure(kind controlCallKind, err error) error {
	if err == nil {
		return nil
	}
	return controlUnusable(kind, err)
}

// controlZeroBytes clears a transport buffer that held bytes the SDK must not
// keep. Any value handed on to a caller is a private copy, so clearing the read
// buffer bounds the number of live copies.
func controlZeroBytes(contents []byte) {
	for index := range contents {
		contents[index] = 0
	}
}
