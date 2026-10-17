package infrastructure

import (
	"crypto/x509"
	"errors"
	"net/url"
	"strings"

	"github.com/Liapoldus/service-sdk/v2/domain/models"
)

var ErrInvalidReplicaLifecycleContract = errors.New("invalid Service SDK replica lifecycle contract")

type ReplicaLifecycleEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type ReplicaLifecycleDocument struct {
	MediaType    string   `json:"mediaType"`
	MaximumBytes int64    `json:"maximumBytes"`
	Required     []string `json:"required"`
	Status       int      `json:"status,omitempty"`
}

type ReplicaLifecycleProblem struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
}

type ReplicaRegistrationSemantics struct {
	ActiveDuplicate            string   `json:"activeDuplicate"`
	ChangedImmutableMetadata   string   `json:"changedImmutableMetadata"`
	NewIncarnation             string   `json:"newIncarnation"`
	ImmutableFields            []string `json:"immutableFields"`
	RenewableFields            []string `json:"renewableFields"`
	DeregisterUnknown          string   `json:"deregisterUnknown"`
	RevokedIncarnationCanRenew bool     `json:"revokedIncarnationCanRenew"`
}

type ReleaseCohortCompatibility struct {
	SameReleaseDigest string `json:"sameReleaseDigest"`
	DifferentRelease  string `json:"differentRelease"`
	MissingEvidence   string `json:"missingEvidence"`
	VersionRange      string `json:"versionRange"`
}

type ReplicaLifecycleContract struct {
	ContractVersion   string `json:"contractVersion"`
	ProblemMediaType  string `json:"problemMediaType"`
	TransportSecurity struct {
		TLSRequired               bool   `json:"tlsRequired"`
		ClientCertificateRequired bool   `json:"clientCertificateRequired"`
		CertificateIdentity       string `json:"certificateIdentity"`
	} `json:"transportSecurity"`
	Identity struct {
		URITemplate    string `json:"uriTemplate"`
		CommonNameUsed bool   `json:"commonNameUsed"`
	} `json:"identity"`
	Lease struct {
		TTLSeconds           int `json:"ttlSeconds"`
		RenewIntervalSeconds int `json:"renewIntervalSeconds"`
	} `json:"lease"`
	Deadlines struct {
		RegisterSeconds   int `json:"registerSeconds"`
		RenewSeconds      int `json:"renewSeconds"`
		DeregisterSeconds int `json:"deregisterSeconds"`
	} `json:"deadlines"`
	Limits struct {
		MaximumPeerEndpoints    int `json:"maximumPeerEndpoints"`
		MaximumContractsPerList int `json:"maximumContractsPerList"`
	} `json:"limits"`
	Endpoints   map[string]ReplicaLifecycleEndpoint `json:"endpoints"`
	Requests    map[string]ReplicaLifecycleDocument `json:"requests"`
	Responses   map[string]ReplicaLifecycleDocument `json:"responses"`
	LeaseExpiry struct {
		RenewalAfterExpiry     string `json:"renewalAfterExpiry"`
		OldIncarnationRenewal  string `json:"oldIncarnationRenewal"`
		ExpiredReplicaEligible bool   `json:"expiredReplicaEligible"`
	} `json:"leaseExpiry"`
	RegistrationSemantics      ReplicaRegistrationSemantics       `json:"registrationSemantics"`
	ReleaseCohortCompatibility ReleaseCohortCompatibility         `json:"releaseCohortCompatibility"`
	Problems                   map[string]ReplicaLifecycleProblem `json:"problems"`
}

// LoadReplicaLifecycleContract returns the immutable owner contract for generic
// replica registration, lease renewal and deregistration.
func LoadReplicaLifecycleContract() (ReplicaLifecycleContract, error) {
	contract := newReplicaLifecycleContract()
	if err := contract.Validate(); err != nil {
		return ReplicaLifecycleContract{}, err
	}
	return contract, nil
}

// ReplicaLifecycleSchema returns an owned copy of the exact versioned request
// and response DTO schema for Core implementations and conformance consumers.
func ReplicaLifecycleSchema() []byte {
	return schemaDocument(replicaLifecycleDefinition())
}

