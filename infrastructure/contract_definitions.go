package infrastructure

// Code-owned versioned constants. Constructors allocate fresh maps and slices.
// Public JSON constants are derived from these definitions, never loaded at runtime.
func newHTTPContract() HTTPContract {
	var contract HTTPContract
	contract.ContractVersion = expectedContractVersion
	contract.TransportSecurity.MinimumTLSVersion = 772
	contract.TransportSecurity.ClientCertificateRequired = true
	contract.TransportSecurity.TrustDomain = "core-control-plane"
	contract.TransportSecurity.PeerIdentity.CommonNameRequired = true
	contract.TransportSecurity.PeerIdentity.CommonNameMaximumLength = 256
	contract.TransportSecurity.PeerIdentity.UniformResourceIdentifierPrefix = "spiffe://liapoldus/core/"
	contract.TransportSecurity.PeerIdentity.RevocationFailClosed = true
	contract.Identity.Replica.MaximumBytes = 256
	contract.Identity.Replica.Fields = []string{"instanceId", "replicaId"}
	contract.Identity.Registration.MediaType = "application/json"
	contract.Identity.Registration.MaximumBytes = 4096
	contract.Identity.Registration.Required = []string{"contractVersion", "instanceId", "replicaId", "appliedGeneration", "ready"}
	contract.Service.Responses.Health.Status = 200
	contract.Service.Responses.Health.Body = map[string]string{
		"status": "ok",
	}
	contract.Service.Responses.ContentTypes.JSON = "application/json"
	contract.Service.Responses.ContentTypes.Metrics = "text/plain; version=0.0.4; charset=utf-8"
	contract.Service.Responses.Metrics.ReadyMetricName = "liapoldus_service_ready"
	contract.Service.Responses.Metrics.ReadyMetricHelp = "Whether the service has an active configuration generation."
	contract.Service.Responses.Metrics.LifecycleCounterName = "liapoldus_service_lifecycle_total"
	contract.Service.Responses.Metrics.LifecycleCounterHelp = "Count of service lifecycle operations by kind and bounded outcome."
	contract.Service.Responses.Metrics.LifecycleCounterKindLabel = "kind"
	contract.Service.Responses.Metrics.LifecycleCounterOutcomeLabel = "outcome"
	contract.Service.Responses.Metrics.PullFailureCounterName = "liapoldus_service_config_pull_failures_total"
	contract.Service.Responses.Metrics.PullFailureCounterHelp = "Count of exact-generation configuration pull attempts that did not yield an applicable document."
	contract.Service.Endpoints = map[string]Endpoint{
		"adminAction":    {Method: "POST", Path: "/_liapoldus/v1/admin-action/{page}/{action}"},
		"adminSurface":   {Method: "GET", Path: "/_liapoldus/v1/admin-surface"},
		"artifactStream": {Method: "POST", Path: "/_liapoldus/v1/artifact-stream"},
		"configSchema":   {Method: "GET", Path: "/_liapoldus/v1/config-schema"},
		"health":         {Method: "GET", Path: "/healthz"},
		"identity":       {Method: "GET", Path: "/_liapoldus/v1/identity"},
		"manifest":       {Method: "GET", Path: "/_liapoldus/v1/manifest"},
		"metrics":        {Method: "GET", Path: "/metrics"},
		"ready":          {Method: "GET", Path: "/_liapoldus/v1/ready"},
		"reload":         {Method: "POST", Path: "/_liapoldus/v1/reload"},
	}
	contract.Service.ReloadRequest.MediaType = "application/json"
	contract.Service.ReloadRequest.MaximumBytes = 4096
	contract.Service.ReloadRequest.Required = []string{"generation", "sha256", "schemaVersion"}
	contract.Service.ReloadRequest.DigestAlgorithm = ""
	contract.Service.ReloadAcknowledgement.MediaType = "application/json"
	contract.Service.ReloadAcknowledgement.MaximumBytes = 4096
	contract.Service.ReloadAcknowledgement.Required = []string{"generation", "sha256", "schemaVersion", "applied", "outcome"}
	contract.Service.ReloadAcknowledgement.DigestAlgorithm = ""
	contract.Service.Readiness.MediaType = "application/json"
	contract.Service.Readiness.MaximumBytes = 4096
	contract.Service.Readiness.Required = []string{"ready", "generation", "sha256", "schemaVersion", "pendingGeneration", "instanceId", "replicaId"}
	contract.Service.Readiness.DigestAlgorithm = ""
	contract.Service.Manifest.MediaType = "application/json"
	contract.Service.Manifest.MaximumBytes = 262144
	contract.Service.Manifest.Required = nil
	contract.Service.Manifest.DigestAlgorithm = ""
	contract.Service.ConfigurationSchema.MediaType = "application/json"
	contract.Service.ConfigurationSchema.MaximumBytes = 262144
	contract.Service.ConfigurationSchema.Required = nil
	contract.Service.ConfigurationSchema.DigestAlgorithm = ""
	contract.Service.MaximumMetadataBytes = 262144
	contract.Service.ArtifactStream.MediaType = "multipart/form-data"
	contract.Service.ArtifactStream.MetadataMediaType = "application/json"
	contract.Service.ArtifactStream.Parts = []string{"metadata", "artifact"}
	contract.Service.ArtifactStream.PartOrder = []string{"metadata", "artifact"}
	contract.Service.ArtifactStream.MaximumArtifactBytes = 134217728
	contract.Service.ArtifactStream.MinimumArtifactBytes = 1
	contract.Service.ArtifactStream.MaximumMetadataBytes = 65536
	contract.Service.ArtifactStream.MaximumMultipartOverheadBytes = 65536
	contract.Service.ArtifactStream.MaximumRequestBytes = 134348800
	contract.Service.ArtifactStream.MaximumReceiptBytes = 4096
	contract.Service.ArtifactStream.AcceptedStatus = 202
	contract.Service.ArtifactStream.FilenameForwarded = false
	contract.Service.ArtifactStream.InvocationContext.MaximumBytes = 8192
	contract.Service.ArtifactStream.InvocationContext.Required = []string{"callerId", "instanceId", "pageId", "actionId", "surfaceDigest", "idempotencyKey", "requestId"}
	contract.Service.ArtifactStream.InvocationContext.Optional = []string{"ifMatch"}
	contract.Service.ArtifactStream.InvocationContext.Headers = map[string]string{
		"actionId":       "Liapoldus-Action",
		"callerId":       "Liapoldus-Caller",
		"idempotencyKey": "Idempotency-Key",
		"ifMatch":        "If-Match",
		"instanceId":     "Liapoldus-Instance",
		"pageId":         "Liapoldus-Page",
		"requestId":      "Liapoldus-Request-ID",
		"surfaceDigest":  "Liapoldus-Surface-Digest",
	}
	contract.Service.AdminSurface.MediaType = "application/json"
	contract.Service.AdminSurface.MaximumBytes = 262144
	contract.Service.AdminSurface.Required = nil
	contract.Service.AdminSurface.DigestAlgorithm = "SHA-256"
	contract.Service.AdminAction.MediaType = "application/json"
	contract.Service.AdminAction.MaximumRequestBytes = 1048576
	contract.Service.AdminAction.MaximumResponseBytes = 1048576
	contract.Service.AdminAction.MaximumPageIDBytes = 128
	contract.Service.AdminAction.MaximumActionIDBytes = 128
	contract.Service.AdminAction.PathSegmentPattern = "^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$"
	contract.Service.AdminAction.ResponseStatus.Minimum = 200
	contract.Service.AdminAction.ResponseStatus.Maximum = 599
	contract.Service.AdminAction.DeadlineSeconds = 30
	contract.Service.AdminAction.InvocationContext.MaximumBytes = 8192
	contract.Service.AdminAction.InvocationContext.UnknownHeaderPrefix = "Liapoldus-"
	contract.Service.AdminAction.InvocationContext.Required = []string{"callerId", "instanceId", "pageId", "actionId", "surfaceDigest", "requestId"}
	contract.Service.AdminAction.InvocationContext.Optional = []string{"idempotencyKey", "ifMatch"}
	contract.Service.AdminAction.InvocationContext.Headers = map[string]string{
		"actionId":       "Liapoldus-Action",
		"callerId":       "Liapoldus-Caller",
		"idempotencyKey": "Idempotency-Key",
		"ifMatch":        "If-Match",
		"instanceId":     "Liapoldus-Instance",
		"pageId":         "Liapoldus-Page",
		"requestId":      "Liapoldus-Request-ID",
		"surfaceDigest":  "Liapoldus-Surface-Digest",
	}
	contract.Core.ConfigPull.Method = "GET"
	contract.Core.ConfigPull.PathTemplate = "/internal/v1/service-config/{generation}"
	contract.Core.ConfigPull.ResponseMediaType = "application/json"
	contract.Core.ConfigPull.MaximumBytes = 262144
	contract.Core.ConfigPull.RequestMediaType = "application/json"
	contract.Core.ConfigPull.GenerationStates = []string{"active", "previous"}
	contract.Core.ConfigPull.ResponseHeaders = map[string]string{
		"generation":      "Liapoldus-Generation",
		"generationState": "Liapoldus-Generation-State",
		"schemaVersion":   "Liapoldus-Schema-Version",
		"sha256":          "Liapoldus-SHA256",
	}
	contract.Core.SecretGrant.Issue.Method = "POST"
	contract.Core.SecretGrant.Issue.PathTemplate = "/internal/v1/service-secret-grants"
	contract.Core.SecretGrant.Issue.RequestMediaType = "application/json"
	contract.Core.SecretGrant.Issue.ResponseMediaType = "application/json"
	contract.Core.SecretGrant.Issue.MaximumRequestBytes = 4096
	contract.Core.SecretGrant.Issue.MaximumResponseBytes = 4096
	contract.Core.SecretGrant.Issue.Required = []string{"reference", "purpose", "generation"}
	contract.Core.SecretGrant.Redemption.Method = "POST"
	contract.Core.SecretGrant.Redemption.PathTemplate = "/internal/v1/service-secret-grants/{handle}/redemption"
	contract.Core.SecretGrant.Redemption.RequestMediaType = "application/json"
	contract.Core.SecretGrant.Redemption.ResponseMediaType = "application/json"
	contract.Core.SecretGrant.Redemption.MaximumRequestBytes = 4096
	contract.Core.SecretGrant.Redemption.MaximumResponseBytes = 65536
	contract.Core.SecretGrant.Redemption.Required = []string{"handle"}
	contract.Core.SecretGrant.HandleMaximumLength = 512
	contract.Core.SecretGrant.ReferenceMaximumLength = 512
	contract.Core.SecretGrant.PurposeMaximumLength = 128
	contract.Core.SecretGrant.RedemptionUseLimit = 1
	contract.Outcomes.Reload = []string{"applied", "alreadyActive", "invalidRequest", "notPermitted", "unknownGeneration", "staleGeneration", "generationConflict", "digestMismatch", "schemaVersionMismatch", "documentMalformed", "documentOversized", "applyRejected", "coreUnavailable", "cancelled", "deadlineExceeded"}
	contract.Outcomes.ConfigPull = []string{"succeeded", "notPermitted", "unknownGeneration", "staleGeneration", "digestMismatch", "schemaVersionMismatch", "documentMalformed", "documentOversized", "coreUnavailable", "cancelled", "deadlineExceeded"}
	contract.Outcomes.SecretRedemption = []string{"granted", "notPermitted", "grantUnknown", "grantDenied", "grantExpired", "grantSpent", "coreUnavailable", "cancelled", "deadlineExceeded"}
	contract.Problems = map[string]ProblemContract{
		"internalError":        {Status: 500, Code: "internal_error"},
		"methodNotAllowed":     {Status: 405, Code: "method_not_allowed"},
		"notFound":             {Status: 404, Code: "not_found"},
		"payloadOversized":     {Status: 413, Code: "payload_oversized"},
		"unsupportedMediaType": {Status: 415, Code: "unsupported_media_type"},
	}
	contract.OutcomeProblems = map[string]string{
		"applyRejected":         "applyRejected",
		"cancelled":             "cancelled",
		"coreUnavailable":       "coreUnavailable",
		"deadlineExceeded":      "deadlineExceeded",
		"digestMismatch":        "digestMismatch",
		"documentMalformed":     "documentMalformed",
		"documentOversized":     "documentOversized",
		"generationConflict":    "generationConflict",
		"grantDenied":           "grantDenied",
		"grantExpired":          "grantExpired",
		"grantSpent":            "grantSpent",
		"grantUnknown":          "grantUnknown",
		"invalidRequest":        "invalidRequest",
		"notPermitted":          "notPermitted",
		"schemaVersionMismatch": "schemaVersionMismatch",
		"staleGeneration":       "staleGeneration",
		"unknownGeneration":     "unknownGeneration",
	}
	contract.SuccessOutcomes = []string{"applied", "alreadyActive", "succeeded", "granted"}
	contract.Errors = map[string]ProblemContract{
		"applyRejected":            {Status: 422, Code: "configuration_rejected"},
		"cancelled":                {Status: 503, Code: "cancelled"},
		"configurationUnavailable": {Status: 502, Code: "configuration_unavailable"},
		"coreUnavailable":          {Status: 502, Code: "core_unavailable"},
		"deadlineExceeded":         {Status: 504, Code: "deadline_exceeded"},
		"digestMismatch":           {Status: 502, Code: "digest_mismatch"},
		"documentMalformed":        {Status: 502, Code: "configuration_malformed"},
		"documentOversized":        {Status: 502, Code: "configuration_oversized"},
		"generationConflict":       {Status: 409, Code: "generation_conflict"},
		"grantDenied":              {Status: 403, Code: "secret_grant_denied"},
		"grantExpired":             {Status: 403, Code: "secret_grant_expired"},
		"grantSpent":               {Status: 403, Code: "secret_grant_spent"},
		"grantUnknown":             {Status: 404, Code: "secret_grant_unknown"},
		"invalidRequest":           {Status: 400, Code: "invalid_request"},
		"notPermitted":             {Status: 403, Code: "not_permitted"},
		"notReady":                 {Status: 503, Code: "not_ready"},
		"schemaVersionMismatch":    {Status: 502, Code: "schema_version_mismatch"},
		"staleGeneration":          {Status: 409, Code: "generation_stale"},
		"unknownGeneration":        {Status: 404, Code: "generation_unknown"},
	}
	contract.Deadlines.ServiceReloadSeconds = 30
	contract.Deadlines.ServiceReadinessSeconds = 5
	contract.Deadlines.CoreConfigPullSeconds = 10
	contract.Deadlines.CoreSecretGrantSeconds = 10
	contract.Deadlines.CoreSecretRedemptionSeconds = 10
	contract.Deadlines.ServiceReadHeaderSeconds = 5
	contract.Deadlines.ServiceReadSeconds = 15
	contract.Deadlines.ServiceWriteSeconds = 60
	contract.Deadlines.ServiceIdleSeconds = 120
	contract.Deadlines.ServiceShutdownGraceSeconds = 20
	contract.Deadlines.ClientResponseHeaderSeconds = 10
	contract.Deadlines.ClientDialSeconds = 10
	contract.Deadlines.ArtifactStreamSeconds = 600
	contract.Logging.Stream = "stdout"
	contract.Logging.Levels = []string{"debug", "info", "warn", "error"}
	contract.Logging.DefaultLevel = "info"
	contract.Logging.MaximumFields = 24
	contract.Logging.MaximumKeyLength = 64
	contract.Logging.MaximumValueLength = 256
	contract.Logging.RedactedPlaceholder = "[redacted]"
	contract.Logging.RedactedKeys = []string{"authorization", "certificate", "config", "cookie", "document", "dsn", "grant", "handle", "key", "password", "private", "secret", "settings", "token", "value"}
	contract.Logging.AlwaysRedactedKeys = []string{"authorization", "cookie", "document", "grant", "handle", "password", "privatekey", "secret", "settings", "token"}
	contract.Idempotency.ReloadKeyedBy = []string{"instanceId", "replicaId", "generation", "sha256", "schemaVersion"}
	contract.Idempotency.RepeatOfActiveGeneration = "alreadyActive"
	contract.Idempotency.ConflictingDescriptorForActiveGeneration = "generationConflict"
	return contract
}
func newReplicaLifecycleContract() ReplicaLifecycleContract {
	var contract ReplicaLifecycleContract
	contract.ContractVersion = "liapoldus.service-sdk.replica-lifecycle.v2"
	contract.ProblemMediaType = "application/json"
	contract.TransportSecurity.TLSRequired = true
	contract.TransportSecurity.ClientCertificateRequired = true
	contract.TransportSecurity.CertificateIdentity = "spiffe-uri-template"
	contract.Identity.URITemplate = "spiffe://liapoldus/service/{instanceId}/{replicaId}/{incarnationId}"
	contract.Identity.CommonNameUsed = false
	contract.Lease.TTLSeconds = 30
	contract.Lease.RenewIntervalSeconds = 10
	contract.Deadlines.RegisterSeconds = 5
	contract.Deadlines.RenewSeconds = 5
	contract.Deadlines.DeregisterSeconds = 5
	contract.Limits.MaximumPeerEndpoints = 4
	contract.Limits.MaximumContractsPerList = 64
	contract.Endpoints = map[string]ReplicaLifecycleEndpoint{
		"deregister": {Method: "POST", Path: "/internal/v2/service-replicas/deregister"},
		"register":   {Method: "POST", Path: "/internal/v2/service-replicas/register"},
		"renew":      {Method: "POST", Path: "/internal/v2/service-replicas/renew"},
	}
	contract.Requests = map[string]ReplicaLifecycleDocument{
		"deregister": {MediaType: "application/json", MaximumBytes: 4096, Required: []string{"contractVersion", "identity"}, Status: 0},
		"register":   {MediaType: "application/json", MaximumBytes: 65536, Required: []string{"contractVersion", "identity", "restEndpoint", "peerEndpoints", "release", "advertisedContracts", "acceptedContracts", "appliedGeneration", "ready"}, Status: 0},
		"renew":      {MediaType: "application/json", MaximumBytes: 8192, Required: []string{"contractVersion", "identity", "appliedGeneration", "ready"}, Status: 0},
	}
	contract.Responses = map[string]ReplicaLifecycleDocument{
		"deregister": {MediaType: "application/json", MaximumBytes: 4096, Required: []string{"contractVersion", "identity", "deregistered"}, Status: 200},
		"register":   {MediaType: "application/json", MaximumBytes: 8192, Required: []string{"contractVersion", "identity", "serverTime", "leaseExpiresAt"}, Status: 201},
		"renew":      {MediaType: "application/json", MaximumBytes: 8192, Required: []string{"contractVersion", "identity", "serverTime", "leaseExpiresAt"}, Status: 200},
	}
	contract.LeaseExpiry.RenewalAfterExpiry = "re-register-with-new-incarnation"
	contract.LeaseExpiry.OldIncarnationRenewal = "reject"
	contract.LeaseExpiry.ExpiredReplicaEligible = false
	contract.RegistrationSemantics.ActiveDuplicate = "same-identity-and-immutable-metadata-renews-lease"
	contract.RegistrationSemantics.ChangedImmutableMetadata = "reject-with-identity-conflict"
	contract.RegistrationSemantics.NewIncarnation = "replaces-expired-or-deregistered-incarnation"
	contract.RegistrationSemantics.ImmutableFields = []string{"identity", "restEndpoint", "peerEndpoints", "release", "advertisedContracts", "acceptedContracts"}
	contract.RegistrationSemantics.RenewableFields = []string{"appliedGeneration", "ready"}
	contract.RegistrationSemantics.DeregisterUnknown = "reject-with-replica-not-registered"
	contract.RegistrationSemantics.RevokedIncarnationCanRenew = false
	contract.ReleaseCohortCompatibility.SameReleaseDigest = "compatible"
	contract.ReleaseCohortCompatibility.DifferentRelease = "mutual-acceptance-of-all-advertised-contract-versions"
	contract.ReleaseCohortCompatibility.MissingEvidence = "incompatible"
	contract.ReleaseCohortCompatibility.VersionRange = "semver-half-open"
	contract.Problems = map[string]ReplicaLifecycleProblem{
		"identityConflict": {Status: 409, Code: "identity_conflict"},
		"identityMismatch": {Status: 403, Code: "identity_mismatch"},
		"invalidRequest":   {Status: 400, Code: "invalid_request"},
		"leaseExpired":     {Status: 409, Code: "lease_expired"},
		"notRegistered":    {Status: 404, Code: "replica_not_registered"},
		"unauthenticated":  {Status: 401, Code: "unauthenticated"},
		"unavailable":      {Status: 503, Code: "core_unavailable"},
	}
	return contract
}
func newPeerDirectoryPollContract() PeerDirectoryPollContract {
	var contract PeerDirectoryPollContract
	contract.ContractVersion = "liapoldus.service-sdk.peer-directory-poll.v1"
	contract.ProblemMediaType = "application/json"
	contract.DirectoryMediaType = "application/json"
	contract.TransportSecurity.TLSRequired = true
	contract.TransportSecurity.ClientCertificateRequired = true
	contract.TransportSecurity.CertificateIdentity = "spiffe-uri-template"
	contract.IdentityBinding = "authenticated-client-certificate-uri-san"
	contract.Identity.URITemplate = "spiffe://liapoldus/service/{instanceId}/{replicaId}/{incarnationId}"
	contract.Identity.CommonNameUsed = false
	contract.Endpoint.Method = "GET"
	contract.Endpoint.Path = "/internal/v2/service-peer-directory"
	contract.Headers.Request = []string{"accept:application/json", "if-none-match:strong-generation-tag"}
	contract.Headers.Response = []string{"etag:strong-generation-tag", "cache-control:no-store"}
	contract.Poll.InitialRequest = "immediate-current-directory"
	contract.Poll.Unchanged = "304-not-modified"
	contract.Poll.Changed = "200-directory"
	contract.Poll.WaitParameter = "waitMs"
	contract.Poll.DefaultWaitMs = 20000
	contract.Poll.MaximumWaitMs = 20000
	contract.Poll.RequestDeadlineMs = 25000
	contract.Poll.ETag = "strong-generation-tag"
	contract.Poll.IfNoneMatch = "exact-current-etag-only"
	contract.Poll.InitialWait = "must-be-zero"
	contract.Poll.Cancellation = "abort-pending-wait"
	contract.Poll.Retry = "caller-controlled-no-automatic-replay"
	contract.Requests.MaximumBytes = 0
	contract.Requests.QueryParameters = []string{"waitMs"}
	contract.Requests.UnknownQueryParameters = "reject"
	contract.Responses.Directory.Status = 200
	contract.Responses.Directory.MediaType = "application/json"
	contract.Responses.Directory.MaximumBytes = 1048576
	contract.Responses.Directory.Schema = "https://liapoldus.github.io/spec/service-sdk/v2/peer-directory.schema.json"
	contract.Responses.Directory.ETagMatches = "quoted-directory-generation"
	contract.Responses.Directory.CallerMustMatchAuthenticatedIdentity = true
	contract.Responses.Unchanged.Status = 304
	contract.Responses.Unchanged.MaximumBytes = 0
	contract.Responses.Unchanged.MediaType = "none"
	contract.Responses.Unchanged.ETag = "same-as-if-none-match"
	contract.Problems = map[string]ReplicaLifecycleProblem{
		"identityMismatch": {Status: 403, Code: "identity_mismatch"},
		"invalidRequest":   {Status: 400, Code: "invalid_request"},
		"unauthenticated":  {Status: 401, Code: "unauthenticated"},
		"unavailable":      {Status: 503, Code: "core_unavailable"},
	}
	return contract
}
func newLoopbackPlaintextProfile() LoopbackPlaintextProfile {
	var contract LoopbackPlaintextProfile
	contract.ContractVersion = expectedLoopbackPlaintextVersion
	contract.EnabledByDefault = false
	contract.Transport.Network = "tcp"
	contract.Transport.HostAddressMustBeLiteralIP = true
	contract.Transport.HostMustBeLoopback = true
	contract.Transport.RemoteBindAllowed = false
	contract.Transport.TLSFailureFallback = false
	contract.ExposedEndpoint.HTTPContractVersion = expectedContractVersion
	contract.ExposedEndpoint.EndpointKey = "health"
	contract.NotFoundProblemKey = "notFound"
	contract.ProblemResponseField = "code"
	contract.ExcludedEndpointKeys = []string{"ready", "identity", "manifest", "configSchema", "reload", "artifactStream", "adminSurface", "adminAction", "metrics"}
	contract.CoreEndpointsAlwaysRequireMutualTLS = []string{"configPull", "secretGrant.issue", "secretGrant.redemption"}
	return contract
}
