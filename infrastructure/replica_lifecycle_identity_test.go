package infrastructure

import (
	"crypto/x509"
	"net/url"
	"testing"

	"github.com/Liapoldus/plugin-sdk/v2/domain/models"
)

func TestReplicaIdentityComesFromExactClientCertificateURI(t *testing.T) {
	contract, err := LoadReplicaLifecycleContract()
	if err != nil {
		t.Fatal(err)
	}
	identity := models.PeerReplicaID{InstanceID: "server", ReplicaID: "server", IncarnationID: "smoke-20261011", PlacementID: "local"}
	uri, err := contract.ReplicaIdentityURI(identity)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := ReplicaIdentityFromClientCertificate(contract, &x509.Certificate{URIs: []*url.URL{parsed}}, "server", "server", "local")
	if err != nil {
		t.Fatal(err)
	}
	if actual != identity {
		t.Fatalf("identity=%+v want %+v", actual, identity)
	}
}

func TestReplicaIdentityRejectsCertificateForAnotherReplica(t *testing.T) {
	contract, err := LoadReplicaLifecycleContract()
	if err != nil {
		t.Fatal(err)
	}
	uri, err := contract.ReplicaIdentityURI(models.PeerReplicaID{InstanceID: "echo", ReplicaID: "echo", IncarnationID: "smoke-20261011", PlacementID: "local"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplicaIdentityFromClientCertificate(contract, &x509.Certificate{URIs: []*url.URL{parsed}}, "server", "server", "local"); err == nil {
		t.Fatal("expected mismatched plugin identity to be rejected")
	}
}

func TestReplicaIdentityRejectsAmbiguousCertificateURIs(t *testing.T) {
	contract, err := LoadReplicaLifecycleContract()
	if err != nil {
		t.Fatal(err)
	}
	first, _ := url.Parse("spiffe://liapoldus/plugin/server/server/inc-1")
	second, _ := url.Parse("spiffe://liapoldus/plugin/server/server/inc-2")
	if _, err := ReplicaIdentityFromClientCertificate(contract, &x509.Certificate{URIs: []*url.URL{first, second}}, "server", "server", "local"); err == nil {
		t.Fatal("expected ambiguous certificate identity to be rejected")
	}
}