func (contract ReplicaLifecycleContract) Validate() error {
	uriTemplate := contract.Identity.URITemplate
	if contract.ContractVersion == "" || contract.ProblemMediaType == "" || !contract.TransportSecurity.TLSRequired ||
		!contract.TransportSecurity.ClientCertificateRequired || contract.TransportSecurity.CertificateIdentity == "" ||
		contract.Identity.CommonNameUsed || !strings.Contains(uriTemplate, "{instanceId}") ||
		!strings.Contains(uriTemplate, "{replicaId}") || !strings.Contains(uriTemplate, "{incarnationId}") ||
		contract.Lease.TTLSeconds <= 0 || contract.Lease.RenewIntervalSeconds <= 0 ||
		contract.Lease.RenewIntervalSeconds >= contract.Lease.TTLSeconds ||
		contract.Deadlines.RegisterSeconds <= 0 || contract.Deadlines.RenewSeconds <= 0 || contract.Deadlines.DeregisterSeconds <= 0 ||
		contract.Limits.MaximumPeerEndpoints <= 0 || contract.Limits.MaximumContractsPerList <= 0 ||
		contract.LeaseExpiry.RenewalAfterExpiry == "" || contract.LeaseExpiry.OldIncarnationRenewal == "" ||
		contract.LeaseExpiry.ExpiredReplicaEligible || contract.RegistrationSemantics.ActiveDuplicate == "" ||
		contract.RegistrationSemantics.ChangedImmutableMetadata == "" || contract.RegistrationSemantics.NewIncarnation == "" ||
		len(contract.RegistrationSemantics.ImmutableFields) == 0 || len(contract.RegistrationSemantics.RenewableFields) == 0 ||
		contract.RegistrationSemantics.DeregisterUnknown == "" || contract.RegistrationSemantics.RevokedIncarnationCanRenew {
		return ErrInvalidReplicaLifecycleContract
	}
	if contract.ReleaseCohortCompatibility.SameReleaseDigest != "compatible" ||
		contract.ReleaseCohortCompatibility.DifferentRelease != "mutual-acceptance-of-all-advertised-contract-versions" ||
		contract.ReleaseCohortCompatibility.MissingEvidence != "incompatible" ||
		contract.ReleaseCohortCompatibility.VersionRange != "semver-half-open" {
		return ErrInvalidReplicaLifecycleContract
	}
	for _, name := range []string{"register", "renew", "deregister"} {
		endpoint, ok := contract.Endpoints[name]
		if !ok || endpoint.Method != "POST" || !strings.HasPrefix(endpoint.Path, "/") ||
			strings.ContainsAny(endpoint.Path, "?#\\") {
			return ErrInvalidReplicaLifecycleContract
		}
		request, requestOK := contract.Requests[name]
		response, responseOK := contract.Responses[name]
		if !requestOK || !responseOK || request.MediaType == "" || response.MediaType != request.MediaType ||
			request.MaximumBytes <= 0 || response.MaximumBytes <= 0 || len(request.Required) == 0 || len(response.Required) == 0 {
			return ErrInvalidReplicaLifecycleContract
		}
	}
	for _, name := range []string{"register", "renew", "deregister"} {
		if contract.Responses[name].Status < 200 || contract.Responses[name].Status >= 300 {
			return ErrInvalidReplicaLifecycleContract
		}
	}
	for _, name := range []string{"invalidRequest", "unauthenticated", "identityMismatch", "notRegistered", "identityConflict", "leaseExpired", "unavailable"} {
		problem, ok := contract.Problems[name]
		if !ok || problem.Status < 400 || problem.Code == "" {
			return ErrInvalidReplicaLifecycleContract
		}
	}
	return nil
}

// ReplicaIdentityURI expands the certificate identity URI declared by this
// contract for one fully validated instance/replica/incarnation/placement tuple.
func (contract ReplicaLifecycleContract) ReplicaIdentityURI(identity models.PeerReplicaID) (string, error) {
	if err := contract.Validate(); err != nil {
		return "", err
	}
	return replicaIdentityURI(contract, identity)
}

// ReplicaIdentityFromClientCertificate binds the runtime incarnation to the
// one exact SDK lifecycle URI carried by the configured client certificate.
// This lets operator-issued certificates define the replica incarnation; a
// process must not invent a random incarnation that its certificate cannot
// authenticate.
func ReplicaIdentityFromClientCertificate(contract ReplicaLifecycleContract, certificate *x509.Certificate, instanceID, replicaID, placementID string) (models.PeerReplicaID, error) {
	if contract.Validate() != nil || certificate == nil || len(certificate.URIs) != 1 ||
		!models.ValidInstanceID(instanceID) || !models.ValidReplicaID(replicaID) || !models.ValidInstanceID(placementID) {
		return models.PeerReplicaID{}, models.ErrReplicaIdentityMismatch
	}
	template := contract.Identity.URITemplate
	if strings.Count(template, "{instanceId}") != 1 || strings.Count(template, "{replicaId}") != 1 ||
		strings.Count(template, "{incarnationId}") != 1 || !strings.HasSuffix(template, "{incarnationId}") {
		return models.PeerReplicaID{}, ErrInvalidReplicaLifecycleContract
	}
	prefix := strings.TrimSuffix(template, "{incarnationId}")
	prefix = strings.ReplaceAll(prefix, "{instanceId}", url.PathEscape(instanceID))
	prefix = strings.ReplaceAll(prefix, "{replicaId}", url.PathEscape(replicaID))
	uri := certificate.URIs[0].String()
	if !strings.HasPrefix(uri, prefix) {
		return models.PeerReplicaID{}, models.ErrReplicaIdentityMismatch
	}
	escapedIncarnation := strings.TrimPrefix(uri, prefix)
	if escapedIncarnation == "" || strings.Contains(escapedIncarnation, "/") {
		return models.PeerReplicaID{}, models.ErrReplicaIdentityMismatch
	}
	incarnationID, err := url.PathUnescape(escapedIncarnation)
	if err != nil {
		return models.PeerReplicaID{}, models.ErrReplicaIdentityMismatch
	}
	identity := models.PeerReplicaID{
		InstanceID: instanceID, ReplicaID: replicaID,
		IncarnationID: incarnationID, PlacementID: placementID,
	}
	expectedURI, err := contract.ReplicaIdentityURI(identity)
	if err != nil || expectedURI != uri {
		return models.PeerReplicaID{}, models.ErrReplicaIdentityMismatch
	}
	return identity, nil
}

func (contract ReplicaLifecycleContract) endpointURL(base *url.URL, name string) (*url.URL, error) {
	endpoint, ok := contract.Endpoints[name]
	if !ok || base == nil || base.Scheme != "https" || base.Host == "" || base.User != nil ||
		base.RawQuery != "" || base.Fragment != "" || !strings.HasPrefix(endpoint.Path, "/") {
		return nil, ErrInvalidReplicaLifecycleContract
	}
	parsed, err := url.ParseRequestURI(endpoint.Path)
	if err != nil || parsed.Host != "" || parsed.Scheme != "" {
		return nil, ErrInvalidReplicaLifecycleContract
	}
	target := *base
	target.Path = strings.TrimSuffix(base.Path, "/") + parsed.Path
	target.RawPath = ""
	return &target, nil
}

func replicaIdentityURI(contract ReplicaLifecycleContract, identity models.PeerReplicaID) (string, error) {
	if !identity.Valid() {
		return "", ErrInvalidReplicaLifecycleContract
	}
	uri := contract.Identity.URITemplate
	for placeholder, value := range map[string]string{
		"{instanceId}":    identity.InstanceID,
		"{replicaId}":     identity.ReplicaID,
		"{incarnationId}": identity.IncarnationID,
	} {
		uri = strings.ReplaceAll(uri, placeholder, url.PathEscape(value))
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "spiffe" || parsed.Host == "" || strings.Contains(uri, "{") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidReplicaLifecycleContract
	}
	return uri, nil
}
